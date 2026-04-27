package product

import (
	"encoding/json"
	"errors"
	"metis/internal/app/license/domain"
	"metis/internal/app/license/testutil"
	"testing"

	"metis/internal/database"
)

func newPlanService(db *database.DB) *PlanService {
	return &PlanService{
		planRepo:    &PlanRepo{DB: db},
		productRepo: &ProductRepo{DB: db},
	}
}

func TestPlanService_CreatePlan(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productSvc := &ProductService{
		productRepo:      &ProductRepo{DB: db},
		planRepo:         &PlanRepo{DB: db},
		keyRepo:          &ProductKeyRepo{DB: db},
		db:               db,
		jwtSecret:        []byte("test-jwt-secret"),
		licenseKeySecret: []byte("test-license-secret"),
	}
	planSvc := newPlanService(db)

	product, err := productSvc.CreateProduct("domain.Product", "prod-plan", "")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Update product with a constraint schema
	schema := json.RawMessage(`[{"key":"core","label":"Core","features":[{"key":"seats","type":"number","min":1,"max":100}]}]`)
	if err := productSvc.UpdateConstraintSchema(product.ID, schema); err != nil {
		t.Fatalf("failed to set schema: %v", err)
	}

	t.Run("valid plan without constraints", func(t *testing.T) {
		plan, err := planSvc.CreatePlan(product.ID, "Basic", nil, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if plan.Name != "Basic" {
			t.Errorf("Name = %q, want %q", plan.Name, "Basic")
		}
	})

	t.Run("valid plan with matching constraints", func(t *testing.T) {
		values := json.RawMessage(`{"core":{"seats":10}}`)
		plan, err := planSvc.CreatePlan(product.ID, "Pro", values, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if plan.Name != "Pro" {
			t.Errorf("Name = %q, want %q", plan.Name, "Pro")
		}
	})

	t.Run("duplicate name", func(t *testing.T) {
		_, err := planSvc.CreatePlan(product.ID, "Basic", nil, 0)
		if !errors.Is(err, ErrPlanNameExists) {
			t.Errorf("expected ErrPlanNameExists, got %v", err)
		}
	})

	t.Run("invalid constraint values", func(t *testing.T) {
		values := json.RawMessage(`{"core":{"seats":999}}`)
		_, err := planSvc.CreatePlan(product.ID, "Enterprise", values, 0)
		if !errors.Is(err, ErrInvalidConstraintValues) {
			t.Errorf("expected ErrInvalidConstraintValues, got %v", err)
		}
	})
}

func TestPlanService_SetDefaultPlan(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productSvc := &ProductService{
		productRepo:      &ProductRepo{DB: db},
		planRepo:         &PlanRepo{DB: db},
		keyRepo:          &ProductKeyRepo{DB: db},
		db:               db,
		jwtSecret:        []byte("test-jwt-secret"),
		licenseKeySecret: []byte("test-license-secret"),
	}
	planSvc := newPlanService(db)

	product, err := productSvc.CreateProduct("domain.Product", "prod-default", "")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	plan1, err := planSvc.CreatePlan(product.ID, "domain.Plan 1", nil, 0)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	plan2, err := planSvc.CreatePlan(product.ID, "domain.Plan 2", nil, 0)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Set plan1 as default
	if err := planSvc.SetDefaultPlan(plan1.ID, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Now set plan2 as default; plan1 should no longer be default
	if err := planSvc.SetDefaultPlan(plan2.ID, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var count int64
	db.Model(&domain.Plan{}).Where("product_id = ? AND is_default = ?", product.ID, true).Count(&count)
	if count != 1 {
		t.Errorf("expected exactly 1 default plan, got %d", count)
	}

	// Verify plan2 is the default
	var p2 domain.Plan
	db.First(&p2, plan2.ID)
	if !p2.IsDefault {
		t.Error("expected plan2 to be default")
	}

	var p1 domain.Plan
	db.First(&p1, plan1.ID)
	if p1.IsDefault {
		t.Error("expected plan1 to no longer be default")
	}
}

func TestPlanService_UpdatePlan(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productSvc := &ProductService{
		productRepo:      &ProductRepo{DB: db},
		planRepo:         &PlanRepo{DB: db},
		keyRepo:          &ProductKeyRepo{DB: db},
		db:               db,
		jwtSecret:        []byte("test-jwt-secret"),
		licenseKeySecret: []byte("test-license-secret"),
	}
	planSvc := newPlanService(db)

	product, err := productSvc.CreateProduct("domain.Product", "prod-update", "")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	schema := json.RawMessage(`[{"key":"core","features":[{"key":"seats","type":"number","min":1,"max":100}]}]`)
	if err := productSvc.UpdateConstraintSchema(product.ID, schema); err != nil {
		t.Fatalf("failed to set schema: %v", err)
	}

	plan, err := planSvc.CreatePlan(product.ID, "Starter", nil, 0)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	t.Run("rename plan", func(t *testing.T) {
		newName := "Starter Plus"
		updated, err := planSvc.UpdatePlan(plan.ID, &newName, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Name != newName {
			t.Errorf("Name = %q, want %q", updated.Name, newName)
		}
	})

	t.Run("update with invalid constraints", func(t *testing.T) {
		values := json.RawMessage(`{"core":{"seats":0}}`)
		_, err := planSvc.UpdatePlan(plan.ID, nil, values, nil)
		if !errors.Is(err, ErrInvalidConstraintValues) {
			t.Errorf("expected ErrInvalidConstraintValues, got %v", err)
		}
	})
}
