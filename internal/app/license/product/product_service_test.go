package product

import (
	"errors"
	"metis/internal/app/license/domain"
	"metis/internal/app/license/testutil"
	"testing"
)

func TestProductService_CreateProduct(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := &ProductService{
		productRepo:      &ProductRepo{DB: db},
		planRepo:         &PlanRepo{DB: db},
		keyRepo:          &ProductKeyRepo{DB: db},
		db:               db,
		jwtSecret:        []byte("test-jwt-secret"),
		licenseKeySecret: []byte("test-license-secret"),
	}

	product, err := svc.CreateProduct("Metis Enterprise", "metis-ent", "Enterprise edition")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if product.Name != "Metis Enterprise" {
		t.Errorf("Name = %q, want %q", product.Name, "Metis Enterprise")
	}
	if product.Code != "metis-ent" {
		t.Errorf("Code = %q, want %q", product.Code, "metis-ent")
	}
	if product.Status != domain.StatusUnpublished {
		t.Errorf("Status = %q, want %q", product.Status, domain.StatusUnpublished)
	}

	// Assert that exactly one domain.ProductKey was created and marked current
	keys, err := svc.keyRepo.FindByProductIDAndVersion(product.ID, 1)
	if err != nil {
		t.Fatalf("failed to find product key: %v", err)
	}
	if !keys.IsCurrent {
		t.Error("expected initial key to be current")
	}
	if keys.PublicKey == "" {
		t.Error("expected public key to be non-empty")
	}
}

func TestProductService_UpdateStatus(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := &ProductService{
		productRepo:      &ProductRepo{DB: db},
		planRepo:         &PlanRepo{DB: db},
		keyRepo:          &ProductKeyRepo{DB: db},
		db:               db,
		jwtSecret:        []byte("test-jwt-secret"),
		licenseKeySecret: []byte("test-license-secret"),
	}

	product, err := svc.CreateProduct("domain.Product", "prod-1", "")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tests := []struct {
		name      string
		from      string
		to        string
		wantErr   bool
		wantErrIs error
	}{
		{"unpublished -> published", domain.StatusUnpublished, domain.StatusPublished, false, nil},
		{"published -> unpublished", domain.StatusPublished, domain.StatusUnpublished, false, nil},
		{"published -> archived", domain.StatusPublished, domain.StatusArchived, false, nil},
		{"archived -> unpublished", domain.StatusArchived, domain.StatusUnpublished, false, nil},
		{"unpublished -> archived", domain.StatusUnpublished, domain.StatusArchived, false, nil},
		{"archived -> published", domain.StatusArchived, domain.StatusPublished, true, ErrInvalidStatusTransition},
		{"published -> published", domain.StatusPublished, domain.StatusPublished, true, ErrInvalidStatusTransition},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset product status for each case
			db.Model(&domain.Product{}).Where("id = ?", product.ID).Update("status", tt.from)

			err := svc.UpdateStatus(product.ID, tt.to)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
					t.Errorf("expected %v, got %v", tt.wantErrIs, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var updated domain.Product
			db.First(&updated, product.ID)
			if updated.Status != tt.to {
				t.Errorf("status = %q, want %q", updated.Status, tt.to)
			}
		})
	}
}

func TestProductService_RotateKey(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := &ProductService{
		productRepo:      &ProductRepo{DB: db},
		planRepo:         &PlanRepo{DB: db},
		keyRepo:          &ProductKeyRepo{DB: db},
		db:               db,
		jwtSecret:        []byte("test-jwt-secret"),
		licenseKeySecret: []byte("test-license-secret"),
	}

	product, err := svc.CreateProduct("domain.Product", "prod-rotate", "")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	oldKey, err := svc.keyRepo.FindCurrentByProductID(product.ID)
	if err != nil {
		t.Fatalf("failed to find initial key: %v", err)
	}

	newKey, err := svc.RotateKey(product.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if newKey.Version != oldKey.Version+1 {
		t.Errorf("newKey.Version = %d, want %d", newKey.Version, oldKey.Version+1)
	}
	if !newKey.IsCurrent {
		t.Error("expected new key to be current")
	}

	// Old key should no longer be current
	refreshedOld, err := svc.keyRepo.FindByProductIDAndVersion(product.ID, oldKey.Version)
	if err != nil {
		t.Fatalf("failed to find old key: %v", err)
	}
	if refreshedOld.IsCurrent {
		t.Error("expected old key to be revoked (not current)")
	}
	if refreshedOld.RevokedAt == nil {
		t.Error("expected old key to have revoked_at set")
	}
}

func TestProductService_CreateProduct_DuplicateCode(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := &ProductService{
		productRepo:      &ProductRepo{DB: db},
		planRepo:         &PlanRepo{DB: db},
		keyRepo:          &ProductKeyRepo{DB: db},
		db:               db,
		jwtSecret:        []byte("test-jwt-secret"),
		licenseKeySecret: []byte("test-license-secret"),
	}

	_, err := svc.CreateProduct("domain.Product A", "same-code", "")
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	_, err = svc.CreateProduct("domain.Product B", "same-code", "")
	if err == nil {
		t.Fatal("expected error for duplicate code, got nil")
	}
	if err != ErrProductCodeExists {
		t.Errorf("expected ErrProductCodeExists, got %v", err)
	}
}
