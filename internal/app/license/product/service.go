package product

import (
	"encoding/json"
	"errors"
	"fmt"
	licensecrypto "metis/internal/app/license/crypto"
	"metis/internal/app/license/domain"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
)

var (
	ErrProductNotFound         = errors.New("error.license.product_not_found")
	ErrProductCodeExists       = errors.New("error.license.product_code_exists")
	ErrInvalidStatusTransition = errors.New("error.license.invalid_status_transition")
	ErrPlanNotFound            = errors.New("error.license.plan_not_found")
	ErrPlanNameExists          = errors.New("error.license.plan_name_exists")
	ErrInvalidConstraintSchema = errors.New("error.license.invalid_constraint_schema")
	ErrInvalidConstraintValues = errors.New("error.license.invalid_constraint_values")
)

// Valid status transitions
var statusTransitions = map[string][]string{
	domain.StatusUnpublished: {domain.StatusPublished, domain.StatusArchived},
	domain.StatusPublished:   {domain.StatusUnpublished, domain.StatusArchived},
	domain.StatusArchived:    {domain.StatusUnpublished},
}

// --- ProductService ---

type ProductService struct {
	productRepo      *ProductRepo
	planRepo         *PlanRepo
	keyRepo          *ProductKeyRepo
	db               *database.DB
	jwtSecret        []byte
	licenseKeySecret []byte
}

func NewProductService(i do.Injector) (*ProductService, error) {
	licenseKeySecret, _ := do.InvokeNamed[[]byte](i, "licenseKeySecret")
	return &ProductService{
		productRepo:      do.MustInvoke[*ProductRepo](i),
		planRepo:         do.MustInvoke[*PlanRepo](i),
		keyRepo:          do.MustInvoke[*ProductKeyRepo](i),
		db:               do.MustInvoke[*database.DB](i),
		jwtSecret:        do.MustInvoke[[]byte](i),
		licenseKeySecret: licenseKeySecret,
	}, nil
}

func (s *ProductService) CreateProduct(name, code, description string) (*domain.Product, error) {
	exists, err := s.productRepo.ExistsByCode(code)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrProductCodeExists
	}

	encKey, err := licensecrypto.GetEncryptionKeyWithFallback(s.licenseKeySecret, s.jwtSecret)
	if err != nil {
		return nil, err
	}

	pubKey, encPrivKey, err := licensecrypto.GenerateKeyPair(encKey)
	if err != nil {
		return nil, fmt.Errorf("generate key pair: %w", err)
	}

	product := &domain.Product{
		Name:        name,
		Code:        code,
		Description: description,
		Status:      domain.StatusUnpublished,
	}

	// Generate per-product license key for .lic file encryption
	licenseKey, err := licensecrypto.GenerateLicenseKey()
	if err != nil {
		return nil, err
	}
	product.LicenseKey = licenseKey

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(product).Error; err != nil {
			return err
		}
		key := &domain.ProductKey{
			ProductID:           product.ID,
			Version:             1,
			PublicKey:           pubKey,
			EncryptedPrivateKey: encPrivKey,
			IsCurrent:           true,
		}
		return tx.Create(key).Error
	})
	if err != nil {
		return nil, err
	}

	return product, nil
}

func (s *ProductService) GetProduct(id uint) (*domain.Product, error) {
	p, err := s.productRepo.FindByIDWithPlans(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}

	// Backfill licenseKey for products created before this field was added
	if p.LicenseKey == "" {
		if newKey, err := licensecrypto.GenerateLicenseKey(); err == nil {
			p.LicenseKey = newKey
			_ = s.productRepo.Update(p)
		}
	}

	return p, nil
}

func (s *ProductService) ListProducts(params ProductListParams) ([]ProductListItem, int64, error) {
	return s.productRepo.List(params)
}

type UpdateProductParams struct {
	Name        *string
	Description *string
}

func (s *ProductService) UpdateProduct(id uint, params UpdateProductParams) (*domain.Product, error) {
	p, err := s.productRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}

	if params.Name != nil {
		p.Name = *params.Name
	}
	if params.Description != nil {
		p.Description = *params.Description
	}

	if err := s.productRepo.Update(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *ProductService) UpdateConstraintSchema(id uint, schemaJSON json.RawMessage) error {
	// Validate the schema structure
	var schema domain.ConstraintSchema
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidConstraintSchema, err)
	}
	if err := validateConstraintSchema(schema); err != nil {
		return err
	}

	_, err := s.productRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProductNotFound
		}
		return err
	}

	return s.productRepo.UpdateSchema(id, schemaJSON)
}

func (s *ProductService) UpdateStatus(id uint, newStatus string) error {
	p, err := s.productRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProductNotFound
		}
		return err
	}

	allowed := statusTransitions[p.Status]
	valid := false
	for _, s := range allowed {
		if s == newStatus {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidStatusTransition, p.Status, newStatus)
	}

	return s.productRepo.UpdateStatus(id, newStatus)
}

func (s *ProductService) RotateKey(productID uint) (*domain.ProductKey, error) {
	encKey, err := licensecrypto.GetEncryptionKeyWithFallback(s.licenseKeySecret, s.jwtSecret)
	if err != nil {
		return nil, err
	}

	current, err := s.keyRepo.FindCurrentByProductID(productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}

	pubKey, encPrivKey, err := licensecrypto.GenerateKeyPair(encKey)
	if err != nil {
		return nil, fmt.Errorf("generate key pair: %w", err)
	}

	newKey := &domain.ProductKey{
		ProductID:           productID,
		Version:             current.Version + 1,
		PublicKey:           pubKey,
		EncryptedPrivateKey: encPrivKey,
		IsCurrent:           true,
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.keyRepo.RevokeByProductID(tx, productID); err != nil {
			return err
		}
		return s.keyRepo.CreateInTx(tx, newKey)
	})
	if err != nil {
		return nil, err
	}

	return newKey, nil
}

func (s *ProductService) GetPublicKey(productID uint) (*domain.ProductKey, error) {
	k, err := s.keyRepo.FindCurrentByProductID(productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}
	return k, nil
}

// --- PlanService ---

type PlanService struct {
	planRepo    *PlanRepo
	productRepo *ProductRepo
}

func NewPlanService(i do.Injector) (*PlanService, error) {
	return &PlanService{
		planRepo:    do.MustInvoke[*PlanRepo](i),
		productRepo: do.MustInvoke[*ProductRepo](i),
	}, nil
}

func (s *PlanService) CreatePlan(productID uint, name string, constraintValues json.RawMessage, sortOrder int) (*domain.Plan, error) {
	product, err := s.productRepo.FindByID(productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}

	exists, err := s.planRepo.ExistsByName(productID, name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrPlanNameExists
	}

	if len(product.ConstraintSchema) > 0 && len(constraintValues) > 0 {
		if err := validateConstraintValues(product.ConstraintSchema.RawMessage(), constraintValues); err != nil {
			return nil, err
		}
	}

	plan := &domain.Plan{
		ProductID:        productID,
		Name:             name,
		ConstraintValues: domain.JSONText(constraintValues),
		SortOrder:        sortOrder,
	}
	if err := s.planRepo.Create(plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func (s *PlanService) UpdatePlan(id uint, name *string, constraintValues json.RawMessage, sortOrder *int) (*domain.Plan, error) {
	plan, err := s.planRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPlanNotFound
		}
		return nil, err
	}

	if name != nil {
		exists, err := s.planRepo.ExistsByName(plan.ProductID, *name, id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrPlanNameExists
		}
		plan.Name = *name
	}

	if constraintValues != nil {
		product, err := s.productRepo.FindByID(plan.ProductID)
		if err != nil {
			return nil, err
		}
		if len(product.ConstraintSchema) > 0 {
			if err := validateConstraintValues(product.ConstraintSchema.RawMessage(), constraintValues); err != nil {
				return nil, err
			}
		}
		plan.ConstraintValues = domain.JSONText(constraintValues)
	}

	if sortOrder != nil {
		plan.SortOrder = *sortOrder
	}

	if err := s.planRepo.Update(plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func (s *PlanService) DeletePlan(id uint) error {
	_, err := s.planRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPlanNotFound
		}
		return err
	}
	return s.planRepo.Delete(id)
}

func (s *PlanService) SetDefaultPlan(id uint, isDefault bool) error {
	plan, err := s.planRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPlanNotFound
		}
		return err
	}

	if isDefault {
		if err := s.planRepo.ClearDefault(plan.ProductID); err != nil {
			return err
		}
	}
	return s.planRepo.SetDefault(id, isDefault)
}

// --- Validation helpers ---

func validateConstraintSchema(schema domain.ConstraintSchema) error {
	moduleKeys := make(map[string]bool)
	for _, m := range schema {
		if m.Key == "" {
			return fmt.Errorf("%w: module key is required", ErrInvalidConstraintSchema)
		}
		if moduleKeys[m.Key] {
			return fmt.Errorf("%w: duplicate module key: %s", ErrInvalidConstraintSchema, m.Key)
		}
		moduleKeys[m.Key] = true

		featureKeys := make(map[string]bool)
		for _, f := range m.Features {
			if f.Key == "" {
				return fmt.Errorf("%w: feature key is required in module %s", ErrInvalidConstraintSchema, m.Key)
			}
			if featureKeys[f.Key] {
				return fmt.Errorf("%w: duplicate feature key %s in module %s", ErrInvalidConstraintSchema, f.Key, m.Key)
			}
			featureKeys[f.Key] = true

			switch f.Type {
			case domain.FeatureTypeNumber, domain.FeatureTypeEnum, domain.FeatureTypeMultiSelect:
				// valid
			default:
				return fmt.Errorf("%w: invalid feature type %s for %s.%s", ErrInvalidConstraintSchema, f.Type, m.Key, f.Key)
			}
		}
	}
	return nil
}

func validateConstraintValues(schemaJSON json.RawMessage, valuesJSON json.RawMessage) error {
	var schema domain.ConstraintSchema
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		return fmt.Errorf("%w: cannot parse schema: %v", ErrInvalidConstraintValues, err)
	}

	var values map[string]map[string]any
	if err := json.Unmarshal(valuesJSON, &values); err != nil {
		return fmt.Errorf("%w: constraint values must be {moduleKey: {featureKey: value}}", ErrInvalidConstraintValues)
	}

	// Build schema lookup
	schemaLookup := make(map[string]map[string]domain.ConstraintFeature)
	for _, m := range schema {
		features := make(map[string]domain.ConstraintFeature)
		for _, f := range m.Features {
			features[f.Key] = f
		}
		schemaLookup[m.Key] = features
	}

	// Validate values against schema
	for moduleKey, moduleValues := range values {
		features, ok := schemaLookup[moduleKey]
		if !ok {
			return fmt.Errorf("%w: unknown module key: %s", ErrInvalidConstraintValues, moduleKey)
		}
		for featureKey, value := range moduleValues {
			// "enabled" is a reserved module-level toggle, not a feature key
			if featureKey == "enabled" {
				continue
			}
			feature, ok := features[featureKey]
			if !ok {
				return fmt.Errorf("%w: unknown feature key %s in module %s", ErrInvalidConstraintValues, featureKey, moduleKey)
			}
			if err := validateFeatureValue(feature, value, moduleKey); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateFeatureValue(feature domain.ConstraintFeature, value any, moduleKey string) error {
	switch feature.Type {
	case domain.FeatureTypeNumber:
		num, ok := toFloat64(value)
		if !ok {
			return fmt.Errorf("%w: %s.%s must be a number", ErrInvalidConstraintValues, moduleKey, feature.Key)
		}
		if feature.Min != nil && num < *feature.Min {
			return fmt.Errorf("%w: %s.%s value %v is less than min %v", ErrInvalidConstraintValues, moduleKey, feature.Key, num, *feature.Min)
		}
		if feature.Max != nil && num > *feature.Max {
			return fmt.Errorf("%w: %s.%s value %v exceeds max %v", ErrInvalidConstraintValues, moduleKey, feature.Key, num, *feature.Max)
		}
	case domain.FeatureTypeEnum:
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("%w: %s.%s must be a string", ErrInvalidConstraintValues, moduleKey, feature.Key)
		}
		if len(feature.Options) > 0 && !contains(feature.Options, str) {
			return fmt.Errorf("%w: %s.%s value %q not in options", ErrInvalidConstraintValues, moduleKey, feature.Key, str)
		}
	case domain.FeatureTypeMultiSelect:
		arr, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%w: %s.%s must be an array", ErrInvalidConstraintValues, moduleKey, feature.Key)
		}
		for _, v := range arr {
			str, ok := v.(string)
			if !ok {
				return fmt.Errorf("%w: %s.%s array items must be strings", ErrInvalidConstraintValues, moduleKey, feature.Key)
			}
			if len(feature.Options) > 0 && !contains(feature.Options, str) {
				return fmt.Errorf("%w: %s.%s value %q not in options", ErrInvalidConstraintValues, moduleKey, feature.Key, str)
			}
		}
	}
	return nil
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
