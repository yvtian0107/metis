package bootstrap

import (
	"encoding/json"
	. "metis/internal/app/itsm/domain"
	itsmtools "metis/internal/app/itsm/tools"
	"strings"
	"testing"

	"metis/internal/model"
)

func TestSeedCatalogs_CreatesExpectedRootsAndChildren(t *testing.T) {
	db := newTestDB(t)

	if err := seedCatalogs(db); err != nil {
		t.Fatalf("seed catalogs: %v", err)
	}

	var count int64
	if err := db.Model(&ServiceCatalog{}).Count(&count).Error; err != nil {
		t.Fatalf("count catalogs: %v", err)
	}
	if count != 24 {
		t.Fatalf("expected 24 catalogs, got %d", count)
	}

	var roots int64
	if err := db.Model(&ServiceCatalog{}).Where("parent_id IS NULL").Count(&roots).Error; err != nil {
		t.Fatalf("count roots: %v", err)
	}
	if roots != 6 {
		t.Fatalf("expected 6 roots, got %d", roots)
	}
}

func TestMigrateServiceRuntimeVersions_BackfillsServicesAndLegacyTickets(t *testing.T) {
	db := newTestDB(t)
	catalog := ServiceCatalog{Name: "Root", Code: "runtime-root", IsActive: true}
	if err := db.Create(&catalog).Error; err != nil {
		t.Fatalf("create catalog: %v", err)
	}
	service := ServiceDefinition{
		Name:              "Runtime Service",
		Code:              "runtime-service",
		CatalogID:         catalog.ID,
		EngineType:        "smart",
		CollaborationSpec: "initial spec",
		IsActive:          true,
	}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	ticket := Ticket{
		Code:        "TICK-LEGACY-RUNTIME",
		Title:       "legacy ticket",
		ServiceID:   service.ID,
		EngineType:  "smart",
		Status:      TicketStatusDecisioning,
		PriorityID:  1,
		RequesterID: 1,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create legacy ticket: %v", err)
	}

	if err := migrateServiceRuntimeVersions(db); err != nil {
		t.Fatalf("migrate service runtime versions: %v", err)
	}
	if err := migrateServiceRuntimeVersions(db); err != nil {
		t.Fatalf("migrate service runtime versions second run: %v", err)
	}

	var versions []ServiceDefinitionVersion
	if err := db.Where("service_id = ?", service.ID).Find(&versions).Error; err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected one idempotent runtime version, got %+v", versions)
	}
	var updated Ticket
	if err := db.First(&updated, ticket.ID).Error; err != nil {
		t.Fatalf("load ticket: %v", err)
	}
	if updated.ServiceVersionID == nil || *updated.ServiceVersionID != versions[0].ID {
		t.Fatalf("expected legacy ticket backfilled with version %d, got %v", versions[0].ID, updated.ServiceVersionID)
	}
}

func TestSeedServiceDefinitions_ServerAccessUsesNaturalSpecAndPreservesStructuredContract(t *testing.T) {
	db := newTestDB(t)

	if err := seedCatalogs(db); err != nil {
		t.Fatalf("seed catalogs: %v", err)
	}
	if err := seedSLATemplates(db); err != nil {
		t.Fatalf("seed SLA templates: %v", err)
	}
	if err := seedServiceDefinitions(db); err != nil {
		t.Fatalf("seed service definitions: %v", err)
	}

	var service ServiceDefinition
	if err := db.Where("code = ?", "prod-server-temporary-access").First(&service).Error; err != nil {
		t.Fatalf("find server access service: %v", err)
	}

	for _, forbidden := range []string{
		"target_servers",
		"access_window",
		"operation_purpose",
		"access_reason",
		"form.access_reason",
		"position_department",
		"department_code",
		"position_code",
		"ops_admin",
		"network_admin",
		"security_admin",
	} {
		if strings.Contains(service.CollaborationSpec, forbidden) {
			t.Fatalf("server access collaboration spec should be natural text, found machine token %q in %q", forbidden, service.CollaborationSpec)
		}
	}

	var schema struct {
		Fields []struct {
			Key  string `json:"key"`
			Type string `json:"type"`
		} `json:"fields"`
	}
	if err := json.Unmarshal([]byte(service.IntakeFormSchema), &schema); err != nil {
		t.Fatalf("unmarshal intake form schema: %v", err)
	}
	got := make([]string, 0, len(schema.Fields))
	for _, field := range schema.Fields {
		got = append(got, field.Key)
		if field.Key == "access_reason" && field.Type != "textarea" {
			t.Fatalf("expected access_reason to remain free text textarea, got %q", field.Type)
		}
	}
	want := []string{"target_servers", "access_window", "operation_purpose", "access_reason"}
	if len(got) != len(want) {
		t.Fatalf("expected field keys %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected field keys %v, got %v", want, got)
		}
	}

	operator := itsmtools.NewOperator(db, nil, nil, nil, nil, nil)
	detail, err := operator.LoadService(service.ID)
	if err != nil {
		t.Fatalf("load service through operator: %v", err)
	}
	if len(service.WorkflowJSON) == 0 {
		t.Fatal("expected seeded server access workflow json")
	}
	if detail.FormSchema == nil {
		t.Fatal("expected operator detail to include form schema")
	}
	if len(detail.FormFields) != len(want) {
		t.Fatalf("expected %d operator form fields, got %d", len(want), len(detail.FormFields))
	}
	for i, key := range want {
		if detail.FormFields[i].Key != key {
			t.Fatalf("expected operator field keys %v, got %+v", want, detail.FormFields)
		}
	}
	if detail.RoutingFieldHint != nil {
		t.Fatalf("expected textarea routing field to be ignored, got %+v", detail.RoutingFieldHint)
	}

	workflow := string(service.WorkflowJSON)
	for _, required := range []string{"form.access_reason", "it", "ops_admin", "network_admin", "security_admin"} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("expected server access workflow to preserve structured token %q, got %s", required, workflow)
		}
	}
}

func TestSeedServiceDefinitions_DBBackupUsesNaturalSpecAndPreservesStructuredContract(t *testing.T) {
	db := newTestDB(t)

	if err := seedCatalogs(db); err != nil {
		t.Fatalf("seed catalogs: %v", err)
	}
	if err := seedSLATemplates(db); err != nil {
		t.Fatalf("seed SLA templates: %v", err)
	}
	if err := seedServiceDefinitions(db); err != nil {
		t.Fatalf("seed service definitions: %v", err)
	}

	var service ServiceDefinition
	if err := db.Where("code = ?", "db-backup-whitelist-action-flow").First(&service).Error; err != nil {
		t.Fatalf("find db backup service: %v", err)
	}

	for _, forbidden := range []string{
		"database_name",
		"source_ip",
		"whitelist_window",
		"access_reason",
		"position_department",
		"department_code",
		"position_code",
		"db_admin",
		"decision.execute_action",
		"db_backup_whitelist_precheck",
		"db_backup_whitelist_apply",
		"backup_whitelist_precheck",
		"backup_whitelist_apply",
	} {
		if strings.Contains(service.CollaborationSpec, forbidden) {
			t.Fatalf("db backup collaboration spec should be natural text, found machine token %q in %q", forbidden, service.CollaborationSpec)
		}
	}

	var schema struct {
		Fields []struct {
			Key  string `json:"key"`
			Type string `json:"type"`
		} `json:"fields"`
	}
	if err := json.Unmarshal([]byte(service.IntakeFormSchema), &schema); err != nil {
		t.Fatalf("unmarshal db backup intake form schema: %v", err)
	}
	want := []struct {
		key string
		typ string
	}{
		{"database_name", "text"},
		{"source_ip", "text"},
		{"whitelist_window", "text"},
		{"access_reason", "textarea"},
	}
	if len(schema.Fields) != len(want) {
		t.Fatalf("expected field keys %v, got %+v", want, schema.Fields)
	}
	for i, field := range schema.Fields {
		if field.Key != want[i].key || field.Type != want[i].typ {
			t.Fatalf("expected field %d to be %s/%s, got %s/%s", i, want[i].key, want[i].typ, field.Key, field.Type)
		}
	}

	var actions []ServiceAction
	if err := db.Where("service_id = ?", service.ID).Order("code ASC").Find(&actions).Error; err != nil {
		t.Fatalf("load db backup actions: %v", err)
	}
	actionIDsByCode := map[string]uint{}
	for _, action := range actions {
		actionIDsByCode[action.Code] = action.ID
	}
	for _, code := range []string{"db_backup_whitelist_precheck", "db_backup_whitelist_apply"} {
		if actionIDsByCode[code] == 0 {
			t.Fatalf("expected seeded db backup action %q, got %#v", code, actionIDsByCode)
		}
	}

	var workflow struct {
		Nodes []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
			Data struct {
				Label    string `json:"label"`
				ActionID uint   `json:"action_id"`
			} `json:"data"`
		} `json:"nodes"`
		Edges []struct {
			Source string `json:"source"`
			Target string `json:"target"`
			Data   struct {
				Outcome string `json:"outcome"`
			} `json:"data"`
		} `json:"edges"`
	}
	if err := json.Unmarshal([]byte(service.WorkflowJSON), &workflow); err != nil {
		t.Fatalf("unmarshal db backup workflow json: %v", err)
	}
	actionNodeIDs := map[uint]string{}
	for _, node := range workflow.Nodes {
		if node.Type == "action" {
			actionNodeIDs[node.Data.ActionID] = node.ID
			if node.Data.ActionID == actionIDsByCode["db_backup_whitelist_precheck"] && !strings.Contains(node.Data.Label, "预检") {
				t.Fatalf("expected precheck action node label to mention precheck, got %q", node.Data.Label)
			}
			if node.Data.ActionID == actionIDsByCode["db_backup_whitelist_apply"] && !strings.Contains(node.Data.Label, "放行") {
				t.Fatalf("expected apply action node label to mention release, got %q", node.Data.Label)
			}
		}
	}
	applyNodeID := actionNodeIDs[actionIDsByCode["db_backup_whitelist_apply"]]
	if actionNodeIDs[actionIDsByCode["db_backup_whitelist_precheck"]] == "" || applyNodeID == "" {
		t.Fatalf("expected workflow action nodes bound to real action ids, got %#v workflow=%s", actionNodeIDs, service.WorkflowJSON)
	}
	for _, edge := range workflow.Edges {
		if edge.Source == "db_process" && edge.Data.Outcome == "rejected" && edge.Target == applyNodeID {
			t.Fatalf("db backup rejected edge must not pass through apply action: %s", service.WorkflowJSON)
		}
	}
}

func TestSeedServiceDefinitions_BossUsesNaturalSpecAndPreservesStructuredContract(t *testing.T) {
	db := newTestDB(t)

	if err := seedCatalogs(db); err != nil {
		t.Fatalf("seed catalogs: %v", err)
	}
	if err := seedSLATemplates(db); err != nil {
		t.Fatalf("seed SLA templates: %v", err)
	}
	if err := seedServiceDefinitions(db); err != nil {
		t.Fatalf("seed service definitions: %v", err)
	}

	var service ServiceDefinition
	if err := db.Where("code = ?", "boss-serial-change-request").First(&service).Error; err != nil {
		t.Fatalf("find boss service: %v", err)
	}

	for _, forbidden := range []string{
		"subject",
		"request_category",
		"prod_change",
		"risk_level",
		"rollback_required",
		"impact_modules",
		"gateway",
		"change_items",
		"position_department",
		"department_code",
		"position_code",
		"headquarters",
		"serial_reviewer",
		"ops_admin",
	} {
		if strings.Contains(service.CollaborationSpec, forbidden) {
			t.Fatalf("boss collaboration spec should be natural text, found machine token %q in %q", forbidden, service.CollaborationSpec)
		}
	}
	if strings.Contains(service.CollaborationSpec, "\n\n") {
		t.Fatalf("boss collaboration spec should use single line breaks, got %q", service.CollaborationSpec)
	}

	var schema struct {
		Fields []struct {
			Key     string `json:"key"`
			Type    string `json:"type"`
			Options []struct {
				Value string `json:"value"`
			} `json:"options"`
			Props struct {
				Columns []struct {
					Key     string `json:"key"`
					Type    string `json:"type"`
					Options []struct {
						Value string `json:"value"`
					} `json:"options"`
				} `json:"columns"`
			} `json:"props"`
		} `json:"fields"`
	}
	if err := json.Unmarshal([]byte(service.IntakeFormSchema), &schema); err != nil {
		t.Fatalf("unmarshal boss intake form schema: %v", err)
	}

	wantFields := []struct {
		key string
		typ string
	}{
		{"subject", "text"},
		{"request_category", "select"},
		{"risk_level", "radio"},
		{"expected_finish_time", "datetime"},
		{"change_window", "date_range"},
		{"impact_scope", "textarea"},
		{"rollback_required", "select"},
		{"impact_modules", "multi_select"},
		{"change_items", "table"},
	}
	if len(schema.Fields) != len(wantFields) {
		t.Fatalf("expected boss field keys %v, got %+v", wantFields, schema.Fields)
	}
	fieldByKey := map[string]int{}
	for i, field := range schema.Fields {
		if field.Key != wantFields[i].key || field.Type != wantFields[i].typ {
			t.Fatalf("expected field %d to be %s/%s, got %s/%s", i, wantFields[i].key, wantFields[i].typ, field.Key, field.Type)
		}
		fieldByKey[field.Key] = i
	}
	assertOptionValues := func(key string, want []string) {
		t.Helper()
		field := schema.Fields[fieldByKey[key]]
		got := make([]string, 0, len(field.Options))
		for _, opt := range field.Options {
			got = append(got, opt.Value)
		}
		if len(got) != len(want) {
			t.Fatalf("expected %s options %v, got %v", key, want, got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("expected %s options %v, got %v", key, want, got)
			}
		}
	}
	assertOptionValues("request_category", []string{"prod_change", "access_grant", "emergency_support"})
	assertOptionValues("risk_level", []string{"low", "medium", "high"})
	assertOptionValues("rollback_required", []string{"required", "not_required"})
	assertOptionValues("impact_modules", []string{"gateway", "payment", "monitoring", "order"})

	changeItems := schema.Fields[fieldByKey["change_items"]]
	wantColumns := []string{"system", "resource", "permission_level", "effective_range", "reason"}
	if len(changeItems.Props.Columns) != len(wantColumns) {
		t.Fatalf("expected change_items columns %v, got %+v", wantColumns, changeItems.Props.Columns)
	}
	for i, column := range changeItems.Props.Columns {
		if column.Key != wantColumns[i] {
			t.Fatalf("expected change_items columns %v, got %+v", wantColumns, changeItems.Props.Columns)
		}
		if column.Key == "permission_level" {
			got := make([]string, 0, len(column.Options))
			for _, opt := range column.Options {
				got = append(got, opt.Value)
			}
			want := []string{"read", "read_write"}
			if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
				t.Fatalf("expected permission_level options %v, got %v", want, got)
			}
		}
	}
}

func TestSeedServiceDefinitions_DBBackupMigratesLegacyActionCodes(t *testing.T) {
	db := newTestDB(t)

	if err := seedCatalogs(db); err != nil {
		t.Fatalf("seed catalogs: %v", err)
	}
	if err := seedSLATemplates(db); err != nil {
		t.Fatalf("seed SLA templates: %v", err)
	}

	var catalog ServiceCatalog
	if err := db.Where("code = ?", "application-platform:database").First(&catalog).Error; err != nil {
		t.Fatalf("find catalog: %v", err)
	}
	service := ServiceDefinition{
		Name:              "生产数据库备份白名单临时放行申请",
		Code:              "db-backup-whitelist-action-flow",
		CatalogID:         catalog.ID,
		EngineType:        "smart",
		CollaborationSpec: "旧协作规范",
		IsActive:          true,
	}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create legacy service: %v", err)
	}
	legacyConfig := JSONField(`{"url":"/custom-precheck","method":"POST"}`)
	if err := db.Create(&ServiceAction{
		Name:       "旧预检",
		Code:       "backup_whitelist_precheck",
		ActionType: "http",
		ConfigJSON: legacyConfig,
		ServiceID:  service.ID,
		IsActive:   true,
	}).Error; err != nil {
		t.Fatalf("create legacy action: %v", err)
	}

	if err := seedServiceDefinitions(db); err != nil {
		t.Fatalf("seed service definitions: %v", err)
	}

	var migrated ServiceAction
	if err := db.Where("service_id = ? AND code = ?", service.ID, "db_backup_whitelist_precheck").First(&migrated).Error; err != nil {
		t.Fatalf("expected legacy precheck action to migrate to canonical code: %v", err)
	}
	if string(migrated.ConfigJSON) != string(legacyConfig) {
		t.Fatalf("expected migration to preserve action config, got %s", migrated.ConfigJSON)
	}
	var legacyCount int64
	if err := db.Model(&ServiceAction{}).Where("service_id = ? AND code = ?", service.ID, "backup_whitelist_precheck").Count(&legacyCount).Error; err != nil {
		t.Fatalf("count legacy action: %v", err)
	}
	if legacyCount != 0 {
		t.Fatalf("expected legacy action code to be migrated, still found %d", legacyCount)
	}
}

func TestSeedServiceDefinitions_VPNUsesNaturalSpecAndPreservesStructuredContract(t *testing.T) {
	db := newTestDB(t)

	if err := seedCatalogs(db); err != nil {
		t.Fatalf("seed catalogs: %v", err)
	}
	if err := seedSLATemplates(db); err != nil {
		t.Fatalf("seed SLA templates: %v", err)
	}
	if err := seedServiceDefinitions(db); err != nil {
		t.Fatalf("seed service definitions: %v", err)
	}

	var service ServiceDefinition
	if err := db.Where("code = ?", "vpn-access-request").First(&service).Error; err != nil {
		t.Fatalf("find vpn service: %v", err)
	}

	for _, forbidden := range []string{
		"vpn_account",
		"device_usage",
		"request_kind",
		"form.request_kind",
		"position_department",
		"department_code",
		"position_code",
		"network_admin",
		"security_admin",
		"online_support",
		"troubleshooting",
		"production_emergency",
		"network_access_issue",
		"external_collaboration",
		"long_term_remote_work",
		"cross_border_access",
		"security_compliance",
	} {
		if strings.Contains(service.CollaborationSpec, forbidden) {
			t.Fatalf("vpn collaboration spec should be natural text, found machine token %q in %q", forbidden, service.CollaborationSpec)
		}
	}

	var schema struct {
		Fields []struct {
			Key     string `json:"key"`
			Type    string `json:"type"`
			Options []struct {
				Value string `json:"value"`
			} `json:"options"`
		} `json:"fields"`
	}
	if err := json.Unmarshal([]byte(service.IntakeFormSchema), &schema); err != nil {
		t.Fatalf("unmarshal vpn intake form schema: %v", err)
	}
	fieldTypes := map[string]string{}
	optionValues := map[string]bool{}
	for _, field := range schema.Fields {
		fieldTypes[field.Key] = field.Type
		for _, option := range field.Options {
			optionValues[option.Value] = true
		}
	}
	expectedFields := map[string]string{
		"vpn_account":  "text",
		"device_usage": "textarea",
		"request_kind": "select",
	}
	for key, typ := range expectedFields {
		if fieldTypes[key] != typ {
			t.Fatalf("expected vpn field %s type %s, got fields=%v", key, typ, fieldTypes)
		}
	}
	for _, value := range []string{"online_support", "troubleshooting", "production_emergency", "network_access_issue", "external_collaboration", "long_term_remote_work", "cross_border_access", "security_compliance"} {
		if !optionValues[value] {
			t.Fatalf("expected request_kind option %q, got %#v", value, optionValues)
		}
	}

	workflow := string(service.WorkflowJSON)
	for _, required := range []string{"form.request_kind", "it", "network_admin", "security_admin"} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("expected vpn workflow json to preserve %q, got %s", required, workflow)
		}
	}
}

func TestSeedCatalogs_IsIdempotentByCode(t *testing.T) {
	db := newTestDB(t)

	if err := seedCatalogs(db); err != nil {
		t.Fatalf("seed catalogs first run: %v", err)
	}
	if err := seedCatalogs(db); err != nil {
		t.Fatalf("seed catalogs second run: %v", err)
	}

	var count int64
	if err := db.Model(&ServiceCatalog{}).Count(&count).Error; err != nil {
		t.Fatalf("count catalogs: %v", err)
	}
	if count != 24 {
		t.Fatalf("expected 24 catalogs after rerun, got %d", count)
	}
}

func TestSeedCatalogs_RecreatesSoftDeletedCatalog(t *testing.T) {
	db := newTestDB(t)

	if err := seedCatalogs(db); err != nil {
		t.Fatalf("seed catalogs: %v", err)
	}

	var catalog ServiceCatalog
	if err := db.Where("code = ?", "account-access:provisioning").First(&catalog).Error; err != nil {
		t.Fatalf("find seeded catalog: %v", err)
	}
	originalID := catalog.ID
	if err := db.Delete(&catalog).Error; err != nil {
		t.Fatalf("soft delete catalog: %v", err)
	}

	if err := seedCatalogs(db); err != nil {
		t.Fatalf("seed catalogs rerun: %v", err)
	}

	var restored ServiceCatalog
	if err := db.Where("code = ?", "account-access:provisioning").First(&restored).Error; err != nil {
		t.Fatalf("find restored catalog: %v", err)
	}
	if restored.ID != originalID {
		t.Fatalf("expected soft-deleted catalog to be restored in place, got original=%d restored=%d", originalID, restored.ID)
	}

	var visibleCount int64
	if err := db.Model(&ServiceCatalog{}).Where("code = ?", "account-access:provisioning").Count(&visibleCount).Error; err != nil {
		t.Fatalf("count restored catalog: %v", err)
	}
	if visibleCount != 1 {
		t.Fatalf("expected restored catalog to be visible once, got %d", visibleCount)
	}
}

func TestSeedMenus_RestoresSoftDeletedApprovalPendingMenu(t *testing.T) {
	db := newTestDB(t)

	if err := seedMenus(db); err != nil {
		t.Fatalf("seed menus: %v", err)
	}

	var menu model.Menu
	if err := db.Where("permission = ?", "itsm:ticket:approval:pending").First(&menu).Error; err != nil {
		t.Fatalf("find approval pending menu: %v", err)
	}
	originalID := menu.ID
	if err := db.Delete(&menu).Error; err != nil {
		t.Fatalf("soft delete approval pending menu: %v", err)
	}

	if err := seedMenus(db); err != nil {
		t.Fatalf("seed menus rerun: %v", err)
	}

	var restored model.Menu
	if err := db.Where("permission = ?", "itsm:ticket:approval:pending").First(&restored).Error; err != nil {
		t.Fatalf("find restored approval pending menu: %v", err)
	}
	if restored.ID != originalID {
		t.Fatalf("expected approval pending menu to be restored in place, got original=%d restored=%d", originalID, restored.ID)
	}
	if restored.Name != "我的待办" {
		t.Fatalf("expected restored menu name 我的待办, got %s", restored.Name)
	}
	if restored.Path != "/itsm/tickets/approvals/pending" {
		t.Fatalf("expected restored menu path /itsm/tickets/approvals/pending, got %s", restored.Path)
	}
	if restored.Sort != 2 {
		t.Fatalf("expected restored menu sort 2, got %d", restored.Sort)
	}

	var visibleCount int64
	if err := db.Model(&model.Menu{}).Where("permission = ?", "itsm:ticket:approval:pending").Count(&visibleCount).Error; err != nil {
		t.Fatalf("count visible approval pending menu: %v", err)
	}
	if visibleCount != 1 {
		t.Fatalf("expected restored approval pending menu to be visible once, got %d", visibleCount)
	}

	var totalCount int64
	if err := db.Unscoped().Model(&model.Menu{}).Where("permission = ?", "itsm:ticket:approval:pending").Count(&totalCount).Error; err != nil {
		t.Fatalf("count all approval pending menu rows: %v", err)
	}
	if totalCount != 1 {
		t.Fatalf("expected one approval pending menu row including soft-deleted records, got %d", totalCount)
	}
}
