package bootstrap

import (
	"fmt"
	"log/slog"
	. "metis/internal/app/itsm/config"
	"metis/internal/app/itsm/definition"
	. "metis/internal/app/itsm/domain"
	"metis/internal/app/itsm/prompts"
	"strconv"
	"strings"
	"time"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"

	"metis/internal/app/itsm/tools"
	"metis/internal/model"
)

func SeedITSM(db *gorm.DB, enforcer *casbin.Enforcer) error {
	migratePriorityCommitmentColumns(db)
	if err := MigrateTicketStatusModel(db); err != nil {
		return err
	}
	if err := migrateServiceDeskSubmissionIndex(db); err != nil {
		return err
	}

	if err := seedMenus(db); err != nil {
		return err
	}
	if err := seedCatalogs(db); err != nil {
		return err
	}
	if err := seedPolicies(enforcer); err != nil {
		return err
	}
	if err := seedPriorities(db); err != nil {
		return err
	}
	if err := seedSLATemplates(db); err != nil {
		return err
	}
	if err := tools.SeedTools(db); err != nil {
		return err
	}
	if err := tools.SeedAgents(db); err != nil {
		return err
	}
	if err := SeedEngineConfig(db); err != nil {
		return err
	}
	if err := seedServiceDefinitions(db); err != nil {
		return err
	}
	if err := migrateServiceRuntimeVersions(db); err != nil {
		return err
	}
	return RepairCompletedHumanAssignments(db)
}

func migrateServiceDeskSubmissionIndex(db *gorm.DB) error {
	if !db.Migrator().HasTable(&ServiceDeskSubmission{}) {
		return nil
	}
	if !db.Migrator().HasIndex(&ServiceDeskSubmission{}, "idx_itsm_submission_draft") {
		return db.AutoMigrate(&ServiceDeskSubmission{})
	}

	legacy, err := hasLegacyServiceDeskSubmissionIndex(db)
	if err != nil {
		return err
	}
	if !legacy {
		return nil
	}
	if err := db.Migrator().DropIndex(&ServiceDeskSubmission{}, "idx_itsm_submission_draft"); err != nil {
		return err
	}
	if err := db.AutoMigrate(&ServiceDeskSubmission{}); err != nil {
		return err
	}
	slog.Info("seed: rebuilt idx_itsm_submission_draft with request_hash")
	return nil
}

func hasLegacyServiceDeskSubmissionIndex(db *gorm.DB) (bool, error) {
	expected := "(session_id, draft_version, fields_hash, request_hash)"
	switch db.Dialector.Name() {
	case "postgres":
		var indexDef string
		row := db.Raw(`
			SELECT indexdef
			FROM pg_indexes
			WHERE schemaname = ANY (current_schemas(false))
			  AND tablename = ?
			  AND indexname = ?
			LIMIT 1
		`, (&ServiceDeskSubmission{}).TableName(), "idx_itsm_submission_draft").Row()
		if err := row.Scan(&indexDef); err != nil {
			return false, err
		}
		normalized := strings.ToLower(strings.Join(strings.Fields(indexDef), " "))
		return !strings.Contains(normalized, expected), nil
	case "sqlite":
		type indexInfoRow struct {
			Seqno int    `gorm:"column:seqno"`
			Name  string `gorm:"column:name"`
		}
		var rows []indexInfoRow
		if err := db.Raw("PRAGMA index_info('idx_itsm_submission_draft')").Scan(&rows).Error; err != nil {
			return false, err
		}
		if len(rows) != 4 {
			return true, nil
		}
		return rows[0].Name != "session_id" || rows[1].Name != "draft_version" || rows[2].Name != "fields_hash" || rows[3].Name != "request_hash", nil
	default:
		return false, nil
	}
}

func migrateServiceRuntimeVersions(db *gorm.DB) error {
	if err := db.AutoMigrate(&ServiceDefinitionVersion{}, &Ticket{}); err != nil {
		return err
	}
	var services []ServiceDefinition
	if err := db.Find(&services).Error; err != nil {
		return err
	}
	for _, svc := range services {
		if _, err := definition.GetOrCreateServiceRuntimeVersion(db, svc.ID); err != nil {
			return err
		}
	}

	var tickets []Ticket
	if err := db.Where("service_version_id IS NULL").Find(&tickets).Error; err != nil {
		return err
	}
	for _, ticket := range tickets {
		version, err := definition.GetOrCreateServiceRuntimeVersion(db, ticket.ServiceID)
		if err != nil {
			return err
		}
		if err := db.Model(&Ticket{}).Where("id = ? AND service_version_id IS NULL", ticket.ID).
			Update("service_version_id", version.ID).Error; err != nil {
			return err
		}
	}
	return nil
}

func MigrateTicketStatusModel(db *gorm.DB) error {
	if !db.Migrator().HasTable(&Ticket{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&Ticket{}, "outcome") {
		return nil
	}
	if db.Migrator().HasTable(&TicketActivity{}) {
		if err := migrateHumanActivityStatuses(db); err != nil {
			return err
		}
	}
	if db.Migrator().HasTable(&TicketAssignment{}) {
		if err := migrateAssignmentStatuses(db); err != nil {
			return err
		}
	}
	return migrateTicketStatuses(db)
}

func migrateHumanActivityStatuses(db *gorm.DB) error {
	return db.Model(&TicketActivity{}).
		Where("activity_type IN ?", []string{"approve", "form", "process"}).
		Where("status = ?", "completed").
		Where("transition_outcome IN ?", []string{"approved", "rejected"}).
		Update("status", gorm.Expr("transition_outcome")).Error
}

func migrateAssignmentStatuses(db *gorm.DB) error {
	type row struct {
		AssignmentID uint
		Outcome      string
	}
	var rows []row
	if err := db.Table("itsm_ticket_assignments AS assign").
		Select("assign.id AS assignment_id, act.transition_outcome AS outcome").
		Joins("JOIN itsm_ticket_activities AS act ON act.id = assign.activity_id").
		Where("assign.status = ?", "completed").
		Where("act.transition_outcome IN ?", []string{TicketOutcomeApproved, TicketOutcomeRejected}).
		Scan(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		status := AssignmentApproved
		if row.Outcome == TicketOutcomeRejected {
			status = AssignmentRejected
		}
		if err := db.Model(&TicketAssignment{}).Where("id = ?", row.AssignmentID).Update("status", status).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateTicketStatuses(db *gorm.DB) error {
	var tickets []Ticket
	if err := db.Find(&tickets).Error; err != nil {
		return err
	}
	for _, ticket := range tickets {
		status, outcome := deriveTicketStatusOutcome(db, ticket)
		if status == ticket.Status && outcome == ticket.Outcome {
			continue
		}
		updates := map[string]any{
			"status":  status,
			"outcome": outcome,
		}
		if IsTerminalTicketStatus(status) && ticket.FinishedAt == nil {
			now := time.Now()
			updates["finished_at"] = now
		}
		if err := db.Model(&Ticket{}).Where("id = ?", ticket.ID).Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

func deriveTicketStatusOutcome(db *gorm.DB, ticket Ticket) (string, string) {
	status := ticket.Status
	outcome := ticket.Outcome
	if status == "" {
		status = TicketStatusSubmitted
	}
	if isNewTicketStatus(status) {
		return normalizeNewTicketStatusOutcome(db, ticket.ID, status, outcome)
	}

	switch status {
	case "pending":
		return TicketStatusSubmitted, ""
	case "in_progress", "waiting_action":
		return deriveActiveTicketStatus(db, ticket.ID), ""
	case "failed":
		return TicketStatusFailed, TicketOutcomeFailed
	case "cancelled":
		if hasTimelineEvent(db, ticket.ID, "withdrawn") {
			return TicketStatusWithdrawn, TicketOutcomeWithdrawn
		}
		return TicketStatusCancelled, TicketOutcomeCancelled
	case "completed":
		lastOutcome := lastHumanOutcome(db, ticket.ID)
		if lastOutcome == TicketOutcomeRejected {
			return TicketStatusRejected, TicketOutcomeRejected
		}
		if lastOutcome == TicketOutcomeApproved {
			return TicketStatusCompleted, TicketOutcomeApproved
		}
		return TicketStatusCompleted, TicketOutcomeFulfilled
	default:
		return TicketStatusSubmitted, ""
	}
}

func normalizeNewTicketStatusOutcome(db *gorm.DB, ticketID uint, status string, outcome string) (string, string) {
	switch status {
	case TicketStatusSubmitted, TicketStatusWaitingHuman, TicketStatusApprovedDecisioning, TicketStatusRejectedDecisioning, TicketStatusDecisioning, TicketStatusExecutingAction:
		return status, ""
	case TicketStatusCompleted:
		if outcome == TicketOutcomeApproved || outcome == TicketOutcomeFulfilled {
			return TicketStatusCompleted, outcome
		}
		if outcome == TicketOutcomeRejected {
			return TicketStatusRejected, TicketOutcomeRejected
		}
		lastOutcome := lastHumanOutcome(db, ticketID)
		if lastOutcome == TicketOutcomeRejected {
			return TicketStatusRejected, TicketOutcomeRejected
		}
		if lastOutcome == TicketOutcomeApproved {
			return TicketStatusCompleted, TicketOutcomeApproved
		}
		return TicketStatusCompleted, TicketOutcomeFulfilled
	case TicketStatusRejected:
		if outcome == "" {
			outcome = TicketOutcomeRejected
		}
		return TicketStatusRejected, outcome
	case TicketStatusWithdrawn:
		if outcome == "" {
			outcome = TicketOutcomeWithdrawn
		}
		return TicketStatusWithdrawn, outcome
	case TicketStatusCancelled:
		if outcome == TicketOutcomeWithdrawn || hasTimelineEvent(db, ticketID, "withdrawn") {
			return TicketStatusWithdrawn, TicketOutcomeWithdrawn
		}
		if outcome == "" {
			outcome = TicketOutcomeCancelled
		}
		return TicketStatusCancelled, outcome
	case TicketStatusFailed:
		if outcome == "" {
			outcome = TicketOutcomeFailed
		}
		return TicketStatusFailed, outcome
	default:
		return TicketStatusSubmitted, ""
	}
}

func isNewTicketStatus(status string) bool {
	switch status {
	case TicketStatusSubmitted, TicketStatusWaitingHuman, TicketStatusApprovedDecisioning, TicketStatusRejectedDecisioning,
		TicketStatusDecisioning, TicketStatusExecutingAction, TicketStatusCompleted, TicketStatusRejected,
		TicketStatusWithdrawn, TicketStatusCancelled, TicketStatusFailed:
		return true
	default:
		return false
	}
}

func deriveActiveTicketStatus(db *gorm.DB, ticketID uint) string {
	var activity TicketActivity
	err := db.Where("ticket_id = ? AND status IN ?", ticketID, []string{"pending", "in_progress"}).
		Order("id DESC").
		First(&activity).Error
	if err != nil {
		return TicketStatusDecisioning
	}
	switch strings.TrimSpace(activity.ActivityType) {
	case "action", "notify":
		return TicketStatusExecutingAction
	case "approve", "form", "process", "wait":
		return TicketStatusWaitingHuman
	default:
		return TicketStatusDecisioning
	}
}

func lastHumanOutcome(db *gorm.DB, ticketID uint) string {
	var activity TicketActivity
	err := db.Where("ticket_id = ? AND activity_type IN ? AND transition_outcome IN ?", ticketID,
		[]string{"approve", "form", "process"}, []string{TicketOutcomeApproved, TicketOutcomeRejected}).
		Order("finished_at DESC, id DESC").
		First(&activity).Error
	if err != nil {
		return ""
	}
	return activity.TransitionOutcome
}

func hasTimelineEvent(db *gorm.DB, ticketID uint, eventType string) bool {
	var count int64
	db.Model(&TicketTimeline{}).Where("ticket_id = ? AND event_type = ?", ticketID, eventType).Count(&count)
	return count > 0
}

func migratePriorityCommitmentColumns(db *gorm.DB) {
	if !db.Migrator().HasTable(&Priority{}) {
		return
	}

	for _, column := range []string{"default_response_minutes", "default_resolution_minutes"} {
		if !db.Migrator().HasColumn("itsm_priorities", column) {
			continue
		}
		if err := db.Exec("ALTER TABLE itsm_priorities DROP COLUMN " + column).Error; err != nil {
			slog.Warn("seed: failed to drop priority commitment column", "column", column, "error", err)
			continue
		}
		slog.Info("seed: dropped priority commitment column", "column", column)
	}
}

func RepairCompletedHumanAssignments(db *gorm.DB) error {
	if !db.Migrator().HasTable(&TicketAssignment{}) ||
		!db.Migrator().HasTable(&TicketActivity{}) ||
		!db.Migrator().HasTable(&TicketTimeline{}) {
		return nil
	}

	type repairRow struct {
		AssignmentID       uint
		OperatorID         uint
		Outcome            string
		ActivityFinishedAt *time.Time
		TimelineCreatedAt  *time.Time
	}

	var rows []repairRow
	err := db.Table("itsm_ticket_assignments AS assign").
		Select(`
			assign.id AS assignment_id,
			tl.operator_id AS operator_id,
			act.transition_outcome AS outcome,
			act.finished_at AS activity_finished_at,
			tl.created_at AS timeline_created_at
		`).
		Joins("JOIN itsm_ticket_activities AS act ON act.id = assign.activity_id").
		Joins("JOIN itsm_ticket_timelines AS tl ON tl.ticket_id = act.ticket_id AND tl.activity_id = act.id").
		Where("act.activity_type IN ?", []string{"approve", "form", "process"}).
		Where("act.status IN ?", []string{"completed", TicketOutcomeApproved, TicketOutcomeRejected}).
		Where("assign.status = ?", "pending").
		Where("tl.event_type = ? AND tl.operator_id > 0", "activity_completed").
		Where("assign.user_id = tl.operator_id OR assign.assignee_id = tl.operator_id").
		Scan(&rows).Error
	if err != nil {
		return err
	}

	repaired := 0
	seen := make(map[uint]struct{}, len(rows))
	for _, row := range rows {
		if _, ok := seen[row.AssignmentID]; ok {
			continue
		}
		seen[row.AssignmentID] = struct{}{}

		finishedAt := row.ActivityFinishedAt
		if finishedAt == nil {
			finishedAt = row.TimelineCreatedAt
		}
		status := AssignmentApproved
		if row.Outcome == TicketOutcomeRejected {
			status = AssignmentRejected
		}
		updates := map[string]any{
			"assignee_id": row.OperatorID,
			"status":      status,
			"is_current":  false,
		}
		if finishedAt != nil {
			updates["finished_at"] = *finishedAt
		}
		result := db.Model(&TicketAssignment{}).
			Where("id = ? AND status = ?", row.AssignmentID, AssignmentPending).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			repaired++
		}
	}
	if repaired > 0 {
		slog.Info("seed: repaired completed ITSM assignments", "count", repaired)
	}
	return nil
}

func seedCatalogs(db *gorm.DB) error {
	type catalogSeed struct {
		Name        string
		Code        string
		Description string
		Icon        string
		SortOrder   int
		ParentCode  string // empty for root
	}

	seeds := []catalogSeed{
		// ── 一级域 ──────────────────────────────────────────
		{Name: "账号与权限", Code: "account-access", Description: "围绕身份、账户与访问控制的目录分类。", Icon: "ShieldCheck", SortOrder: 10},
		{Name: "终端与办公支持", Code: "workplace-support", Description: "围绕终端设备、办公环境与桌面支持的目录分类。", Icon: "Monitor", SortOrder: 20},
		{Name: "基础设施与网络", Code: "infra-network", Description: "围绕网络、主机、存储和基础运行环境的目录分类。", Icon: "Globe", SortOrder: 30},
		{Name: "应用与平台支持", Code: "application-platform", Description: "围绕企业应用、发布平台和数据库服务的目录分类。", Icon: "Container", SortOrder: 40},
		{Name: "安全与合规", Code: "security-compliance", Description: "围绕安全事件、漏洞治理与审计合规的目录分类。", Icon: "ShieldAlert", SortOrder: 50},
		{Name: "监控与告警", Code: "monitoring-alerting", Description: "围绕监控平台、告警治理和值班机制的目录分类。", Icon: "Bell", SortOrder: 60},

		// ── 账号与权限 子分类 ─────────────────────────────────
		{Name: "账号开通", Code: "account-access:provisioning", ParentCode: "account-access", Description: "员工账号开通、账号重建与账号合并。", Icon: "User", SortOrder: 1},
		{Name: "权限申请", Code: "account-access:authorization", ParentCode: "account-access", Description: "系统角色、数据权限与临时授权相关分类。", Icon: "Lock", SortOrder: 2},
		{Name: "密码与 MFA", Code: "account-access:credential", ParentCode: "account-access", Description: "密码重置、MFA 绑定与身份验证协助。", Icon: "KeyRound", SortOrder: 3},

		// ── 终端与办公支持 子分类 ─────────────────────────────
		{Name: "电脑与外设", Code: "workplace-support:endpoint", ParentCode: "workplace-support", Description: "笔记本、显示器、外设与桌面环境支持。", Icon: "Monitor", SortOrder: 1},
		{Name: "办公软件支持", Code: "workplace-support:office-software", ParentCode: "workplace-support", Description: "办公套件、协作工具与客户端故障处理。", Icon: "LayoutGrid", SortOrder: 2},
		{Name: "打印与会议室设备", Code: "workplace-support:meeting-room", ParentCode: "workplace-support", Description: "打印、投屏、音视频设备与会议室终端支持。", Icon: "Video", SortOrder: 3},

		// ── 基础设施与网络 子分类 ─────────────────────────────
		{Name: "网络与 VPN", Code: "infra-network:network", ParentCode: "infra-network", Description: "办公网络、专线、VPN 与连通性支持。", Icon: "Globe", SortOrder: 1},
		{Name: "服务器与主机", Code: "infra-network:compute", ParentCode: "infra-network", Description: "物理机、云主机与运行环境相关分类。", Icon: "Server", SortOrder: 2},
		{Name: "存储与备份", Code: "infra-network:storage", ParentCode: "infra-network", Description: "共享存储、对象存储与备份恢复支持。", Icon: "Database", SortOrder: 3},

		// ── 应用与平台支持 子分类 ─────────────────────────────
		{Name: "企业应用支持", Code: "application-platform:business-app", ParentCode: "application-platform", Description: "内部业务系统和通用平台的日常支持。", Icon: "LayoutGrid", SortOrder: 1},
		{Name: "发布与变更协助", Code: "application-platform:release", ParentCode: "application-platform", Description: "发布窗口、变更执行与回滚协助。", Icon: "Container", SortOrder: 2},
		{Name: "数据库支持", Code: "application-platform:database", ParentCode: "application-platform", Description: "数据库开通、巡检与性能支持。", Icon: "Database", SortOrder: 3},

		// ── 安全与合规 子分类 ─────────────────────────────────
		{Name: "安全事件协助", Code: "security-compliance:incident", ParentCode: "security-compliance", Description: "安全事件上报、分析与应急支持。", Icon: "Bell", SortOrder: 1},
		{Name: "漏洞与基线", Code: "security-compliance:vulnerability", ParentCode: "security-compliance", Description: "漏洞修复、基线加固与巡检协助。", Icon: "Bug", SortOrder: 2},
		{Name: "审计与合规支持", Code: "security-compliance:audit", ParentCode: "security-compliance", Description: "审计材料准备、合规检查与追踪协助。", Icon: "FileSearch", SortOrder: 3},

		// ── 监控与告警 子分类 ─────────────────────────────────
		{Name: "监控接入", Code: "monitoring-alerting:onboarding", ParentCode: "monitoring-alerting", Description: "新增监控项、采集接入和指标配置。", Icon: "LineChart", SortOrder: 1},
		{Name: "告警治理", Code: "monitoring-alerting:governance", ParentCode: "monitoring-alerting", Description: "告警收敛、规则优化与噪音治理。", Icon: "BellRing", SortOrder: 2},
		{Name: "值班与通知策略", Code: "monitoring-alerting:oncall", ParentCode: "monitoring-alerting", Description: "值班排班、升级策略和通知链路维护。", Icon: "Clock", SortOrder: 3},
	}

	// First pass: create root catalogs (no parent)
	for _, s := range seeds {
		if s.ParentCode != "" {
			continue
		}
		if ok, err := upsertSeedCatalog(db, ServiceCatalog{
			Name: s.Name, Code: s.Code, Description: s.Description,
			Icon: s.Icon, SortOrder: s.SortOrder, IsActive: true,
		}); err != nil {
			slog.Error("seed: failed to create catalog", "code", s.Code, "error", err)
			continue
		} else if !ok {
			continue
		}
		slog.Info("seed: created catalog", "code", s.Code, "name", s.Name)
	}

	// Second pass: create child catalogs
	for _, s := range seeds {
		if s.ParentCode == "" {
			continue
		}
		var existing ServiceCatalog
		if err := db.Where("code = ?", s.Code).First(&existing).Error; err == nil {
			continue
		}
		var parent ServiceCatalog
		if err := db.Where("code = ?", s.ParentCode).First(&parent).Error; err != nil {
			slog.Error("seed: parent catalog not found", "code", s.Code, "parentCode", s.ParentCode, "error", err)
			continue
		}
		cat := ServiceCatalog{
			Name: s.Name, Code: s.Code, Description: s.Description,
			Icon: s.Icon, ParentID: &parent.ID, SortOrder: s.SortOrder, IsActive: true,
		}
		if ok, err := upsertSeedCatalog(db, cat); err != nil {
			slog.Error("seed: failed to create catalog", "code", s.Code, "error", err)
			continue
		} else if !ok {
			continue
		}
		slog.Info("seed: created catalog", "code", s.Code, "name", s.Name)
	}

	return nil
}

func upsertSeedCatalog(db *gorm.DB, cat ServiceCatalog) (bool, error) {
	var existing ServiceCatalog
	if err := db.Where("code = ?", cat.Code).First(&existing).Error; err == nil {
		return false, nil
	}
	if err := db.Unscoped().Where("code = ?", cat.Code).First(&existing).Error; err == nil {
		updates := map[string]any{
			"name":        cat.Name,
			"description": cat.Description,
			"icon":        cat.Icon,
			"parent_id":   cat.ParentID,
			"sort_order":  cat.SortOrder,
			"is_active":   true,
			"deleted_at":  nil,
		}
		return true, db.Unscoped().Model(&ServiceCatalog{}).Where("id = ?", existing.ID).Updates(updates).Error
	}
	return true, db.Create(&cat).Error
}

func seedMenus(db *gorm.DB) error {
	// ITSM 顶级目录
	var itsmDir model.Menu
	if tx := db.Where("permission = ?", "itsm").Limit(1).Find(&itsmDir); tx.Error != nil {
		return tx.Error
	} else if tx.RowsAffected == 0 {
		itsmDir = model.Menu{
			Name:       "ITSM",
			Type:       model.MenuTypeDirectory,
			Icon:       "Headset",
			Permission: "itsm",
			Sort:       400,
		}
		if err := db.Create(&itsmDir).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", itsmDir.Name, "permission", itsmDir.Permission)
	}

	// Migrate: flatten old "工单管理" intermediate directory
	var ticketDir model.Menu
	if tx := db.Where("permission = ?", "itsm:ticket").Limit(1).Find(&ticketDir); tx.Error == nil && tx.RowsAffected > 0 {
		// Move children to ITSM top-level
		db.Model(&model.Menu{}).Where("parent_id = ?", ticketDir.ID).Update("parent_id", itsmDir.ID)
		// Soft-delete the intermediate directory
		db.Delete(&ticketDir)
		slog.Info("seed: flattened ticket menu directory", "oldId", ticketDir.ID)
	}

	// Migrate: remove standalone "服务目录" menu, catalog management is now inline in services page
	var oldCatalogMenu model.Menu
	if tx := db.Where("permission = ?", "itsm:catalog:list").Limit(1).Find(&oldCatalogMenu); tx.Error == nil && tx.RowsAffected > 0 {
		// Delete associated buttons
		db.Where("parent_id = ?", oldCatalogMenu.ID).Delete(&model.Menu{})
		db.Delete(&oldCatalogMenu)
		slog.Info("seed: removed standalone catalog menu", "oldId", oldCatalogMenu.ID)
	}

	// Migrate: rename "服务定义" to "服务目录" for unified workspace
	var existingServiceMenu model.Menu
	if tx := db.Where("permission = ?", "itsm:service:list").Limit(1).Find(&existingServiceMenu); tx.Error == nil && tx.RowsAffected > 0 {
		if existingServiceMenu.Name == "服务定义" {
			db.Model(&existingServiceMenu).Update("name", "服务目录")
			slog.Info("seed: renamed service menu to 服务目录")
		}
	}

	if err := db.Where("permission = ?", "itsm:ticket:history").Delete(&model.Menu{}).Error; err != nil {
		slog.Warn("seed: failed to remove history ticket menu", "error", err)
	}
	if err := db.Where("permission IN ?", []string{
		"itsm:ticket:todo",
		"itsm:ticket:approvals",
	}).Delete(&model.Menu{}).Error; err != nil {
		slog.Warn("seed: failed to remove obsolete approval menus", "error", err)
	}
	// 服务台
	seedMenu(db, &itsmDir.ID, "服务台", model.MenuTypeMenu, "/itsm/service-desk", "Headset", "itsm:service-desk:use", 0)

	// 我的工单
	seedMenu(db, &itsmDir.ID, "我的工单", model.MenuTypeMenu, "/itsm/tickets/mine", "User", "itsm:ticket:mine", 1)
	seedMenu(db, &itsmDir.ID, "我的待办", model.MenuTypeMenu, "/itsm/tickets/approvals/pending", "ClipboardCheck", "itsm:ticket:approval:pending", 2)
	seedMenu(db, &itsmDir.ID, "历史工单", model.MenuTypeMenu, "/itsm/tickets/approvals/history", "History", "itsm:ticket:approval:history", 3)

	// 服务目录 (unified workspace: catalogs + services)
	serviceMenu := seedMenu(db, &itsmDir.ID, "服务目录", model.MenuTypeMenu, "/itsm/services", "Cog", "itsm:service:list", 4)
	seedButtons(db, serviceMenu, []model.Menu{
		{Name: "新增服务", Type: model.MenuTypeButton, Permission: "itsm:service:create", Sort: 0},
		{Name: "编辑服务", Type: model.MenuTypeButton, Permission: "itsm:service:update", Sort: 1},
		{Name: "删除服务", Type: model.MenuTypeButton, Permission: "itsm:service:delete", Sort: 2},
		{Name: "新增分类", Type: model.MenuTypeButton, Permission: "itsm:catalog:create", Sort: 3},
		{Name: "编辑分类", Type: model.MenuTypeButton, Permission: "itsm:catalog:update", Sort: 4},
		{Name: "删除分类", Type: model.MenuTypeButton, Permission: "itsm:catalog:delete", Sort: 5},
	})

	// SLA 管理
	slaMenu := seedMenu(db, &itsmDir.ID, "SLA 管理", model.MenuTypeMenu, "/itsm/sla", "Timer", "itsm:sla:list", 6)
	seedButtons(db, slaMenu, []model.Menu{
		{Name: "新增SLA", Type: model.MenuTypeButton, Permission: "itsm:sla:create", Sort: 0},
		{Name: "编辑SLA", Type: model.MenuTypeButton, Permission: "itsm:sla:update", Sort: 1},
		{Name: "删除SLA", Type: model.MenuTypeButton, Permission: "itsm:sla:delete", Sort: 2},
	})

	// 优先级管理
	priorityMenu := seedMenu(db, &itsmDir.ID, "优先级管理", model.MenuTypeMenu, "/itsm/priorities", "Flag", "itsm:priority:list", 5)
	seedButtons(db, priorityMenu, []model.Menu{
		{Name: "新增优先级", Type: model.MenuTypeButton, Permission: "itsm:priority:create", Sort: 0},
		{Name: "编辑优先级", Type: model.MenuTypeButton, Permission: "itsm:priority:update", Sort: 1},
		{Name: "删除优先级", Type: model.MenuTypeButton, Permission: "itsm:priority:delete", Sort: 2},
	})

	// 工单监控
	allTicketMenu := seedMenu(db, &itsmDir.ID, "工单监控", model.MenuTypeMenu, "/itsm/tickets", "List", "itsm:ticket:list", 7)
	seedButtons(db, allTicketMenu, []model.Menu{
		{Name: "指派工单", Type: model.MenuTypeButton, Permission: "itsm:ticket:assign", Sort: 1},
		{Name: "取消工单", Type: model.MenuTypeButton, Permission: "itsm:ticket:cancel", Sort: 3},
		{Name: "工单覆写", Type: model.MenuTypeButton, Permission: "itsm:ticket:override", Sort: 4},
	})

	// 智能岗位
	seedMenu(db, &itsmDir.ID, "智能岗位", model.MenuTypeMenu, "/itsm/smart-staffing", "Briefcase", "itsm:smart-staffing:config", 10)
	// 引擎设置
	seedMenu(db, &itsmDir.ID, "引擎设置", model.MenuTypeMenu, "/itsm/engine-settings", "Settings", "itsm:engine-settings:config", 11)
	db.Where("permission = ?", "itsm:engine:config").Delete(&model.Menu{})

	// 表单管理 - migrated away: remove menu and buttons
	var formMenu model.Menu
	if tx := db.Where("permission = ?", "itsm:form:list").Limit(1).Find(&formMenu); tx.Error == nil && tx.RowsAffected > 0 {
		db.Where("parent_id = ?", formMenu.ID).Delete(&model.Menu{})
		db.Delete(&formMenu)
		slog.Info("seed: removed form management menu")
	}

	return nil
}

func seedMenu(db *gorm.DB, parentID *uint, name string, menuType model.MenuType, path, icon, permission string, sort int) *model.Menu {
	var menu model.Menu
	if tx := db.Unscoped().Where("permission = ?", permission).Limit(1).Find(&menu); tx.Error != nil {
		slog.Error("seed: failed to query menu", "permission", permission, "error", tx.Error)
		return nil
	} else if tx.RowsAffected == 0 {
		menu = model.Menu{
			ParentID:   parentID,
			Name:       name,
			Type:       menuType,
			Path:       path,
			Icon:       icon,
			Permission: permission,
			Sort:       sort,
		}
		if err := db.Create(&menu).Error; err != nil {
			slog.Error("seed: failed to create menu", "permission", permission, "error", err)
			return nil
		}
		slog.Info("seed: created menu", "name", menu.Name, "permission", menu.Permission)
	} else if menu.DeletedAt.Valid || menu.Name != name || menu.Type != menuType || menu.Path != path || menu.Icon != icon || menu.Sort != sort || !sameMenuParent(menu.ParentID, parentID) {
		if err := db.Unscoped().Model(&menu).Updates(map[string]any{
			"name":       name,
			"type":       menuType,
			"path":       path,
			"icon":       icon,
			"sort":       sort,
			"parent_id":  parentID,
			"deleted_at": nil,
		}).Error; err != nil {
			slog.Error("seed: failed to update menu", "permission", permission, "error", err)
			return nil
		}
		menu.Name = name
		menu.Type = menuType
		menu.Path = path
		menu.Icon = icon
		menu.Sort = sort
		menu.ParentID = parentID
		menu.DeletedAt.Valid = false
	}
	return &menu
}

func sameMenuParent(a *uint, b *uint) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func seedButtons(db *gorm.DB, parent *model.Menu, buttons []model.Menu) {
	if parent == nil {
		return
	}
	for _, btn := range buttons {
		var existing model.Menu
		if tx := db.Where("permission = ?", btn.Permission).Limit(1).Find(&existing); tx.Error != nil {
			slog.Error("seed: failed to query button", "permission", btn.Permission, "error", tx.Error)
			continue
		} else if tx.RowsAffected == 0 {
			btn.ParentID = &parent.ID
			if err := db.Create(&btn).Error; err != nil {
				slog.Error("seed: failed to create button", "permission", btn.Permission, "error", err)
				continue
			}
			slog.Info("seed: created menu", "name", btn.Name, "permission", btn.Permission)
		}
	}
}

func seedPolicies(enforcer *casbin.Enforcer) error {
	policies := [][]string{
		// Catalogs
		{"admin", "/api/v1/itsm/catalogs", "POST"},
		{"admin", "/api/v1/itsm/catalogs/tree", "GET"},
		{"admin", "/api/v1/itsm/catalogs/service-counts", "GET"},
		{"admin", "/api/v1/itsm/catalogs/:id", "PUT"},
		{"admin", "/api/v1/itsm/catalogs/:id", "DELETE"},
		// Services
		{"admin", "/api/v1/itsm/services", "POST"},
		{"admin", "/api/v1/itsm/services", "GET"},
		{"admin", "/api/v1/itsm/services/:id", "GET"},
		{"admin", "/api/v1/itsm/services/:id", "PUT"},
		{"admin", "/api/v1/itsm/services/:id", "DELETE"},
		{"admin", "/api/v1/itsm/services/:id/health", "GET"},
		// Service Actions
		{"admin", "/api/v1/itsm/services/:id/actions", "POST"},
		{"admin", "/api/v1/itsm/services/:id/actions", "GET"},
		{"admin", "/api/v1/itsm/services/:id/actions/:actionId", "PUT"},
		{"admin", "/api/v1/itsm/services/:id/actions/:actionId", "DELETE"},
		// Service Knowledge Documents
		{"admin", "/api/v1/itsm/services/:id/knowledge-documents", "POST"},
		{"admin", "/api/v1/itsm/services/:id/knowledge-documents", "GET"},
		{"admin", "/api/v1/itsm/services/:id/knowledge-documents/:docId", "DELETE"},
		// Smart Staffing
		{"admin", "/api/v1/itsm/smart-staffing/config", "GET"},
		{"admin", "/api/v1/itsm/smart-staffing/config", "PUT"},
		{"admin", "/api/v1/itsm/engine-settings/config", "GET"},
		{"admin", "/api/v1/itsm/engine-settings/config", "PUT"},
		// Workflow Generate
		{"admin", "/api/v1/itsm/workflows/generate", "POST"},
		{"admin", "/api/v1/itsm/workflows/capabilities", "GET"},
		// Service Desk
		{"user", "/api/v1/itsm/smart-staffing/config", "GET"},
		{"admin", "/api/v1/itsm/service-desk/sessions/:sid/state", "GET"},
		{"admin", "/api/v1/itsm/service-desk/sessions/:sid/draft/submit", "POST"},
		{"user", "/api/v1/itsm/service-desk/sessions/:sid/state", "GET"},
		{"user", "/api/v1/itsm/service-desk/sessions/:sid/draft/submit", "POST"},
		{"user", "/api/v1/ai/sessions", "GET"},
		{"user", "/api/v1/ai/sessions", "POST"},
		{"user", "/api/v1/ai/sessions/:sid", "GET"},
		{"user", "/api/v1/ai/sessions/:sid", "DELETE"},
		{"user", "/api/v1/ai/sessions/:sid/chat", "POST"},
		{"user", "/api/v1/ai/sessions/:sid/stream", "GET"},
		{"user", "/api/v1/ai/sessions/:sid/cancel", "POST"},
		{"user", "/api/v1/ai/sessions/:sid/images", "POST"},
		// Priorities
		{"admin", "/api/v1/itsm/priorities", "POST"},
		{"admin", "/api/v1/itsm/priorities", "GET"},
		{"admin", "/api/v1/itsm/priorities/:id", "PUT"},
		{"admin", "/api/v1/itsm/priorities/:id", "DELETE"},
		// SLA
		{"admin", "/api/v1/itsm/sla", "POST"},
		{"admin", "/api/v1/itsm/sla", "GET"},
		{"admin", "/api/v1/itsm/sla/notification-channels", "GET"},
		{"admin", "/api/v1/itsm/sla/:id", "PUT"},
		{"admin", "/api/v1/itsm/sla/:id", "DELETE"},
		// Escalation Rules
		{"admin", "/api/v1/itsm/sla/:id/escalations", "POST"},
		{"admin", "/api/v1/itsm/sla/:id/escalations", "GET"},
		{"admin", "/api/v1/itsm/sla/:id/escalations/:escalationId", "PUT"},
		{"admin", "/api/v1/itsm/sla/:id/escalations/:escalationId", "DELETE"},
		// Tickets
		{"admin", "/api/v1/itsm/tickets", "GET"},
		{"admin", "/api/v1/itsm/tickets", "POST"},
		{"admin", "/api/v1/itsm/tickets/mine", "GET"},
		{"user", "/api/v1/itsm/tickets/mine", "GET"},
		{"user", "/api/v1/itsm/tickets", "POST"},
		{"admin", "/api/v1/itsm/tickets/approvals/pending", "GET"},
		{"admin", "/api/v1/itsm/tickets/approvals/history", "GET"},
		{"user", "/api/v1/itsm/tickets/approvals/pending", "GET"},
		{"user", "/api/v1/itsm/tickets/approvals/history", "GET"},
		{"admin", "/api/v1/itsm/tickets/monitor", "GET"},
		{"admin", "/api/v1/itsm/tickets/decision-quality", "GET"},
		{"admin", "/api/v1/itsm/tickets/:id", "GET"},
		{"user", "/api/v1/itsm/tickets/:id", "GET"},
		{"admin", "/api/v1/itsm/tickets/:id/assign", "PUT"},
		{"admin", "/api/v1/itsm/tickets/:id/cancel", "PUT"},
		{"admin", "/api/v1/itsm/tickets/:id/timeline", "GET"},
		{"user", "/api/v1/itsm/tickets/:id/timeline", "GET"},
		// Classic engine routes
		{"admin", "/api/v1/itsm/tickets/:id/progress", "POST"},
		{"user", "/api/v1/itsm/tickets/:id/progress", "POST"},
		{"admin", "/api/v1/itsm/tickets/:id/signal", "POST"},
		{"admin", "/api/v1/itsm/tickets/:id/activities", "GET"},
		{"user", "/api/v1/itsm/tickets/:id/activities", "GET"},
		// Process variables
		{"admin", "/api/v1/itsm/tickets/:id/variables", "GET"},
		{"user", "/api/v1/itsm/tickets/:id/variables", "GET"},
		{"admin", "/api/v1/itsm/tickets/:id/override/jump", "POST"},
		{"admin", "/api/v1/itsm/tickets/:id/override/reassign", "POST"},
		{"admin", "/api/v1/itsm/tickets/:id/override/retry-ai", "POST"},
		{"admin", "/api/v1/itsm/tickets/:id/recovery", "POST"},
		{"admin", "/api/v1/itsm/tickets/:id/tokens", "GET"},
		{"user", "/api/v1/itsm/tickets/:id/tokens", "GET"},
		{"user", "/api/v1/itsm/tickets/:id/claim", "POST"},
	}

	menuPerms := [][]string{
		{"admin", "itsm", "read"},
		{"admin", "itsm:service-desk:use", "read"},
		{"user", "itsm", "read"},
		{"user", "itsm:service-desk:use", "read"},
		{"admin", "itsm:catalog:create", "read"},
		{"admin", "itsm:catalog:update", "read"},
		{"admin", "itsm:catalog:delete", "read"},
		{"admin", "itsm:service:list", "read"},
		{"admin", "itsm:service:create", "read"},
		{"admin", "itsm:service:update", "read"},
		{"admin", "itsm:service:delete", "read"},
		{"admin", "itsm:ticket", "read"},
		{"admin", "itsm:ticket:list", "read"},
		{"admin", "itsm:ticket:assign", "read"},
		{"admin", "itsm:ticket:cancel", "read"},
		{"admin", "itsm:ticket:override", "read"},
		{"admin", "itsm:ticket:mine", "read"},
		{"user", "itsm:ticket:mine", "read"},
		{"admin", "itsm:ticket:approval:pending", "read"},
		{"admin", "itsm:ticket:approval:history", "read"},
		{"user", "itsm:ticket:approval:pending", "read"},
		{"user", "itsm:ticket:approval:history", "read"},
		{"admin", "itsm:priority:list", "read"},
		{"admin", "itsm:priority:create", "read"},
		{"admin", "itsm:priority:update", "read"},
		{"admin", "itsm:priority:delete", "read"},
		{"admin", "itsm:sla:list", "read"},
		{"admin", "itsm:sla:create", "read"},
		{"admin", "itsm:sla:update", "read"},
		{"admin", "itsm:sla:delete", "read"},
		{"admin", "itsm:smart-staffing:config", "read"},
		{"admin", "itsm:engine-settings:config", "read"},
	}

	allPolicies := append(policies, menuPerms...)
	for _, p := range allPolicies {
		if has, _ := enforcer.HasPolicy(p); !has {
			if _, err := enforcer.AddPolicy(p); err != nil {
				slog.Error("seed: failed to add policy", "policy", p, "error", err)
			}
		}
	}

	_, _ = enforcer.RemovePolicy("admin", "/api/v1/itsm/engine/config", "GET")
	_, _ = enforcer.RemovePolicy("admin", "/api/v1/itsm/engine/config", "PUT")
	_, _ = enforcer.RemovePolicy("admin", "itsm:engine:config", "read")

	return nil
}

func seedPriorities(db *gorm.DB) error {
	priorities := []Priority{
		{Name: "紧急", Code: "P0", Value: 1, Color: "#FF0000", Description: "紧急问题，需要立即处理", IsActive: true},
		{Name: "高", Code: "P1", Value: 2, Color: "#FF6600", Description: "高优先级，需要尽快处理", IsActive: true},
		{Name: "中", Code: "P2", Value: 3, Color: "#FFAA00", Description: "中等优先级", IsActive: true},
		{Name: "低", Code: "P3", Value: 4, Color: "#00AA00", Description: "低优先级", IsActive: true},
		{Name: "最低", Code: "P4", Value: 5, Color: "#888888", Description: "最低优先级", IsActive: true},
	}

	for _, p := range priorities {
		var existing Priority
		if err := db.Where("code = ?", p.Code).First(&existing).Error; err != nil {
			if err := db.Create(&p).Error; err != nil {
				slog.Error("seed: failed to create priority", "code", p.Code, "error", err)
				continue
			}
			slog.Info("seed: created priority", "code", p.Code, "name", p.Name)
		}
	}

	return nil
}

func seedSLATemplates(db *gorm.DB) error {
	templates := []SLATemplate{
		{Name: "标准", Code: "standard", Description: "标准 SLA，响应 4 小时，解决 24 小时", ResponseMinutes: 240, ResolutionMinutes: 1440, IsActive: true},
		{Name: "紧急", Code: "urgent", Description: "紧急 SLA，响应 30 分钟，解决 4 小时", ResponseMinutes: 30, ResolutionMinutes: 240, IsActive: true},
		{Name: "快速办公支持", Code: "rapid-workplace", Description: "适用于办公终端、账号开通、基础软件支持等高频轻量服务", ResponseMinutes: 15, ResolutionMinutes: 120, IsActive: true},
		{Name: "关键业务", Code: "critical-business", Description: "适用于影响关键业务连续性的高优先级服务与紧急支持场景", ResponseMinutes: 10, ResolutionMinutes: 60, IsActive: true},
		{Name: "基础设施变更", Code: "infra-change", Description: "适用于服务器、网络、数据库等基础设施类服务和变更协作", ResponseMinutes: 60, ResolutionMinutes: 480, IsActive: true},
	}

	for _, t := range templates {
		var existing SLATemplate
		if err := db.Where("code = ?", t.Code).First(&existing).Error; err != nil {
			if err := db.Create(&t).Error; err != nil {
				slog.Error("seed: failed to create SLA template", "code", t.Code, "error", err)
				continue
			}
			slog.Info("seed: created SLA template", "code", t.Code, "name", t.Name)
		}
	}

	return nil
}

func seedServiceDefinitions(db *gorm.DB) error {
	// Look up the decision agent for smart services
	var decisionAgentID *uint
	var agentRow struct{ ID uint }
	if err := db.Table("ai_agents").Where("code = ?", "itsm.decision").Select("id").First(&agentRow).Error; err == nil {
		decisionAgentID = &agentRow.ID
		slog.Info("seed: found decision agent for smart services", "agentId", agentRow.ID)
	} else {
		slog.Warn("seed: decision agent (code=itsm.decision) not found, smart services will have no agent")
	}

	type serviceSeed struct {
		Name              string
		Code              string
		Description       string
		CatalogCode       string
		SLACode           string
		IntakeFormSchema  string
		WorkflowJSON      string
		CollaborationSpec string
		Actions           []ServiceAction
	}

	serviceRequestFormSchema := `{"version":1,"fields":[{"key":"title","type":"text","label":"请求标题","required":true,"validation":[{"rule":"required","message":"请输入请求标题"}],"width":"full"},{"key":"description","type":"textarea","label":"请求描述","required":true,"validation":[{"rule":"required","message":"请输入请求描述"}],"width":"full","props":{"rows":4}},{"key":"expected_date","type":"date","label":"期望完成日期","width":"half"},{"key":"remarks","type":"textarea","label":"备注","width":"full","props":{"rows":3}}],"layout":{"columns":2,"sections":[{"title":"请求信息","fields":["title","description"]},{"title":"补充信息","fields":["expected_date","remarks"]}]}}`
	bossSerialChangeFormSchema := `{"version":1,"fields":[{"key":"subject","type":"text","label":"申请主题","required":true,"validation":[{"rule":"required","message":"请输入申请主题"}],"width":"full"},{"key":"request_category","type":"select","label":"申请类别","required":true,"validation":[{"rule":"required","message":"请选择申请类别"}],"width":"half","options":[{"label":"生产变更","value":"prod_change"},{"label":"访问授权","value":"access_grant"},{"label":"应急支持","value":"emergency_support"}]},{"key":"risk_level","type":"radio","label":"风险等级","required":true,"validation":[{"rule":"required","message":"请选择风险等级"}],"width":"half","options":[{"label":"低","value":"low"},{"label":"中","value":"medium"},{"label":"高","value":"high"}]},{"key":"expected_finish_time","type":"datetime","label":"期望完成时间","width":"half"},{"key":"change_window","type":"date_range","label":"变更窗口","required":true,"validation":[{"rule":"required","message":"请选择变更窗口"}],"width":"half"},{"key":"impact_scope","type":"textarea","label":"影响范围","required":true,"validation":[{"rule":"required","message":"请输入影响范围"}],"width":"full","props":{"rows":3}},{"key":"rollback_required","type":"select","label":"回滚要求","required":true,"validation":[{"rule":"required","message":"请选择回滚要求"}],"width":"half","options":[{"label":"需要","value":"required"},{"label":"不需要","value":"not_required"}]},{"key":"impact_modules","type":"multi_select","label":"影响模块","required":true,"validation":[{"rule":"required","message":"请选择影响模块"}],"width":"half","options":[{"label":"网关","value":"gateway"},{"label":"支付","value":"payment"},{"label":"监控","value":"monitoring"},{"label":"订单","value":"order"}]},{"key":"change_items","type":"table","label":"变更明细表","required":true,"validation":[{"rule":"required","message":"请填写变更明细"}],"width":"full","props":{"columns":[{"key":"system","type":"text","label":"系统","required":true,"validation":[{"rule":"required","message":"请输入系统"}]},{"key":"resource","type":"text","label":"资源","required":true,"validation":[{"rule":"required","message":"请输入资源"}]},{"key":"permission_level","type":"select","label":"权限级别","required":true,"validation":[{"rule":"required","message":"请选择权限级别"}],"options":[{"label":"只读","value":"read"},{"label":"读写","value":"read_write"}]},{"key":"effective_range","type":"date_range","label":"生效时段","required":true,"validation":[{"rule":"required","message":"请选择生效时段"}]},{"key":"reason","type":"text","label":"变更理由","required":true,"validation":[{"rule":"required","message":"请输入变更理由"}]}]}}],"layout":{"columns":2,"sections":[{"title":"基础信息","fields":["subject","request_category","risk_level","expected_finish_time","change_window"]},{"title":"影响与回滚","fields":["impact_scope","rollback_required","impact_modules"]},{"title":"变更明细","fields":["change_items"]}]}}`
	dbBackupWhitelistFormSchema := `{"version":1,"fields":[{"key":"database_name","type":"text","label":"目标数据库","description":"填写需要临时加入备份白名单的生产数据库实例、库名或连接标识。","placeholder":"例如：prod-mysql-01","required":true,"validation":[{"rule":"required","message":"请输入目标数据库"}],"width":"half"},{"key":"source_ip","type":"text","label":"来源 IP","description":"填写发起备份访问的服务器、跳板机或备份任务来源 IP。","placeholder":"例如：10.20.30.50","required":true,"validation":[{"rule":"required","message":"请输入来源 IP"}],"width":"half"},{"key":"whitelist_window","type":"text","label":"白名单放行时间窗","description":"填写明确的开始和结束时间；系统会拒绝明天晚上、维护窗口等模糊时段。","placeholder":"例如：2026-05-01 22:00:00 ~ 2026-05-01 23:00:00","required":true,"validation":[{"rule":"required","message":"请输入白名单放行时间窗"}],"width":"full"},{"key":"access_reason","type":"textarea","label":"申请原因","description":"说明为什么本次生产数据库备份需要临时放行白名单。","placeholder":"例如：生产备份任务需要从备份服务器临时访问目标数据库","required":true,"validation":[{"rule":"required","message":"请输入申请原因"}],"width":"full","props":{"rows":4}}],"layout":{"columns":2,"sections":[{"title":"白名单放行信息","fields":["database_name","source_ip","whitelist_window"]},{"title":"申请原因","fields":["access_reason"]}]}}`
	vpnAccessFormSchema := `{"version":1,"fields":[{"key":"vpn_account","type":"text","label":"VPN账号","description":"用于登录 VPN 的账号；用户给出的邮箱可直接作为 VPN 账号。","placeholder":"例如：wenhaowu@dev.com","required":true,"validation":[{"rule":"required","message":"请输入 VPN 账号"}],"width":"half"},{"key":"device_usage","type":"textarea","label":"设备与用途说明","description":"说明访问 VPN 的设备或用途；用户已经说明用途时不必额外追问设备型号。","placeholder":"例如：线上支持用、长期远程办公访问内网","required":true,"validation":[{"rule":"required","message":"请输入设备与用途说明"}],"width":"full","props":{"rows":3}},{"key":"request_kind","type":"select","label":"访问原因","description":"选择 VPN 访问原因；系统按该字段路由到网络管理员或信息安全管理员。","placeholder":"请选择访问原因","required":true,"validation":[{"rule":"required","message":"请选择访问原因"}],"width":"half","options":[{"label":"线上支持","value":"online_support"},{"label":"故障排查","value":"troubleshooting"},{"label":"生产应急","value":"production_emergency"},{"label":"网络接入问题","value":"network_access_issue"},{"label":"外部协作","value":"external_collaboration"},{"label":"长期远程办公","value":"long_term_remote_work"},{"label":"跨境访问","value":"cross_border_access"},{"label":"安全合规事项","value":"security_compliance"}]}],"layout":{"columns":2,"sections":[{"title":"VPN 开通信息","fields":["vpn_account","device_usage","request_kind"]}]}}`
	vpnAccessWorkflowJSON := `{"nodes":[{"id":"start","type":"start","position":{"x":400,"y":50},"data":{"label":"开始","nodeType":"start"}},{"id":"request","type":"form","position":{"x":400,"y":200},"data":{"label":"填写 VPN 开通申请","nodeType":"form","participants":[{"type":"requester"}],"formSchema":{"fields":[{"key":"vpn_account","type":"text","label":"VPN账号"},{"key":"device_usage","type":"textarea","label":"设备与用途说明"},{"key":"request_kind","type":"select","label":"访问原因","options":["online_support","troubleshooting","production_emergency","network_access_issue","external_collaboration","long_term_remote_work","cross_border_access","security_compliance"]}]}}},{"id":"route","type":"exclusive","position":{"x":400,"y":380},"data":{"label":"访问原因路由","nodeType":"exclusive"}},{"id":"network_process","type":"process","position":{"x":160,"y":560},"data":{"label":"网络管理员处理","nodeType":"process","participants":[{"type":"position_department","department_code":"it","position_code":"network_admin"}]}},{"id":"security_process","type":"process","position":{"x":640,"y":560},"data":{"label":"信息安全管理员处理","nodeType":"process","participants":[{"type":"position_department","department_code":"it","position_code":"security_admin"}]}},{"id":"end","type":"end","position":{"x":400,"y":760},"data":{"label":"结束","nodeType":"end"}}],"edges":[{"id":"edge_start_request","source":"start","target":"request"},{"id":"edge_request_route","source":"request","target":"route"},{"id":"edge_route_network","source":"route","target":"network_process","data":{"condition":{"field":"form.request_kind","operator":"contains_any","value":["online_support","troubleshooting","production_emergency","network_access_issue"],"edge_id":"edge_route_network"}}},{"id":"edge_route_security","source":"route","target":"security_process","data":{"condition":{"field":"form.request_kind","operator":"contains_any","value":["external_collaboration","long_term_remote_work","cross_border_access","security_compliance"],"edge_id":"edge_route_security"}}},{"id":"edge_network_end","source":"network_process","target":"end"},{"id":"edge_security_end","source":"security_process","target":"end"}]}`

	serverAccessFormSchema := `{"version":1,"fields":[{"key":"target_servers","type":"textarea","label":"\u8bbf\u95ee\u670d\u52a1\u5668","description":"\u586b\u5199\u9700\u8981\u4e34\u65f6\u8bbf\u95ee\u7684\u751f\u4ea7\u670d\u52a1\u5668\u3001\u4e3b\u673a\u540d\u3001IP \u6216\u8d44\u6e90\u8303\u56f4\u3002","placeholder":"\u4f8b\u5982\uff1aprod-api-01 / 10.0.8.21\uff0c\u53ef\u586b\u591a\u53f0","required":true,"validation":[{"rule":"required","message":"\u8bf7\u586b\u5199\u8bbf\u95ee\u670d\u52a1\u5668"}],"width":"full","props":{"rows":3}},{"key":"access_window","type":"date_range","label":"\u8bbf\u95ee\u65f6\u6bb5","description":"\u4e34\u65f6\u8bbf\u95ee\u7684\u8d77\u6b62\u65f6\u95f4\u6216\u7ef4\u62a4\u7a97\u53e3\u3002","required":true,"validation":[{"rule":"required","message":"\u8bf7\u9009\u62e9\u8bbf\u95ee\u65f6\u6bb5"}],"width":"half","props":{"withTime":true,"mode":"datetime"}},{"key":"operation_purpose","type":"textarea","label":"\u64cd\u4f5c\u76ee\u7684","description":"\u8bf4\u660e\u672c\u6b21\u767b\u5f55\u6216\u8bbf\u95ee\u8981\u5b8c\u6210\u7684\u5177\u4f53\u64cd\u4f5c\u3002","placeholder":"\u4f8b\u5982\uff1a\u6392\u67e5\u5e94\u7528\u8fdb\u7a0b\u5f02\u5e38\u3001\u6293\u5305\u5b9a\u4f4d\u8fde\u901a\u6027\u95ee\u9898\u3001\u5b89\u5168\u53d6\u8bc1","required":true,"validation":[{"rule":"required","message":"\u8bf7\u586b\u5199\u64cd\u4f5c\u76ee\u7684"}],"width":"full","props":{"rows":3}},{"key":"access_reason","type":"textarea","label":"\u8bbf\u95ee\u539f\u56e0","description":"\u8bf7\u7528\u81ea\u7136\u8bed\u8a00\u8bf4\u660e\u8bbf\u95ee\u539f\u56e0\uff0c\u7cfb\u7edf\u5c06\u7531\u667a\u80fd\u5f15\u64ce\u5224\u65ad\u8fd0\u7ef4\u3001\u7f51\u7edc\u6216\u5b89\u5168\u8def\u7531\u3002","placeholder":"\u4f8b\u5982\uff1a\u751f\u4ea7\u53d1\u5e03\u540e\u9700\u8981\u67e5\u770b\u65e5\u5fd7\uff0c\u6216\u914d\u5408\u7f51\u7edc\u94fe\u8def\u8bca\u65ad\uff0c\u6216\u505a\u5b89\u5168\u5ba1\u8ba1\u53d6\u8bc1","required":true,"validation":[{"rule":"required","message":"\u8bf7\u586b\u5199\u8bbf\u95ee\u539f\u56e0"}],"width":"full","props":{"rows":4}}],"layout":{"columns":2,"sections":[{"title":"\u8bbf\u95ee\u8303\u56f4","fields":["target_servers","access_window"]},{"title":"\u76ee\u7684\u4e0e\u539f\u56e0","fields":["operation_purpose","access_reason"]}]}}`
	serverAccessWorkflowJSON := `{"nodes":[{"id":"start","type":"start","position":{"x":400,"y":50},"data":{"label":"开始","nodeType":"start"}},{"id":"request","type":"form","position":{"x":400,"y":200},"data":{"label":"填写服务器临时访问申请","nodeType":"form","participants":[{"type":"requester"}],"formSchema":{"fields":[{"key":"target_servers","type":"textarea","label":"访问服务器"},{"key":"access_window","type":"date_range","label":"访问时段"},{"key":"operation_purpose","type":"textarea","label":"操作目的"},{"key":"access_reason","type":"textarea","label":"访问原因"}]}}},{"id":"route","type":"exclusive","position":{"x":400,"y":380},"data":{"label":"访问原因智能参考路由","nodeType":"exclusive"}},{"id":"ops_process","type":"process","position":{"x":120,"y":580},"data":{"label":"运维管理员处理","nodeType":"process","participants":[{"type":"position_department","department_code":"it","position_code":"ops_admin"}]}},{"id":"network_process","type":"process","position":{"x":400,"y":580},"data":{"label":"网络管理员处理","nodeType":"process","participants":[{"type":"position_department","department_code":"it","position_code":"network_admin"}]}},{"id":"security_process","type":"process","position":{"x":680,"y":580},"data":{"label":"安全管理员处理","nodeType":"process","participants":[{"type":"position_department","department_code":"it","position_code":"security_admin"}]}},{"id":"end","type":"end","position":{"x":400,"y":820},"data":{"label":"结束","nodeType":"end"}}],"edges":[{"id":"edge_start_request","source":"start","target":"request"},{"id":"edge_request_route","source":"request","target":"route"},{"id":"edge_route_ops","source":"route","target":"ops_process","data":{"condition":{"field":"form.access_reason","operator":"contains_any","value":["应用发布","进程排障","日志排查","磁盘清理","主机巡检","生产运维操作"],"edge_id":"edge_route_ops"}}},{"id":"edge_route_network","source":"route","target":"network_process","data":{"condition":{"field":"form.access_reason","operator":"contains_any","value":["网络抓包","连通性诊断","ACL调整","负载均衡变更","防火墙策略调整"],"edge_id":"edge_route_network"}}},{"id":"edge_route_security","source":"route","target":"security_process","data":{"condition":{"field":"form.access_reason","operator":"contains_any","value":["安全审计","入侵排查","漏洞修复验证","取证分析","合规检查"],"edge_id":"edge_route_security"}}},{"id":"edge_route_default","source":"route","target":"security_process","data":{"default":true}},{"id":"edge_ops_end","source":"ops_process","target":"end","data":{"outcome":"approved"}},{"id":"edge_ops_rejected","source":"ops_process","target":"request","data":{"outcome":"rejected"}},{"id":"edge_network_end","source":"network_process","target":"end","data":{"outcome":"approved"}},{"id":"edge_network_rejected","source":"network_process","target":"request","data":{"outcome":"rejected"}},{"id":"edge_security_end","source":"security_process","target":"end","data":{"outcome":"approved"}},{"id":"edge_security_rejected","source":"security_process","target":"request","data":{"outcome":"rejected"}}]}`

	bossSerialChangeWorkflowJSON := `{"nodes":[{"id":"start","type":"start","position":{"x":400,"y":50},"data":{"label":"开始","nodeType":"start"}},{"id":"request","type":"form","position":{"x":400,"y":200},"data":{"label":"填写高风险变更申请","nodeType":"form","participants":[{"type":"requester"}],"formSchema":{"fields":[{"key":"subject","type":"text","label":"申请主题"},{"key":"request_category","type":"select","label":"申请类别"},{"key":"risk_level","type":"radio","label":"风险等级"},{"key":"change_window","type":"date_range","label":"变更窗口"},{"key":"impact_scope","type":"textarea","label":"影响范围"},{"key":"rollback_required","type":"select","label":"回滚要求"},{"key":"impact_modules","type":"multi_select","label":"影响模块"},{"key":"change_items","type":"table","label":"变更明细表"}]}}},{"id":"hq_review","type":"process","position":{"x":400,"y":400},"data":{"label":"总部处理人审核","nodeType":"process","participants":[{"type":"position_department","department_code":"headquarters","position_code":"serial_reviewer"}]}},{"id":"ops_review","type":"process","position":{"x":400,"y":600},"data":{"label":"运维管理员处理","nodeType":"process","participants":[{"type":"position_department","department_code":"it","position_code":"ops_admin"}]}},{"id":"end","type":"end","position":{"x":400,"y":800},"data":{"label":"结束","nodeType":"end"}}],"edges":[{"id":"edge_start_request","source":"start","target":"request"},{"id":"edge_request_hq","source":"request","target":"hq_review"},{"id":"edge_hq_approved","source":"hq_review","target":"ops_review","data":{"outcome":"approved"}},{"id":"edge_hq_rejected","source":"hq_review","target":"end","data":{"outcome":"rejected"}},{"id":"edge_ops_approved","source":"ops_review","target":"end","data":{"outcome":"approved"}},{"id":"edge_ops_rejected","source":"ops_review","target":"end","data":{"outcome":"rejected"}}]}`
	seeds := []serviceSeed{
		{
			Name:              "Copilot 账号申请",
			Code:              "copilot-account-request",
			Description:       "用于验证服务申请与管理员处理闭环的内置服务。",
			CatalogCode:       "account-access:provisioning",
			SLACode:           "rapid-workplace",
			IntakeFormSchema:  serviceRequestFormSchema,
			CollaborationSpec: "收集提单用户的 Github 账号信息和申请理由（可选），交给信息部 IT管理员处理。处理任务完成后结束流程。",
		},
		{
			Name:             "高风险变更协同申请（Boss）",
			Code:             "boss-serial-change-request",
			Description:      "用于在系统内直接查看复杂表单、表格明细与两级串行处理流程图的 Boss 级内置服务。",
			CatalogCode:      "application-platform:release",
			SLACode:          "infra-change",
			IntakeFormSchema: bossSerialChangeFormSchema,
			WorkflowJSON:     bossSerialChangeWorkflowJSON,
			CollaborationSpec: `员工在 IT 服务台提交高风险变更协同申请时，服务台需要确认申请主题、申请类别、风险等级、期望完成时间、变更窗口、影响范围、回滚要求、影响模块，以及每一项变更明细。
申请类别包括生产变更、访问授权和应急支持；风险等级包括低、中、高；回滚要求包括需要和不需要；影响模块可选择网关、支付、监控和订单。变更明细需要说明系统、资源、权限级别、生效时段和变更理由，权限级别包括只读和读写。
申请提交后，先交给总部处理人处理；总部处理人完成后，再交给信息部运维管理员处理。运维管理员完成处理后流程结束。`,
		},
		{
			Name:             "生产数据库备份白名单临时放行申请",
			Code:             "db-backup-whitelist-action-flow",
			Description:      "用于验证请求节点预检动作、处理后自动放行动作与工单闭环。",
			CatalogCode:      "application-platform:database",
			SLACode:          "infra-change",
			IntakeFormSchema: dbBackupWhitelistFormSchema,
			CollaborationSpec: `员工在 IT 服务台申请生产数据库备份白名单临时放行时，服务台需要确认目标数据库、发起备份访问的来源 IP、白名单放行时间窗，以及这次临时放行的申请原因。
申请资料收齐后，系统会先做一次白名单参数预检，确认数据库、来源 IP、放行窗口和申请原因满足放行前置条件。预检通过后，交给信息部数据库管理员处理。
数据库管理员完成处理后，系统执行备份白名单放行；放行成功后流程结束。驳回时不进入补充或返工，流程按驳回结果结束。`,
			Actions: []ServiceAction{
				{
					Name: "备份白名单预检", Code: "db_backup_whitelist_precheck",
					Description: "在申请人提交前校验数据库、时间窗与来源 IP 是否齐备。",
					ActionType:  "http", IsActive: true,
					ConfigJSON: JSONField(`{"url":"/precheck","method":"POST","timeout_seconds":5}`),
				},
				{
					Name: "执行备份白名单放行", Code: "db_backup_whitelist_apply",
					Description: "处理完成后自动执行数据库备份白名单放行。",
					ActionType:  "http", IsActive: true,
					ConfigJSON: JSONField(`{"url":"/apply","method":"POST","timeout_seconds":5}`),
				},
			},
		},
		{
			Name:             "生产服务器临时访问申请",
			Code:             "prod-server-temporary-access",
			Description:      "用于验证生产服务器临时访问在主机运维、网络诊断与安全审计语境下的真实分支处理。",
			CatalogCode:      "infra-network:compute",
			SLACode:          "critical-business",
			IntakeFormSchema: serverAccessFormSchema,
			WorkflowJSON:     serverAccessWorkflowJSON,
			CollaborationSpec: `员工在 IT 服务台申请生产服务器临时访问时，服务台需要确认要访问的服务器或资源范围、访问时段、本次操作目的，以及为什么需要临时进入生产环境。

访问原因通常分为三类：应用发布、进程排障、日志排查、磁盘清理、主机巡检、生产运维操作偏主机和应用运维，交给信息部运维管理员处理；网络抓包、连通性诊断、ACL 调整、负载均衡变更、防火墙策略调整偏网络诊断与策略处理，交给信息部网络管理员处理；安全审计、入侵排查、漏洞修复验证、取证分析、合规检查偏安全与合规风险，交给信息部信息安全管理员处理。

处理人完成处理后流程结束。`,
		},
		{
			Name:             "VPN 开通申请",
			Code:             "vpn-access-request",
			Description:      "用于验证 VPN 开通申请在服务匹配、拟提单确认与分支处理下的完整闭环。",
			CatalogCode:      "infra-network:network",
			SLACode:          "standard",
			IntakeFormSchema: vpnAccessFormSchema,
			WorkflowJSON:     vpnAccessWorkflowJSON,
			CollaborationSpec: `员工在 IT 服务台申请开通 VPN 时，服务台需要确认 VPN 账号、准备用什么设备或场景使用，以及这次访问的主要原因。
访问原因包括线上支持、故障排查、生产应急、网络接入问题、外部协作、长期远程办公、跨境访问和安全合规事项。
线上支持、故障排查、生产应急、网络接入问题偏网络连通与业务支持，交给信息部网络管理员处理；外部协作、长期远程办公、跨境访问、安全合规事项涉及外部、长期、跨境或合规风险，交给信息部信息安全管理员处理。
处理人完成处理后流程结束。`,
		},
	}

	for _, s := range seeds {
		var existing ServiceDefinition
		if err := db.Where("code = ?", s.Code).First(&existing).Error; err == nil {
			if existing.Description != s.Description || existing.CollaborationSpec != s.CollaborationSpec {
				if err := db.Model(&existing).Update("collaboration_spec", s.CollaborationSpec).Error; err != nil {
					slog.Error("seed: failed to update service collaboration spec", "code", s.Code, "error", err)
				} else {
					slog.Info("seed: updated service collaboration spec", "code", s.Code)
				}
				_ = db.Model(&existing).Update("description", s.Description).Error
			}
			if s.IntakeFormSchema != "" && string(existing.IntakeFormSchema) != s.IntakeFormSchema {
				if err := db.Model(&existing).Update("intake_form_schema", JSONField(s.IntakeFormSchema)).Error; err != nil {
					slog.Error("seed: failed to update service intake form schema", "code", s.Code, "error", err)
				} else {
					slog.Info("seed: updated service intake form schema", "code", s.Code)
				}
			}
			if s.WorkflowJSON != "" && string(existing.WorkflowJSON) != s.WorkflowJSON {
				if err := db.Model(&existing).Update("workflow_json", JSONField(s.WorkflowJSON)).Error; err != nil {
					slog.Error("seed: failed to update service workflow json", "code", s.Code, "error", err)
				} else {
					slog.Info("seed: updated service workflow json", "code", s.Code)
				}
			}
			seedServiceActions(db, s.Code, existing.ID, s.Actions)
			if s.Code == "db-backup-whitelist-action-flow" {
				seedDBBackupWhitelistWorkflow(db, existing.ID)
			}
			continue
		}

		var catalog ServiceCatalog
		if err := db.Where("code = ?", s.CatalogCode).First(&catalog).Error; err != nil {
			slog.Error("seed: catalog not found for service", "serviceCode", s.Code, "catalogCode", s.CatalogCode, "error", err)
			continue
		}

		var slaID *uint
		var sla SLATemplate
		if err := db.Where("code = ?", s.SLACode).First(&sla).Error; err == nil {
			slaID = &sla.ID
		} else {
			slog.Warn("seed: SLA not found for service, setting to nil", "serviceCode", s.Code, "slaCode", s.SLACode)
		}

		var intakeFormSchema JSONField
		if s.IntakeFormSchema != "" {
			intakeFormSchema = JSONField(s.IntakeFormSchema)
		}

		svc := ServiceDefinition{
			Name:              s.Name,
			Code:              s.Code,
			Description:       s.Description,
			CatalogID:         catalog.ID,
			EngineType:        "smart",
			SLAID:             slaID,
			IntakeFormSchema:  intakeFormSchema,
			WorkflowJSON:      JSONField(s.WorkflowJSON),
			AgentID:           decisionAgentID,
			CollaborationSpec: s.CollaborationSpec,
			IsActive:          true,
		}
		if err := db.Create(&svc).Error; err != nil {
			slog.Error("seed: failed to create service definition", "code", s.Code, "error", err)
			continue
		}
		slog.Info("seed: created service definition", "code", s.Code, "name", s.Name)

		seedServiceActions(db, s.Code, svc.ID, s.Actions)
		if s.Code == "db-backup-whitelist-action-flow" {
			seedDBBackupWhitelistWorkflow(db, svc.ID)
		}
	}

	// Backfill: update existing smart services that have no agent
	if decisionAgentID != nil {
		db.Model(&ServiceDefinition{}).
			Where("engine_type = ? AND agent_id IS NULL", "smart").
			Update("agent_id", *decisionAgentID)
	}

	return nil
}

func seedServiceActions(db *gorm.DB, serviceCode string, serviceID uint, actions []ServiceAction) {
	for _, action := range actions {
		if strings.TrimSpace(action.Code) == "" {
			continue
		}
		var existingAction ServiceAction
		if err := db.Where("service_id = ? AND code = ?", serviceID, action.Code).First(&existingAction).Error; err == nil {
			continue
		}

		migrated := false
		for _, legacyCode := range legacyActionCodesForCanonical(action.Code) {
			var legacyAction ServiceAction
			if err := db.Where("service_id = ? AND code = ?", serviceID, legacyCode).First(&legacyAction).Error; err != nil {
				continue
			}
			updates := map[string]any{
				"code":        action.Code,
				"name":        action.Name,
				"description": action.Description,
				"action_type": action.ActionType,
				"is_active":   action.IsActive,
			}
			if len(legacyAction.ConfigJSON) == 0 && len(action.ConfigJSON) > 0 {
				updates["config_json"] = action.ConfigJSON
			}
			if err := db.Model(&legacyAction).Updates(updates).Error; err != nil {
				slog.Error("seed: failed to migrate legacy service action", "serviceCode", serviceCode, "legacyCode", legacyCode, "actionCode", action.Code, "error", err)
			} else {
				slog.Info("seed: migrated legacy service action", "serviceCode", serviceCode, "legacyCode", legacyCode, "actionCode", action.Code)
			}
			migrated = true
			break
		}
		if migrated {
			continue
		}

		action.ServiceID = serviceID
		if err := db.Create(&action).Error; err != nil {
			slog.Error("seed: failed to create service action", "serviceCode", serviceCode, "actionCode", action.Code, "error", err)
			continue
		}
		slog.Info("seed: created service action", "serviceCode", serviceCode, "actionCode", action.Code)
	}
}

func seedDBBackupWhitelistWorkflow(db *gorm.DB, serviceID uint) {
	var actions []ServiceAction
	if err := db.Where("service_id = ? AND code IN ? AND is_active = ?", serviceID, []string{
		"db_backup_whitelist_precheck",
		"db_backup_whitelist_apply",
	}, true).Find(&actions).Error; err != nil {
		slog.Error("seed: failed to load db backup whitelist actions for workflow", "serviceID", serviceID, "error", err)
		return
	}

	var precheckActionID, applyActionID uint
	for _, action := range actions {
		switch action.Code {
		case "db_backup_whitelist_precheck":
			precheckActionID = action.ID
		case "db_backup_whitelist_apply":
			applyActionID = action.ID
		}
	}
	if precheckActionID == 0 || applyActionID == 0 {
		slog.Warn("seed: db backup whitelist workflow skipped because actions are missing", "serviceID", serviceID, "precheckActionID", precheckActionID, "applyActionID", applyActionID)
		return
	}

	workflowJSON := dbBackupWhitelistWorkflowJSON(precheckActionID, applyActionID)
	if err := db.Model(&ServiceDefinition{}).Where("id = ?", serviceID).Update("workflow_json", JSONField(workflowJSON)).Error; err != nil {
		slog.Error("seed: failed to update db backup whitelist workflow json", "serviceID", serviceID, "error", err)
		return
	}
	slog.Info("seed: updated db backup whitelist workflow json", "serviceID", serviceID)
}

func dbBackupWhitelistWorkflowJSON(precheckActionID, applyActionID uint) string {
	return fmt.Sprintf(`{"nodes":[{"id":"start","type":"start","position":{"x":120,"y":120},"data":{"label":"开始","nodeType":"start"}},{"id":"request","type":"form","position":{"x":380,"y":120},"data":{"label":"填写数据库备份白名单临时放行申请","nodeType":"form","participants":[{"type":"requester"}],"formSchema":{"fields":[{"key":"database_name","type":"text","label":"目标数据库"},{"key":"source_ip","type":"text","label":"来源 IP"},{"key":"whitelist_window","type":"text","label":"白名单放行时间窗"},{"key":"access_reason","type":"textarea","label":"申请原因"}]}}},{"id":"db_precheck_action","type":"action","position":{"x":680,"y":120},"data":{"label":"备份白名单预检","nodeType":"action","action_id":%d}},{"id":"db_process","type":"process","position":{"x":980,"y":120},"data":{"label":"数据库管理员处理","nodeType":"process","participants":[{"type":"position_department","department_code":"it","position_code":"db_admin"}]}},{"id":"db_apply_action","type":"action","position":{"x":1280,"y":120},"data":{"label":"执行备份白名单放行","nodeType":"action","action_id":%d}},{"id":"end","type":"end","position":{"x":1580,"y":120},"data":{"label":"结束","nodeType":"end"}}],"edges":[{"id":"edge_start_request","source":"start","target":"request","data":{}},{"id":"edge_request_precheck","source":"request","target":"db_precheck_action","data":{"outcome":"submitted"}},{"id":"edge_precheck_db","source":"db_precheck_action","target":"db_process","data":{"outcome":"success"}},{"id":"edge_db_apply_ok","source":"db_process","target":"db_apply_action","data":{"outcome":"approved"}},{"id":"edge_apply_end","source":"db_apply_action","target":"end","data":{"outcome":"success"}},{"id":"edge_db_end_reject","source":"db_process","target":"end","data":{"outcome":"rejected"}}]}`, precheckActionID, applyActionID)
}

func legacyActionCodesForCanonical(code string) []string {
	switch code {
	case "db_backup_whitelist_precheck":
		return []string{"backup_whitelist_precheck"}
	case "db_backup_whitelist_apply":
		return []string{"backup_whitelist_apply"}
	default:
		return nil
	}
}

// SeedEngineConfig creates default SystemConfig for smart staffing and engine settings.
func SeedEngineConfig(db *gorm.DB) error {
	migrateSmartTicketEngineConfig(db)

	// Seed default agent_id for smart staffing posts from preset agents.
	agentDefaults := map[string]string{
		SmartTicketIntakeAgentKey:       "itsm.servicedesk",
		SmartTicketDecisionAgentKey:     "itsm.decision",
		SmartTicketSLAAssuranceAgentKey: "itsm.sla_assurance",
	}
	for configKey, agentCode := range agentDefaults {
		var existing model.SystemConfig
		if err := db.Where("\"key\" = ?", configKey).First(&existing).Error; err == nil {
			continue
		}
		value := "0"
		var agentRow struct{ ID uint }
		if err := db.Table("ai_agents").Where("code = ?", agentCode).Select("id").First(&agentRow).Error; err == nil {
			value = strconv.FormatUint(uint64(agentRow.ID), 10)
		}
		cfg := model.SystemConfig{Key: configKey, Value: value}
		if err := db.Create(&cfg).Error; err != nil {
			slog.Error("seed: failed to create system config", "key", configKey, "error", err)
			continue
		}
		slog.Info("seed: created system config", "key", configKey, "value", value)
	}

	migratePathEngineAgentConfig(db)
	seedServiceMatchToolRuntime(db)

	defaults := map[string]string{
		SmartTicketDecisionModeKey:             "direct_first",
		SmartTicketPathModelKey:                "0",
		SmartTicketPathTemperatureKey:          "0.3",
		SmartTicketPathMaxRetriesKey:           "1",
		SmartTicketPathTimeoutKey:              "60",
		SmartTicketPathSystemPromptKey:         prompts.PathBuilderSystemPromptDefault,
		SmartTicketSessionTitleModelKey:        "0",
		SmartTicketSessionTitleTemperatureKey:  "0.2",
		SmartTicketSessionTitleMaxRetriesKey:   "1",
		SmartTicketSessionTitleTimeoutKey:      "30",
		SmartTicketSessionTitlePromptKey:       SessionTitleSystemPromptDefault,
		SmartTicketPublishHealthModelKey:       "0",
		SmartTicketPublishHealthTemperatureKey: "0.2",
		SmartTicketPublishHealthMaxRetriesKey:  "1",
		SmartTicketPublishHealthTimeoutKey:     "45",
		SmartTicketPublishHealthPromptKey:      prompts.PublishHealthSystemPromptDefault,
		SmartTicketGuardAuditLevelKey:          "full",
		SmartTicketGuardFallbackKey:            "0",
	}

	for key, value := range defaults {
		var existing model.SystemConfig
		if err := db.Where("\"key\" = ?", key).First(&existing).Error; err == nil {
			if (key == SmartTicketPathSystemPromptKey || key == SmartTicketPublishHealthPromptKey) && existing.Value != value {
				if err := db.Model(&model.SystemConfig{}).Where("\"key\" = ?", key).Update("value", value).Error; err != nil {
					slog.Error("seed: failed to sync system config", "key", key, "error", err)
				} else {
					slog.Info("seed: synced system config", "key", key)
				}
			}
			continue
		}
		cfg := model.SystemConfig{Key: key, Value: value}
		if err := db.Create(&cfg).Error; err != nil {
			slog.Error("seed: failed to create system config", "key", key, "error", err)
			continue
		}
		slog.Info("seed: created system config", "key", key, "value", value)
	}

	deleteLegacySmartTicketEngineConfig(db)
	deleteLegacyServiceMatcherEngineConfig(db)
	deleteLegacyPathBuilderAgents(db)

	return nil
}

func migrateSmartTicketEngineConfig(db *gorm.DB) {
	keyMap := map[string]string{
		"itsm.engine.servicedesk.agent_id":      SmartTicketIntakeAgentKey,
		"itsm.engine.decision.agent_id":         SmartTicketDecisionAgentKey,
		"itsm.engine.sla_assurance.agent_id":    SmartTicketSLAAssuranceAgentKey,
		"itsm.engine.decision.decision_mode":    SmartTicketDecisionModeKey,
		"itsm.engine.general.max_retries":       SmartTicketPathMaxRetriesKey,
		"itsm.engine.general.timeout_seconds":   SmartTicketPathTimeoutKey,
		"itsm.engine.general.reasoning_log":     SmartTicketGuardAuditLevelKey,
		"itsm.engine.general.fallback_assignee": SmartTicketGuardFallbackKey,
	}
	for legacyKey, newKey := range keyMap {
		var existing model.SystemConfig
		if err := db.Where("\"key\" = ?", newKey).First(&existing).Error; err == nil {
			continue
		}
		var legacy model.SystemConfig
		if err := db.Where("\"key\" = ?", legacyKey).First(&legacy).Error; err != nil {
			continue
		}
		cfg := model.SystemConfig{Key: newKey, Value: legacy.Value}
		if err := db.Create(&cfg).Error; err != nil {
			slog.Error("seed: failed to migrate system config", "from", legacyKey, "to", newKey, "error", err)
			continue
		}
		slog.Info("seed: migrated system config", "from", legacyKey, "to", newKey)
	}
}

func migratePathEngineAgentConfig(db *gorm.DB) {
	type pathAgentRow struct {
		ID          uint
		Code        *string
		ModelID     *uint
		Temperature float64
	}
	var rows []pathAgentRow
	if err := db.Table("ai_agents").
		Where("code IN ?", []string{"itsm.path_builder", "itsm.generator"}).
		Select("id", "code", "model_id", "temperature").
		Order("CASE WHEN code = 'itsm.path_builder' THEN 0 ELSE 1 END").
		Find(&rows).Error; err != nil {
		return
	}
	for _, row := range rows {
		if row.ModelID != nil {
			ensureSystemConfig(db, SmartTicketPathModelKey, strconv.FormatUint(uint64(*row.ModelID), 10))
			ensureSystemConfig(db, SmartTicketPathTemperatureKey, strconv.FormatFloat(row.Temperature, 'f', -1, 64))
			slog.Info("seed: migrated path engine config from legacy internal agent")
			return
		}
	}
}

func seedServiceMatchToolRuntime(db *gorm.DB) {
	if !db.Migrator().HasTable("ai_tools") || !db.Migrator().HasColumn("ai_tools", "runtime_config") {
		return
	}
	var toolRow struct {
		ID            uint
		RuntimeConfig string
	}
	if err := db.Table("ai_tools").Where("name = ?", "itsm.service_match").Select("id", "runtime_config").First(&toolRow).Error; err != nil {
		return
	}
	if toolRow.RuntimeConfig != "" && toolRow.RuntimeConfig != `{"modelId":0,"temperature":0.2,"maxTokens":1024,"timeoutSeconds":30}` {
		return
	}
	var agentRow struct {
		ModelID     *uint
		Temperature float64
		MaxTokens   int
	}
	if err := db.Table("ai_agents").Where("code = ?", "itsm.servicedesk").Select("model_id", "temperature", "max_tokens").First(&agentRow).Error; err != nil || agentRow.ModelID == nil {
		return
	}
	maxTokens := agentRow.MaxTokens
	if maxTokens < 256 || maxTokens > 8192 {
		maxTokens = 1024
	}
	timeoutSeconds := 30
	runtimeConfig := `{"modelId":` + strconv.FormatUint(uint64(*agentRow.ModelID), 10) +
		`,"temperature":` + strconv.FormatFloat(agentRow.Temperature, 'f', -1, 64) +
		`,"maxTokens":` + strconv.Itoa(maxTokens) +
		`,"timeoutSeconds":` + strconv.Itoa(timeoutSeconds) + `}`
	if err := db.Table("ai_tools").Where("id = ?", toolRow.ID).Update("runtime_config", runtimeConfig).Error; err != nil {
		slog.Warn("seed: failed to seed service match tool runtime", "error", err)
		return
	}
	slog.Info("seed: seeded service match tool runtime from service desk agent")
}

func ensureSystemConfig(db *gorm.DB, key string, value string) {
	var existing model.SystemConfig
	if err := db.Where("\"key\" = ?", key).First(&existing).Error; err == nil {
		return
	}
	cfg := model.SystemConfig{Key: key, Value: value}
	if err := db.Create(&cfg).Error; err != nil {
		slog.Error("seed: failed to create system config", "key", key, "error", err)
	}
}

func deleteLegacySmartTicketEngineConfig(db *gorm.DB) {
	legacyKeys := []string{
		"itsm.engine.servicedesk.agent_id",
		"itsm.engine.decision.agent_id",
		"itsm.engine.sla_assurance.agent_id",
		"itsm.engine.decision.decision_mode",
		"itsm.engine.general.max_retries",
		"itsm.engine.general.timeout_seconds",
		"itsm.engine.general.reasoning_log",
		"itsm.engine.general.fallback_assignee",
	}
	if err := db.Where("\"key\" IN ?", legacyKeys).Delete(&model.SystemConfig{}).Error; err != nil {
		slog.Warn("seed: failed to delete legacy smart ticket engine config", "error", err)
	}
}

func deleteLegacyServiceMatcherEngineConfig(db *gorm.DB) {
	keys := []string{
		"itsm.smart_ticket.service_matcher.model_id",
		"itsm.smart_ticket.service_matcher.temperature",
		"itsm.smart_ticket.service_matcher.max_tokens",
		"itsm.smart_ticket.service_matcher.timeout_seconds",
	}
	if err := db.Where("\"key\" IN ?", keys).Delete(&model.SystemConfig{}).Error; err != nil {
		slog.Warn("seed: failed to delete legacy service matcher engine config", "error", err)
	}
}

func deleteLegacyPathBuilderAgents(db *gorm.DB) {
	if err := db.Exec("DELETE FROM ai_agents WHERE code IN ?", []string{"itsm.path_builder", "itsm.generator"}).Error; err != nil {
		slog.Warn("seed: failed to delete legacy path agents", "error", err)
	}
}
