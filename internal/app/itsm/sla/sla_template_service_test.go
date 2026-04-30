package sla

import (
	"testing"

	"github.com/samber/do/v2"

	. "metis/internal/app/itsm/domain"
	"metis/internal/database"
)

func newSLATemplateServiceForTest(t *testing.T) (*SLATemplateService, *database.DB) {
	t.Helper()
	db := &database.DB{DB: newTestDB(t)}
	injector := do.New()
	do.ProvideValue(injector, db)
	do.Provide(injector, NewSLATemplateRepo)
	do.Provide(injector, NewSLATemplateService)
	return do.MustInvoke[*SLATemplateService](injector), db
}

func TestSLATemplateServiceDeleteRejectsActiveServiceReference(t *testing.T) {
	svc, db := newSLATemplateServiceForTest(t)
	sla := SLATemplate{Name: "标准 SLA", Code: "standard-sla", ResponseMinutes: 5, ResolutionMinutes: 30, IsActive: true}
	if err := db.Create(&sla).Error; err != nil {
		t.Fatalf("create sla: %v", err)
	}
	service := ServiceDefinition{Name: "VPN", Code: "vpn", CatalogID: 1, EngineType: "classic", SLAID: &sla.ID, IsActive: true}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}

	if err := svc.Delete(sla.ID); err == nil {
		t.Fatal("expected delete to fail when SLA is referenced by an active service")
	}
}

func TestSLATemplateServiceDeactivateRejectsActiveServiceReference(t *testing.T) {
	svc, db := newSLATemplateServiceForTest(t)
	sla := SLATemplate{Name: "标准 SLA", Code: "standard-sla", ResponseMinutes: 5, ResolutionMinutes: 30, IsActive: true}
	if err := db.Create(&sla).Error; err != nil {
		t.Fatalf("create sla: %v", err)
	}
	service := ServiceDefinition{Name: "VPN", Code: "vpn", CatalogID: 1, EngineType: "classic", SLAID: &sla.ID, IsActive: true}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}

	if _, err := svc.Update(sla.ID, map[string]any{"is_active": false}); err == nil {
		t.Fatal("expected deactivate to fail when SLA is referenced by an active service")
	}
}
