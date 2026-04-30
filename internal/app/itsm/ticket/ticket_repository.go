package ticket

import (
	"fmt"
	. "metis/internal/app/itsm/domain"
	"sync"
	"time"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
)

type TicketRepo struct {
	db    *database.DB
	seqMu sync.Mutex
}

func NewTicketRepo(i do.Injector) (*TicketRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &TicketRepo{db: db}, nil
}

// NextCode generates the next ticket code (TICK-XXXXXX) in a concurrency-safe manner.
func (r *TicketRepo) NextCode() (string, error) {
	r.seqMu.Lock()
	defer r.seqMu.Unlock()

	var maxID uint
	if err := r.db.Model(&Ticket{}).Select("COALESCE(MAX(id), 0)").Scan(&maxID).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("TICK-%06d", maxID+1), nil
}

func (r *TicketRepo) Create(t *Ticket) error {
	return r.db.Create(t).Error
}

func (r *TicketRepo) FindByID(id uint) (*Ticket, error) {
	var t Ticket
	if err := r.db.First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TicketRepo) FindByCode(code string) (*Ticket, error) {
	var t Ticket
	if err := r.db.Where("code = ?", code).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TicketRepo) assignmentOperatorCondition(alias string, userID uint, positionIDs []uint, deptIDs []uint) *gorm.DB {
	col := func(name string) string {
		return fmt.Sprintf("%s.%s", alias, name)
	}

	cond := r.db.Where(fmt.Sprintf("%s = ? OR %s = ?", col("user_id"), col("assignee_id")), userID, userID)
	if len(positionIDs) > 0 && len(deptIDs) > 0 {
		cond = cond.Or(
			r.db.Where(
				fmt.Sprintf("%s = ? AND %s IN ? AND %s IN ?", col("participant_type"), col("position_id"), col("department_id")),
				"position_department", positionIDs, deptIDs,
			),
		)
		cond = cond.Or(
			r.db.Where(
				fmt.Sprintf("COALESCE(%s, '') = '' AND %s IN ? AND %s IN ?", col("participant_type"), col("position_id"), col("department_id")),
				positionIDs, deptIDs,
			),
		)
	}
	if len(positionIDs) > 0 {
		cond = cond.Or(
			r.db.Where(
				fmt.Sprintf("%s = ? AND %s IN ?", col("participant_type"), col("position_id")),
				"position", positionIDs,
			),
		)
		cond = cond.Or(
			r.db.Where(
				fmt.Sprintf("COALESCE(%s, '') = '' AND %s IN ? AND %s IS NULL", col("participant_type"), col("position_id"), col("department_id")),
				positionIDs,
			),
		)
	}
	if len(deptIDs) > 0 {
		cond = cond.Or(
			r.db.Where(
				fmt.Sprintf("%s = ? AND %s IN ?", col("participant_type"), col("department_id")),
				"department", deptIDs,
			),
		)
		cond = cond.Or(
			r.db.Where(
				fmt.Sprintf("COALESCE(%s, '') = '' AND %s IN ? AND %s IS NULL", col("participant_type"), col("department_id"), col("position_id")),
				deptIDs,
			),
		)
	}
	return cond
}

func (r *TicketRepo) Update(id uint, updates map[string]any) error {
	return r.db.Model(&Ticket{}).Where("id = ?", id).Updates(updates).Error
}

type TicketListParams struct {
	Keyword     string
	Status      string
	EngineType  string
	PriorityID  *uint
	ServiceID   *uint
	AssigneeID  *uint
	RequesterID *uint
	StartDate   *time.Time
	EndDate     *time.Time
	Page        int
	PageSize    int
	DeptScope   *[]uint
}

type TicketMonitorParams struct {
	Keyword    string
	Status     string
	EngineType string
	RiskLevel  string
	MetricCode string
	PriorityID *uint
	ServiceID  *uint
	Page       int
	PageSize   int
	DeptScope  *[]uint
	OperatorID uint
}

type TicketApprovalListParams struct {
	Keyword  string
	Page     int
	PageSize int
}

func (r *TicketRepo) List(params TicketListParams) ([]Ticket, int64, error) {
	query := r.db.Model(&Ticket{})

	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		query = query.Where("code LIKE ? OR title LIKE ? OR description LIKE ?", like, like, like)
	}
	if params.Status != "" {
		query = applyTicketStatusFilter(query, params.Status)
	}
	if params.EngineType != "" {
		query = query.Where("engine_type = ?", params.EngineType)
	}
	if params.PriorityID != nil {
		query = query.Where("priority_id = ?", *params.PriorityID)
	}
	if params.ServiceID != nil {
		query = query.Where("service_id = ?", *params.ServiceID)
	}
	if params.AssigneeID != nil {
		query = query.Where("assignee_id = ?", *params.AssigneeID)
	}
	if params.RequesterID != nil {
		query = query.Where("requester_id = ?", *params.RequesterID)
	}
	if params.StartDate != nil {
		query = query.Where("created_at >= ?", *params.StartDate)
	}
	if params.EndDate != nil {
		query = query.Where("created_at <= ?", *params.EndDate)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page, pageSize := normalizePage(params.Page, params.PageSize)

	var items []Ticket
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *TicketRepo) ListMonitorBase(params TicketMonitorParams) ([]Ticket, error) {
	query := r.db.Model(&Ticket{})
	query = r.applyMonitorBaseFilters(query, params)

	var items []Ticket
	if err := query.Order("updated_at DESC, id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *TicketRepo) CountMonitorCompletedToday(params TicketMonitorParams, now time.Time) (int64, error) {
	start := time.Date(now.Local().Year(), now.Local().Month(), now.Local().Day(), 0, 0, 0, 0, now.Local().Location())
	end := start.Add(24 * time.Hour)
	query := r.db.Model(&Ticket{})
	query = r.applyMonitorBaseFilters(query, params).
		Where("status = ? AND finished_at >= ? AND finished_at < ?", TicketStatusCompleted, start, end)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *TicketRepo) applyMonitorBaseFilters(query *gorm.DB, params TicketMonitorParams) *gorm.DB {
	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		query = query.Where("code LIKE ? OR title LIKE ? OR description LIKE ?", like, like, like)
	}
	if params.Status != "" {
		query = applyTicketStatusFilter(query, params.Status)
	}
	if params.EngineType != "" {
		query = query.Where("engine_type = ?", params.EngineType)
	}
	if params.PriorityID != nil {
		query = query.Where("priority_id = ?", *params.PriorityID)
	}
	if params.ServiceID != nil {
		query = query.Where("service_id = ?", *params.ServiceID)
	}
	if params.Status == "" {
		now := time.Now()
		start := time.Date(now.Local().Year(), now.Local().Month(), now.Local().Day(), 0, 0, 0, 0, now.Local().Location())
		end := start.Add(24 * time.Hour)
		query = query.Where(
			"status NOT IN ? OR (status = ? AND finished_at >= ? AND finished_at < ?)",
			TerminalTicketStatuses(), TicketStatusCompleted, start, end,
		)
	}
	query = r.applyMonitorDataScope(query, params)
	return query
}

func (r *TicketRepo) applyMonitorDataScope(query *gorm.DB, params TicketMonitorParams) *gorm.DB {
	if params.DeptScope == nil {
		return query
	}
	if len(*params.DeptScope) == 0 {
		if params.OperatorID == 0 {
			return query.Where("1 = 0")
		}
		return query.Where(
			"requester_id = ? OR assignee_id = ? OR EXISTS ("+
				"SELECT 1 FROM itsm_ticket_assignments a "+
				"WHERE a.ticket_id = itsm_tickets.id AND a.deleted_at IS NULL "+
				"AND (a.user_id = ? OR a.assignee_id = ?))",
			params.OperatorID, params.OperatorID, params.OperatorID, params.OperatorID,
		)
	}
	deptIDs := *params.DeptScope
	return query.Where(
		"requester_id IN (SELECT user_id FROM user_positions WHERE department_id IN ? AND deleted_at IS NULL) "+
			"OR assignee_id IN (SELECT user_id FROM user_positions WHERE department_id IN ? AND deleted_at IS NULL) "+
			"OR EXISTS ("+
			"SELECT 1 FROM itsm_ticket_assignments a "+
			"WHERE a.ticket_id = itsm_tickets.id AND a.deleted_at IS NULL AND ("+
			"a.department_id IN ? "+
			"OR a.user_id IN (SELECT user_id FROM user_positions WHERE department_id IN ? AND deleted_at IS NULL) "+
			"OR a.assignee_id IN (SELECT user_id FROM user_positions WHERE department_id IN ? AND deleted_at IS NULL)))",
		deptIDs, deptIDs, deptIDs, deptIDs, deptIDs,
	)
}

func applyTicketStatusFilter(query *gorm.DB, status string) *gorm.DB {
	switch status {
	case "active":
		return query.Where("status NOT IN ?", TerminalTicketStatuses())
	case "terminal":
		return query.Where("status IN ?", TerminalTicketStatuses())
	case TicketStatusDecisioning:
		return query.Where("status IN ?", []string{
			TicketStatusDecisioning,
			TicketStatusApprovedDecisioning,
			TicketStatusRejectedDecisioning,
		})
	default:
		return query.Where("status = ?", status)
	}
}

func (r *TicketRepo) ListPendingApprovals(params TicketApprovalListParams, operatorID uint, positionIDs []uint, departmentIDs []uint) ([]Ticket, int64, error) {
	query := r.db.Model(&Ticket{}).
		Joins("JOIN itsm_ticket_activities AS act ON act.ticket_id = itsm_tickets.id").
		Joins("JOIN itsm_ticket_assignments AS assign ON assign.ticket_id = itsm_tickets.id AND assign.activity_id = act.id").
		Where("itsm_tickets.status NOT IN ?", TerminalTicketStatuses()).
		Where("act.activity_type IN ? AND act.status IN ?", []string{"approve", "form", "process"}, []string{"pending", "in_progress"}).
		Where("assign.status = ?", AssignmentPending).
		Where(r.assignmentOperatorCondition("assign", operatorID, positionIDs, departmentIDs))

	return r.listApprovalQuery(query, params, true)
}

func (r *TicketRepo) ListApprovalHistory(params TicketApprovalListParams, operatorID uint) ([]Ticket, int64, error) {
	query := r.db.Model(&Ticket{}).
		Joins("JOIN itsm_ticket_activities AS act ON act.ticket_id = itsm_tickets.id").
		Joins("JOIN itsm_ticket_assignments AS assign ON assign.ticket_id = itsm_tickets.id AND assign.activity_id = act.id").
		Where("act.activity_type IN ?", []string{"approve", "form", "process"}).
		Where("assign.status IN ? AND assign.assignee_id = ?", []string{AssignmentApproved, AssignmentRejected}, operatorID)

	return r.listApprovalQuery(query, params, false)
}

func (r *TicketRepo) listApprovalQuery(query *gorm.DB, params TicketApprovalListParams, pending bool) ([]Ticket, int64, error) {
	query = query.Joins("LEFT JOIN itsm_priorities ON itsm_priorities.id = itsm_tickets.priority_id")
	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		query = query.Where("itsm_tickets.code LIKE ? OR itsm_tickets.title LIKE ? OR itsm_tickets.description LIKE ?", like, like, like)
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Distinct("itsm_tickets.id").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page, pageSize := normalizePage(params.Page, params.PageSize)

	offset := (page - 1) * pageSize
	pageQuery := query.Session(&gorm.Session{}).Select("itsm_tickets.id")
	if pending {
		pageQuery = pageQuery.Group("itsm_tickets.id, itsm_priorities.value, itsm_tickets.created_at").
			Order("itsm_priorities.value ASC, itsm_tickets.created_at ASC, itsm_tickets.id ASC")
	} else {
		pageQuery = pageQuery.Group("itsm_tickets.id").
			Order("MAX(assign.finished_at) DESC, itsm_tickets.id DESC")
	}

	var ids []uint
	if err := pageQuery.Offset(offset).Limit(pageSize).Pluck("itsm_tickets.id", &ids).Error; err != nil {
		return nil, 0, err
	}
	if len(ids) == 0 {
		return []Ticket{}, total, nil
	}

	var rawItems []Ticket
	if err := r.db.Model(&Ticket{}).Where("id IN ?", ids).Find(&rawItems).Error; err != nil {
		return nil, 0, err
	}

	itemByID := make(map[uint]Ticket, len(rawItems))
	for _, item := range rawItems {
		itemByID[item.ID] = item
	}

	items := make([]Ticket, 0, len(ids))
	for _, id := range ids {
		item, ok := itemByID[id]
		if !ok {
			continue
		}
		items = append(items, item)
	}

	return items, total, nil
}

// UpdateInTx performs an update within a provided transaction.
func (r *TicketRepo) UpdateInTx(tx *gorm.DB, id uint, updates map[string]any) error {
	return tx.Model(&Ticket{}).Where("id = ?", id).Updates(updates).Error
}

func (r *TicketRepo) DB() *gorm.DB {
	return r.db.DB
}
