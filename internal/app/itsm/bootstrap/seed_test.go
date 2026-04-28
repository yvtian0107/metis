package bootstrap

import (
	"encoding/json"
	. "metis/internal/app/itsm/domain"
	itsmtools "metis/internal/app/itsm/tools"
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

func TestSeedServiceDefinitions_ServerAccessHasIntakeFormSchema(t *testing.T) {
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
