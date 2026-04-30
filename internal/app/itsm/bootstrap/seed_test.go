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
