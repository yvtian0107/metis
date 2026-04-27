package certificate

import (
	"errors"
	licensecrypto "metis/internal/app/license/crypto"
	"metis/internal/app/license/domain"
	licenseepkg "metis/internal/app/license/licensee"
	productpkg "metis/internal/app/license/product"
	"metis/internal/app/license/registration"
	"metis/internal/app/license/testutil"
	"testing"
	"time"

	"github.com/samber/do/v2"

	"metis/internal/database"
)

func newLicenseService(db *database.DB) *LicenseService {
	return &LicenseService{
		licenseRepo:      &LicenseRepo{db: db},
		productRepo:      &productpkg.ProductRepo{DB: db},
		licenseeRepo:     &licenseepkg.LicenseeRepo{DB: db},
		keyRepo:          &productpkg.ProductKeyRepo{DB: db},
		regRepo:          &registration.LicenseRegistrationRepo{DB: db},
		db:               db,
		jwtSecret:        []byte("test-jwt-secret"),
		licenseKeySecret: []byte("test-license-secret"),
	}
}

func newProductService(db *database.DB) *productpkg.ProductService {
	injector := do.New()
	do.ProvideValue(injector, db)
	do.ProvideValue[[]byte](injector, []byte("test-jwt-secret"))
	do.ProvideNamedValue(injector, "licenseKeySecret", []byte("test-license-secret"))
	do.Provide(injector, productpkg.NewProductRepo)
	do.Provide(injector, productpkg.NewPlanRepo)
	do.Provide(injector, productpkg.NewProductKeyRepo)
	do.Provide(injector, productpkg.NewProductService)
	return do.MustInvoke[*productpkg.ProductService](injector)
}

func newLicenseeService(db *database.DB) *licenseepkg.LicenseeService {
	injector := do.New()
	do.ProvideValue(injector, db)
	do.Provide(injector, licenseepkg.NewLicenseeRepo)
	do.Provide(injector, licenseepkg.NewLicenseeService)
	return do.MustInvoke[*licenseepkg.LicenseeService](injector)
}

func TestLicenseService_IssueLicense_HappyPath(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productSvc := newProductService(db)
	licenseeSvc := newLicenseeService(db)
	licenseSvc := newLicenseService(db)

	product, err := productSvc.CreateProduct("domain.Product", "prod-issue", "")
	if err != nil {
		t.Fatalf("setup product failed: %v", err)
	}
	if err := productSvc.UpdateStatus(product.ID, domain.StatusPublished); err != nil {
		t.Fatalf("publish product failed: %v", err)
	}

	licensee, err := licenseeSvc.CreateLicensee(licenseepkg.CreateLicenseeParams{Name: "Acme Corp", Notes: ""})
	if err != nil {
		t.Fatalf("setup licensee failed: %v", err)
	}

	reg, err := licenseSvc.CreateLicenseRegistration(CreateLicenseRegistrationParams{
		ProductID:  &product.ID,
		LicenseeID: &licensee.ID,
		Code:       "RG-ACME-001",
	})
	if err != nil {
		t.Fatalf("setup registration failed: %v", err)
	}

	planName := "Enterprise"
	license, err := licenseSvc.IssueLicense(IssueLicenseParams{
		ProductID:        product.ID,
		LicenseeID:       licensee.ID,
		PlanName:         planName,
		RegistrationCode: reg.Code,
		ValidFrom:        timeNow().Add(-time.Hour),
		IssuedBy:         1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if license.PlanName != planName {
		t.Errorf("PlanName = %q, want %q", license.PlanName, planName)
	}
	if license.RegistrationCode != reg.Code {
		t.Errorf("RegistrationCode = %q, want %q", license.RegistrationCode, reg.Code)
	}
	if license.LifecycleStatus != domain.LicenseLifecycleActive {
		t.Errorf("LifecycleStatus = %q, want %q", license.LifecycleStatus, domain.LicenseLifecycleActive)
	}
	if license.ActivationCode == "" {
		t.Error("expected non-empty ActivationCode")
	}
	if license.Signature == "" {
		t.Error("expected non-empty Signature")
	}

	// Registration should now be bound
	var boundReg domain.LicenseRegistration
	db.First(&boundReg, reg.ID)
	if boundReg.BoundLicenseID == nil || *boundReg.BoundLicenseID != license.ID {
		t.Error("expected registration to be bound to issued license")
	}
}

func TestLicenseService_IssueLicense_Guards(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productSvc := newProductService(db)
	licenseeSvc := newLicenseeService(db)
	licenseSvc := newLicenseService(db)

	product, err := productSvc.CreateProduct("domain.Product", "prod-guard", "")
	if err != nil {
		t.Fatalf("setup product failed: %v", err)
	}

	licensee, err := licenseeSvc.CreateLicensee(licenseepkg.CreateLicenseeParams{Name: "Acme Corp", Notes: ""})
	if err != nil {
		t.Fatalf("setup licensee failed: %v", err)
	}

	// Test unpublished product
	t.Run("unpublished product", func(t *testing.T) {
		_, err := licenseSvc.IssueLicense(IssueLicenseParams{
			ProductID:        product.ID,
			LicenseeID:       licensee.ID,
			PlanName:         "Basic",
			RegistrationCode: "RG-TEST-001",
			ValidFrom:        timeNow(),
			IssuedBy:         1,
		})
		if !errors.Is(err, ErrProductNotPublished) {
			t.Errorf("expected ErrProductNotPublished, got %v", err)
		}
	})

	// Publish product for remaining tests
	if err := productSvc.UpdateStatus(product.ID, domain.StatusPublished); err != nil {
		t.Fatalf("publish product failed: %v", err)
	}

	// Test inactive licensee
	t.Run("inactive licensee", func(t *testing.T) {
		if err := licenseeSvc.UpdateLicenseeStatus(licensee.ID, domain.LicenseeStatusArchived); err != nil {
			t.Fatalf("archive licensee failed: %v", err)
		}
		_, err := licenseSvc.IssueLicense(IssueLicenseParams{
			ProductID:        product.ID,
			LicenseeID:       licensee.ID,
			PlanName:         "Basic",
			RegistrationCode: "RG-TEST-002",
			ValidFrom:        timeNow(),
			IssuedBy:         1,
		})
		if !errors.Is(err, ErrLicenseeNotActive) {
			t.Errorf("expected ErrLicenseeNotActive, got %v", err)
		}
		// Restore for other tests
		if err := licenseeSvc.UpdateLicenseeStatus(licensee.ID, domain.LicenseeStatusActive); err != nil {
			t.Fatalf("reactivate licensee failed: %v", err)
		}
	})

	// Test already-bound registration code
	t.Run("already bound registration code", func(t *testing.T) {
		reg, err := licenseSvc.CreateLicenseRegistration(CreateLicenseRegistrationParams{
			ProductID:  &product.ID,
			LicenseeID: &licensee.ID,
			Code:       "RG-BOUND-001",
		})
		if err != nil {
			t.Fatalf("setup registration failed: %v", err)
		}
		// Issue first license to bind the code
		_, err = licenseSvc.IssueLicense(IssueLicenseParams{
			ProductID:        product.ID,
			LicenseeID:       licensee.ID,
			PlanName:         "Basic",
			RegistrationCode: reg.Code,
			ValidFrom:        timeNow(),
			IssuedBy:         1,
		})
		if err != nil {
			t.Fatalf("first issue failed: %v", err)
		}
		// Second issue with same code should fail
		_, err = licenseSvc.IssueLicense(IssueLicenseParams{
			ProductID:        product.ID,
			LicenseeID:       licensee.ID,
			PlanName:         "Pro",
			RegistrationCode: reg.Code,
			ValidFrom:        timeNow(),
			IssuedBy:         1,
		})
		if !errors.Is(err, ErrRegistrationAlreadyBound) {
			t.Errorf("expected ErrRegistrationAlreadyBound, got %v", err)
		}
	})

	// Test expired registration code
	t.Run("expired registration code", func(t *testing.T) {
		past := timeNow().Add(-24 * time.Hour)
		reg, err := licenseSvc.CreateLicenseRegistration(CreateLicenseRegistrationParams{
			ProductID:  &product.ID,
			LicenseeID: &licensee.ID,
			Code:       "RG-EXPIRED-001",
			ExpiresAt:  &past,
		})
		if err != nil {
			t.Fatalf("setup registration failed: %v", err)
		}
		_, err = licenseSvc.IssueLicense(IssueLicenseParams{
			ProductID:        product.ID,
			LicenseeID:       licensee.ID,
			PlanName:         "Basic",
			RegistrationCode: reg.Code,
			ValidFrom:        timeNow(),
			IssuedBy:         1,
		})
		if !errors.Is(err, ErrRegistrationExpired) {
			t.Errorf("expected ErrRegistrationExpired, got %v", err)
		}
	})

	// Test missing key by deleting the auto-generated key
	t.Run("missing product key", func(t *testing.T) {
		key, _ := licenseSvc.keyRepo.FindCurrentByProductID(product.ID)
		db.Delete(&domain.ProductKey{}, key.ID)

		_, err := licenseSvc.IssueLicense(IssueLicenseParams{
			ProductID:        product.ID,
			LicenseeID:       licensee.ID,
			PlanName:         "Basic",
			RegistrationCode: "RG-MISSING-001",
			ValidFrom:        timeNow(),
			IssuedBy:         1,
		})
		if !errors.Is(err, ErrProductKeyNotFound) {
			t.Errorf("expected ErrProductKeyNotFound, got %v", err)
		}
	})
}

func TestLicenseService_UpgradeLicense(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productSvc := newProductService(db)
	licenseeSvc := newLicenseeService(db)
	licenseSvc := newLicenseService(db)

	product, err := productSvc.CreateProduct("domain.Product", "prod-upgrade", "")
	if err != nil {
		t.Fatalf("setup product failed: %v", err)
	}
	if err := productSvc.UpdateStatus(product.ID, domain.StatusPublished); err != nil {
		t.Fatalf("publish product failed: %v", err)
	}

	licensee, err := licenseeSvc.CreateLicensee(licenseepkg.CreateLicenseeParams{Name: "Acme Corp", Notes: ""})
	if err != nil {
		t.Fatalf("setup licensee failed: %v", err)
	}

	reg, err := licenseSvc.CreateLicenseRegistration(CreateLicenseRegistrationParams{
		ProductID:  &product.ID,
		LicenseeID: &licensee.ID,
		Code:       "RG-UPGRADE-001",
	})
	if err != nil {
		t.Fatalf("setup registration failed: %v", err)
	}

	original, err := licenseSvc.IssueLicense(IssueLicenseParams{
		ProductID:        product.ID,
		LicenseeID:       licensee.ID,
		PlanName:         "Basic",
		RegistrationCode: reg.Code,
		ValidFrom:        timeNow().Add(-time.Hour),
		IssuedBy:         1,
	})
	if err != nil {
		t.Fatalf("setup issue failed: %v", err)
	}

	newLicense, err := licenseSvc.UpgradeLicense(original.ID, IssueLicenseParams{
		ProductID:        product.ID,
		LicenseeID:       licensee.ID,
		PlanName:         "Pro",
		RegistrationCode: reg.Code,
		ValidFrom:        timeNow(),
		IssuedBy:         1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if newLicense.PlanName != "Pro" {
		t.Errorf("PlanName = %q, want %q", newLicense.PlanName, "Pro")
	}
	if newLicense.OriginalLicenseID == nil || *newLicense.OriginalLicenseID != original.ID {
		t.Error("expected new license to reference original license")
	}

	// Original should be revoked
	var orig domain.License
	db.First(&orig, original.ID)
	if orig.LifecycleStatus != domain.LicenseLifecycleRevoked {
		t.Errorf("original lifecycle = %q, want %q", orig.LifecycleStatus, domain.LicenseLifecycleRevoked)
	}
	if orig.Status != domain.LicenseStatusRevoked {
		t.Errorf("original status = %q, want %q", orig.Status, domain.LicenseStatusRevoked)
	}

	// Registration should be bound to new license
	var boundReg domain.LicenseRegistration
	db.First(&boundReg, reg.ID)
	if boundReg.BoundLicenseID == nil || *boundReg.BoundLicenseID != newLicense.ID {
		t.Error("expected registration to be bound to upgraded license")
	}
}

func TestLicenseService_LifecycleStateMachine(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productSvc := newProductService(db)
	licenseeSvc := newLicenseeService(db)
	licenseSvc := newLicenseService(db)

	product, err := productSvc.CreateProduct("domain.Product", "prod-lifecycle", "")
	if err != nil {
		t.Fatalf("setup product failed: %v", err)
	}
	if err := productSvc.UpdateStatus(product.ID, domain.StatusPublished); err != nil {
		t.Fatalf("publish product failed: %v", err)
	}

	licensee, err := licenseeSvc.CreateLicensee(licenseepkg.CreateLicenseeParams{Name: "Acme Corp", Notes: ""})
	if err != nil {
		t.Fatalf("setup licensee failed: %v", err)
	}

	issue := func(code string) *domain.License {
		reg, _ := licenseSvc.CreateLicenseRegistration(CreateLicenseRegistrationParams{
			ProductID:  &product.ID,
			LicenseeID: &licensee.ID,
			Code:       code,
		})
		l, err := licenseSvc.IssueLicense(IssueLicenseParams{
			ProductID:        product.ID,
			LicenseeID:       licensee.ID,
			PlanName:         "Basic",
			RegistrationCode: reg.Code,
			ValidFrom:        timeNow().Add(-time.Hour),
			IssuedBy:         1,
		})
		if err != nil {
			t.Fatalf("issue failed: %v", err)
		}
		return l
	}

	t.Run("revoke sets timestamps", func(t *testing.T) {
		l := issue("RG-REVOKE-001")
		if err := licenseSvc.RevokeLicense(l.ID, 2); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var updated domain.License
		db.First(&updated, l.ID)
		if updated.Status != domain.LicenseStatusRevoked {
			t.Errorf("status = %q, want %q", updated.Status, domain.LicenseStatusRevoked)
		}
		if updated.LifecycleStatus != domain.LicenseLifecycleRevoked {
			t.Errorf("lifecycle = %q, want %q", updated.LifecycleStatus, domain.LicenseLifecycleRevoked)
		}
		if updated.RevokedAt == nil || updated.RevokedBy == nil || *updated.RevokedBy != 2 {
			t.Error("expected revoked_at and revoked_by to be set")
		}
		// Double revoke should fail
		if !errors.Is(licenseSvc.RevokeLicense(l.ID, 2), ErrLicenseAlreadyRevoked) {
			t.Error("expected ErrLicenseAlreadyRevoked on second revoke")
		}
	})

	t.Run("suspend and reactivate", func(t *testing.T) {
		l := issue("RG-SUSPEND-001")
		if err := licenseSvc.SuspendLicense(l.ID, 3); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var suspended domain.License
		db.First(&suspended, l.ID)
		if suspended.LifecycleStatus != domain.LicenseLifecycleSuspended {
			t.Errorf("lifecycle = %q, want %q", suspended.LifecycleStatus, domain.LicenseLifecycleSuspended)
		}
		if suspended.SuspendedAt == nil || suspended.SuspendedBy == nil || *suspended.SuspendedBy != 3 {
			t.Error("expected suspended_at and suspended_by to be set")
		}

		// Double suspend should fail
		if !errors.Is(licenseSvc.SuspendLicense(l.ID, 3), ErrLicenseAlreadySuspended) {
			t.Error("expected ErrLicenseAlreadySuspended on second suspend")
		}

		// Reactivate
		if err := licenseSvc.ReactivateLicense(l.ID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var reactivated domain.License
		db.First(&reactivated, l.ID)
		if reactivated.LifecycleStatus != domain.LicenseLifecycleActive {
			t.Errorf("lifecycle = %q, want %q", reactivated.LifecycleStatus, domain.LicenseLifecycleActive)
		}
		if reactivated.SuspendedAt != nil {
			t.Error("expected suspended_at to be cleared")
		}

		// Reactivate non-suspended should fail
		if !errors.Is(licenseSvc.ReactivateLicense(l.ID), ErrLicenseNotSuspended) {
			t.Error("expected ErrLicenseNotSuspended")
		}
	})

	t.Run("renew extends expiration", func(t *testing.T) {
		l := issue("RG-RENEW-001")
		originalActivationCode := l.ActivationCode
		originalSignature := l.Signature
		originalKeyVersion := l.KeyVersion

		if _, err := productSvc.RotateKey(product.ID); err != nil {
			t.Fatalf("rotate key failed: %v", err)
		}

		newExpiry := timeNow().Add(30 * 24 * time.Hour)
		if err := licenseSvc.RenewLicense(l.ID, &newExpiry, 4); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var renewed domain.License
		db.First(&renewed, l.ID)
		if renewed.LifecycleStatus != domain.LicenseLifecycleActive {
			t.Errorf("lifecycle = %q, want %q", renewed.LifecycleStatus, domain.LicenseLifecycleActive)
		}
		if renewed.ValidUntil == nil || !renewed.ValidUntil.Equal(newExpiry) {
			t.Error("expected valid_until to be updated")
		}
		if renewed.ID != l.ID {
			t.Errorf("ID = %d, want original ID %d", renewed.ID, l.ID)
		}
		if renewed.ActivationCode == originalActivationCode {
			t.Error("expected activation code to change after renew")
		}
		if renewed.Signature == originalSignature {
			t.Error("expected signature to change after renew")
		}
		if renewed.KeyVersion != originalKeyVersion+1 {
			t.Errorf("KeyVersion = %d, want %d", renewed.KeyVersion, originalKeyVersion+1)
		}

		claims, err := licensecrypto.DecodeActivationCode(renewed.ActivationCode)
		if err != nil {
			t.Fatalf("decode renewed activation code failed: %v", err)
		}
		exp, ok := claims["exp"].(float64)
		if !ok {
			t.Fatalf("exp claim type = %T, want number", claims["exp"])
		}
		if int64(exp) != newExpiry.Unix() {
			t.Errorf("exp = %d, want %d", int64(exp), newExpiry.Unix())
		}
		sig, ok := claims["sig"].(string)
		if !ok || sig == "" {
			t.Fatalf("sig claim missing")
		}
		delete(claims, "sig")
		key, err := licenseSvc.keyRepo.FindByProductIDAndVersion(product.ID, renewed.KeyVersion)
		if err != nil {
			t.Fatalf("find renewed key failed: %v", err)
		}
		valid, err := licensecrypto.VerifyLicenseSignature(claims, sig, key.PublicKey)
		if err != nil {
			t.Fatalf("verify renewed signature failed: %v", err)
		}
		if !valid {
			t.Fatal("expected renewed signature to be valid")
		}
	})

	t.Run("renew to permanent clears expiration in certificate", func(t *testing.T) {
		reg, _ := licenseSvc.CreateLicenseRegistration(CreateLicenseRegistrationParams{
			ProductID:  &product.ID,
			LicenseeID: &licensee.ID,
			Code:       "RG-RENEW-PERM-001",
		})
		oldExpiry := timeNow().Add(24 * time.Hour)
		l, err := licenseSvc.IssueLicense(IssueLicenseParams{
			ProductID:        product.ID,
			LicenseeID:       licensee.ID,
			PlanName:         "Basic",
			RegistrationCode: reg.Code,
			ValidFrom:        timeNow().Add(-time.Hour),
			ValidUntil:       &oldExpiry,
			IssuedBy:         1,
		})
		if err != nil {
			t.Fatalf("issue failed: %v", err)
		}
		if err := licenseSvc.RenewLicense(l.ID, nil, 4); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var renewed domain.License
		db.First(&renewed, l.ID)
		if renewed.ValidUntil != nil {
			t.Error("expected valid_until to be cleared")
		}
		claims, err := licensecrypto.DecodeActivationCode(renewed.ActivationCode)
		if err != nil {
			t.Fatalf("decode renewed activation code failed: %v", err)
		}
		if claims["exp"] != nil {
			t.Errorf("exp = %#v, want nil", claims["exp"])
		}
		sig := claims["sig"].(string)
		delete(claims, "sig")
		key, err := licenseSvc.keyRepo.FindByProductIDAndVersion(product.ID, renewed.KeyVersion)
		if err != nil {
			t.Fatalf("find renewed key failed: %v", err)
		}
		valid, err := licensecrypto.VerifyLicenseSignature(claims, sig, key.PublicKey)
		if err != nil {
			t.Fatalf("verify renewed signature failed: %v", err)
		}
		if !valid {
			t.Fatal("expected permanent renewed signature to be valid")
		}
	})

	t.Run("renew to past expiration marks expired", func(t *testing.T) {
		l := issue("RG-RENEW-EXPIRED-001")
		newExpiry := timeNow().Add(-time.Minute)
		if err := licenseSvc.RenewLicense(l.ID, &newExpiry, 4); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var renewed domain.License
		db.First(&renewed, l.ID)
		if renewed.LifecycleStatus != domain.LicenseLifecycleExpired {
			t.Errorf("lifecycle = %q, want %q", renewed.LifecycleStatus, domain.LicenseLifecycleExpired)
		}
	})

	t.Run("operations on revoked license are blocked", func(t *testing.T) {
		l := issue("RG-REVOKED-OP-001")
		if err := licenseSvc.RevokeLicense(l.ID, 2); err != nil {
			t.Fatalf("setup revoke failed: %v", err)
		}
		if !errors.Is(licenseSvc.SuspendLicense(l.ID, 1), ErrLicenseAlreadyRevoked) {
			t.Error("expected ErrLicenseAlreadyRevoked on suspend")
		}
		if !errors.Is(licenseSvc.RenewLicense(l.ID, nil, 1), ErrLicenseAlreadyRevoked) {
			t.Error("expected ErrLicenseAlreadyRevoked on renew")
		}
		_, err := licenseSvc.UpgradeLicense(l.ID, IssueLicenseParams{})
		if !errors.Is(err, ErrLicenseAlreadyRevoked) {
			t.Errorf("expected ErrLicenseAlreadyRevoked on upgrade, got %v", err)
		}
	})
}

func TestLicenseService_BulkReissueLicenses(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productSvc := newProductService(db)
	licenseeSvc := newLicenseeService(db)
	licenseSvc := newLicenseService(db)

	product, err := productSvc.CreateProduct("domain.Product", "prod-reissue", "")
	if err != nil {
		t.Fatalf("setup product failed: %v", err)
	}
	if err := productSvc.UpdateStatus(product.ID, domain.StatusPublished); err != nil {
		t.Fatalf("publish product failed: %v", err)
	}

	licensee, err := licenseeSvc.CreateLicensee(licenseepkg.CreateLicenseeParams{Name: "Acme Corp", Notes: ""})
	if err != nil {
		t.Fatalf("setup licensee failed: %v", err)
	}

	// Issue a license
	reg, _ := licenseSvc.CreateLicenseRegistration(CreateLicenseRegistrationParams{
		ProductID:  &product.ID,
		LicenseeID: &licensee.ID,
		Code:       "RG-REISSUE-001",
	})
	license, err := licenseSvc.IssueLicense(IssueLicenseParams{
		ProductID:        product.ID,
		LicenseeID:       licensee.ID,
		PlanName:         "Basic",
		RegistrationCode: reg.Code,
		ValidFrom:        timeNow().Add(-time.Hour),
		IssuedBy:         1,
	})
	if err != nil {
		t.Fatalf("setup issue failed: %v", err)
	}

	originalVersion := license.KeyVersion

	// Rotate key so the license becomes reissueable
	_, err = productSvc.RotateKey(product.ID)
	if err != nil {
		t.Fatalf("rotate key failed: %v", err)
	}

	// Reissue by explicit ID
	reissued, err := licenseSvc.BulkReissueLicenses(product.ID, []uint{license.ID}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reissued != 1 {
		t.Errorf("reissued = %d, want 1", reissued)
	}

	var updated domain.License
	db.First(&updated, license.ID)
	if updated.KeyVersion != originalVersion+1 {
		t.Errorf("KeyVersion = %d, want %d", updated.KeyVersion, originalVersion+1)
	}
	if updated.ActivationCode == license.ActivationCode {
		t.Error("expected activation code to change after reissue")
	}

	// Test 100-item limit
	manyIDs := make([]uint, 101)
	for i := range manyIDs {
		manyIDs[i] = uint(i + 1)
	}
	_, err = licenseSvc.BulkReissueLicenses(product.ID, manyIDs, 1)
	if !errors.Is(err, ErrBulkReissueTooMany) {
		t.Errorf("expected ErrBulkReissueTooMany, got %v", err)
	}
}

func TestLicenseService_ExportLicFile(t *testing.T) {
	db := testutil.SetupTestDB(t)
	productSvc := newProductService(db)
	licenseeSvc := newLicenseeService(db)
	licenseSvc := newLicenseService(db)

	product, err := productSvc.CreateProduct("domain.Product", "prod-export", "")
	if err != nil {
		t.Fatalf("setup product failed: %v", err)
	}
	if err := productSvc.UpdateStatus(product.ID, domain.StatusPublished); err != nil {
		t.Fatalf("publish product failed: %v", err)
	}

	licensee, err := licenseeSvc.CreateLicensee(licenseepkg.CreateLicenseeParams{Name: "Acme Corp", Notes: ""})
	if err != nil {
		t.Fatalf("setup licensee failed: %v", err)
	}

	reg, _ := licenseSvc.CreateLicenseRegistration(CreateLicenseRegistrationParams{
		ProductID:  &product.ID,
		LicenseeID: &licensee.ID,
		Code:       "RG-EXPORT-001",
	})
	license, err := licenseSvc.IssueLicense(IssueLicenseParams{
		ProductID:        product.ID,
		LicenseeID:       licensee.ID,
		PlanName:         "Basic",
		RegistrationCode: reg.Code,
		ValidFrom:        timeNow().Add(-time.Hour),
		IssuedBy:         1,
	})
	if err != nil {
		t.Fatalf("setup issue failed: %v", err)
	}

	t.Run("happy path", func(t *testing.T) {
		content, filename, err := licenseSvc.ExportLicFile(license.ID, "v1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if content == "" {
			t.Error("expected non-empty content")
		}
		if filename == "" {
			t.Error("expected non-empty filename")
		}
	})

	t.Run("revoked license cannot be exported", func(t *testing.T) {
		if err := licenseSvc.RevokeLicense(license.ID, 1); err != nil {
			t.Fatalf("setup revoke failed: %v", err)
		}
		_, _, err := licenseSvc.ExportLicFile(license.ID, "v1")
		if !errors.Is(err, ErrRevokedLicenseNoExport) {
			t.Errorf("expected ErrRevokedLicenseNoExport, got %v", err)
		}
	})
}
