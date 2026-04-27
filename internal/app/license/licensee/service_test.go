package licensee

import (
	"errors"
	"metis/internal/app/license/domain"
	"metis/internal/app/license/testutil"
	"testing"

	"metis/internal/database"
)

func newLicenseeService(db *database.DB) *LicenseeService {
	return &LicenseeService{repo: &LicenseeRepo{DB: db}}
}

func TestLicenseeService_Create(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := newLicenseeService(db)

	licensee, err := svc.CreateLicensee(CreateLicenseeParams{Name: "Acme Corp", Notes: "Note"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if licensee.Name != "Acme Corp" {
		t.Errorf("Name = %q, want %q", licensee.Name, "Acme Corp")
	}
	if licensee.Status != domain.LicenseeStatusActive {
		t.Errorf("Status = %q, want %q", licensee.Status, domain.LicenseeStatusActive)
	}
	if licensee.Code == "" {
		t.Error("expected non-empty Code")
	}

	// Duplicate name
	_, err = svc.CreateLicensee(CreateLicenseeParams{Name: "Acme Corp", Notes: ""})
	if !errors.Is(err, ErrLicenseeNameExists) {
		t.Errorf("expected ErrLicenseeNameExists, got %v", err)
	}
}

func TestLicenseeService_Update(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := newLicenseeService(db)

	licensee, err := svc.CreateLicensee(CreateLicenseeParams{Name: "Acme Corp", Notes: ""})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	newName := "Acme Inc"
	updated, err := svc.UpdateLicensee(licensee.ID, UpdateLicenseeParams{Name: &newName})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != newName {
		t.Errorf("Name = %q, want %q", updated.Name, newName)
	}
}

func TestLicenseeService_UpdateStatus(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := newLicenseeService(db)

	licensee, err := svc.CreateLicensee(CreateLicenseeParams{Name: "Acme Corp", Notes: ""})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Archive
	if err := svc.UpdateLicenseeStatus(licensee.ID, domain.LicenseeStatusArchived); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var archived domain.Licensee
	db.First(&archived, licensee.ID)
	if archived.Status != domain.LicenseeStatusArchived {
		t.Errorf("Status = %q, want %q", archived.Status, domain.LicenseeStatusArchived)
	}

	// Reactivate
	if err := svc.UpdateLicenseeStatus(licensee.ID, domain.LicenseeStatusActive); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var active domain.Licensee
	db.First(&active, licensee.ID)
	if active.Status != domain.LicenseeStatusActive {
		t.Errorf("Status = %q, want %q", active.Status, domain.LicenseeStatusActive)
	}

	// Invalid transition
	if err := svc.UpdateLicenseeStatus(licensee.ID, "invalid"); !errors.Is(err, ErrLicenseeInvalidStatus) {
		t.Errorf("expected ErrLicenseeInvalidStatus, got %v", err)
	}
}
