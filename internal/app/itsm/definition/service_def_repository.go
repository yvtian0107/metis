package definition

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
	. "metis/internal/app/itsm/domain"

	"metis/internal/database"
)

type ServiceDefRepo struct {
	db *database.DB
}

func NewServiceDefRepo(i do.Injector) (*ServiceDefRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &ServiceDefRepo{db: db}, nil
}

func GetOrCreateServiceRuntimeVersion(db *gorm.DB, serviceID uint) (*ServiceDefinitionVersion, error) {
	repo := &ServiceDefRepo{db: &database.DB{DB: db}}
	return repo.GetOrCreateRuntimeVersion(serviceID)
}

func (r *ServiceDefRepo) Create(svc *ServiceDefinition) error {
	return r.db.Create(svc).Error
}

func (r *ServiceDefRepo) FindByID(id uint) (*ServiceDefinition, error) {
	var s ServiceDefinition
	if err := r.db.First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *ServiceDefRepo) FindByCode(code string) (*ServiceDefinition, error) {
	var s ServiceDefinition
	if err := r.db.Where("code = ?", code).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *ServiceDefRepo) GetOrCreateRuntimeVersion(serviceID uint) (*ServiceDefinitionVersion, error) {
	var svc ServiceDefinition
	if err := r.db.First(&svc, serviceID).Error; err != nil {
		return nil, err
	}
	snapshot, hash, err := r.buildRuntimeVersionSnapshot(&svc)
	if err != nil {
		return nil, err
	}

	var existing ServiceDefinitionVersion
	err = r.db.Where("service_id = ? AND content_hash = ?", serviceID, hash).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var maxVersion int
	if err := r.db.Model(&ServiceDefinitionVersion{}).
		Where("service_id = ?", serviceID).
		Select("COALESCE(MAX(version), 0)").
		Scan(&maxVersion).Error; err != nil {
		return nil, err
	}
	snapshot.Version = maxVersion + 1
	if err := r.db.Create(snapshot).Error; err != nil {
		if err := r.db.Where("service_id = ? AND content_hash = ?", serviceID, hash).First(&existing).Error; err == nil {
			return &existing, nil
		}
		return nil, err
	}
	return snapshot, nil
}

func (r *ServiceDefRepo) buildRuntimeVersionSnapshot(svc *ServiceDefinition) (*ServiceDefinitionVersion, string, error) {
	var actions []ServiceAction
	if err := r.db.Where("service_id = ?", svc.ID).Order("id ASC").Find(&actions).Error; err != nil {
		return nil, "", err
	}
	actionResponses := make([]ServiceActionResponse, len(actions))
	for i, action := range actions {
		actionResponses[i] = action.ToResponse()
	}
	actionsJSON, err := json.Marshal(actionResponses)
	if err != nil {
		return nil, "", err
	}

	slaTemplateJSON, escalationRulesJSON, err := r.buildSLASnapshots(svc.SLAID)
	if err != nil {
		return nil, "", err
	}

	content := struct {
		ServiceID           uint
		EngineType          string
		SLAID               *uint
		IntakeFormSchema    JSONField
		WorkflowJSON        JSONField
		CollaborationSpec   string
		AgentID             *uint
		AgentConfig         JSONField
		KnowledgeBaseIDs    JSONField
		ActionsJSON         json.RawMessage
		SLATemplateJSON     json.RawMessage
		EscalationRulesJSON json.RawMessage
	}{
		ServiceID:           svc.ID,
		EngineType:          svc.EngineType,
		SLAID:               svc.SLAID,
		IntakeFormSchema:    svc.IntakeFormSchema,
		WorkflowJSON:        svc.WorkflowJSON,
		CollaborationSpec:   svc.CollaborationSpec,
		AgentID:             svc.AgentID,
		AgentConfig:         svc.AgentConfig,
		KnowledgeBaseIDs:    svc.KnowledgeBaseIDs,
		ActionsJSON:         actionsJSON,
		SLATemplateJSON:     slaTemplateJSON,
		EscalationRulesJSON: escalationRulesJSON,
	}
	contentJSON, err := json.Marshal(content)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(contentJSON)
	hash := hex.EncodeToString(sum[:])
	return &ServiceDefinitionVersion{
		ServiceID:           svc.ID,
		ContentHash:         hash,
		EngineType:          svc.EngineType,
		SLAID:               svc.SLAID,
		IntakeFormSchema:    svc.IntakeFormSchema,
		WorkflowJSON:        svc.WorkflowJSON,
		CollaborationSpec:   svc.CollaborationSpec,
		AgentID:             svc.AgentID,
		AgentConfig:         svc.AgentConfig,
		KnowledgeBaseIDs:    svc.KnowledgeBaseIDs,
		ActionsJSON:         JSONField(actionsJSON),
		SLATemplateJSON:     JSONField(slaTemplateJSON),
		EscalationRulesJSON: JSONField(escalationRulesJSON),
	}, hash, nil
}

func (r *ServiceDefRepo) buildSLASnapshots(slaID *uint) (json.RawMessage, json.RawMessage, error) {
	if slaID == nil || *slaID == 0 {
		return nil, nil, nil
	}

	var sla SLATemplate
	if err := r.db.First(&sla, *slaID).Error; err != nil {
		return nil, nil, err
	}
	slaJSON, err := json.Marshal(sla.ToResponse())
	if err != nil {
		return nil, nil, err
	}

	var rules []EscalationRule
	if err := r.db.Where("sla_id = ? AND is_active = ?", *slaID, true).
		Order("level ASC, id ASC").
		Find(&rules).Error; err != nil {
		return nil, nil, err
	}
	ruleResponses := make([]EscalationRuleResponse, len(rules))
	for i := range rules {
		ruleResponses[i] = rules[i].ToResponse()
	}
	rulesJSON, err := json.Marshal(ruleResponses)
	if err != nil {
		return nil, nil, err
	}
	return slaJSON, rulesJSON, nil
}

func (r *ServiceDefRepo) Update(id uint, updates map[string]any) error {
	return r.db.Model(&ServiceDefinition{}).Where("id = ?", id).Updates(updates).Error
}

func (r *ServiceDefRepo) Delete(id uint) error {
	return r.db.Delete(&ServiceDefinition{}, id).Error
}

type ServiceDefListParams struct {
	CatalogID     *uint
	RootCatalogID *uint
	EngineType    *string
	Keyword       string
	IsActive      *bool
	Page          int
	PageSize      int
}

const MaxServiceDefPageSize = 100

func (r *ServiceDefRepo) List(params ServiceDefListParams) ([]ServiceDefinition, int64, error) {
	query := r.db.Model(&ServiceDefinition{})

	if params.CatalogID != nil {
		query = query.Where("catalog_id = ?", *params.CatalogID)
	}
	if params.RootCatalogID != nil {
		query = query.Where(
			"catalog_id = ? OR catalog_id IN (SELECT id FROM itsm_service_catalogs WHERE parent_id = ?)",
			*params.RootCatalogID,
			*params.RootCatalogID,
		)
	}
	if params.EngineType != nil {
		query = query.Where("engine_type = ?", *params.EngineType)
	}
	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		query = query.Where("name LIKE ? OR code LIKE ? OR description LIKE ?", like, like, like)
	}
	if params.IsActive != nil {
		query = query.Where("is_active = ?", *params.IsActive)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}
	if params.PageSize > MaxServiceDefPageSize {
		params.PageSize = MaxServiceDefPageSize
	}

	var items []ServiceDefinition
	offset := (params.Page - 1) * params.PageSize
	if err := query.Offset(offset).Limit(params.PageSize).Order("sort_order ASC, id DESC").Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
