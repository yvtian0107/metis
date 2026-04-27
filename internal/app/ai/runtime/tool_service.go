package runtime

import (
	"errors"

	"github.com/samber/do/v2"
)

var (
	ErrToolNotFound            = errors.New("tool not found")
	ErrToolNotExecutable       = errors.New("tool is not executable")
	ErrToolDisableNeedsConfirm = errors.New("tool disable needs confirmation")
)

type ToolService struct {
	repo       *ToolRepo
	runtimeSvc *ToolRuntimeService
	registries []ToolHandlerRegistry
}

func NewToolService(i do.Injector) (*ToolService, error) {
	return &ToolService{
		repo:       do.MustInvoke[*ToolRepo](i),
		runtimeSvc: do.MustInvoke[*ToolRuntimeService](i),
		registries: collectToolRegistries(i),
	}, nil
}

type ToggleToolOptions struct {
	ConfirmImpact bool
}

type ToolImpactError struct {
	ToolName        string
	BoundAgentCount int64
}

func (e *ToolImpactError) Error() string {
	return ErrToolDisableNeedsConfirm.Error()
}

func (s *ToolService) List() ([]ToolResponse, error) {
	tools, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	responses := make([]ToolResponse, 0, len(tools))
	for _, tool := range tools {
		resp, err := s.responseForTool(tool)
		if err != nil {
			return nil, err
		}
		responses = append(responses, resp)
	}
	return responses, nil
}

func (s *ToolService) ToggleActive(id uint, isActive bool, opts ToggleToolOptions) (*ToolResponse, error) {
	t, err := s.repo.FindByID(id)
	if err != nil {
		return nil, ErrToolNotFound
	}
	availability := s.availabilityForTool(*t)
	if isActive && (availability.Status == ToolAvailabilityRiskDisabled || availability.Status == ToolAvailabilityUnimplemented) {
		return nil, ErrToolNotExecutable
	}
	boundCount, err := s.repo.CountBoundAgents(t.ID)
	if err != nil {
		return nil, err
	}
	if !isActive && t.IsActive && boundCount > 0 && !opts.ConfirmImpact {
		return nil, &ToolImpactError{ToolName: t.Name, BoundAgentCount: boundCount}
	}
	t.IsActive = isActive
	if err := s.repo.Update(t); err != nil {
		return nil, err
	}
	resp := t.ToResponse()
	resp = applyToolAvailability(resp, s.availabilityForTool(*t), boundCount)
	return &resp, nil
}

func (s *ToolService) responseForTool(t Tool) (ToolResponse, error) {
	boundCount, err := s.repo.CountBoundAgents(t.ID)
	if err != nil {
		return ToolResponse{}, err
	}
	return applyToolAvailability(t.ToResponse(), s.availabilityForTool(t), boundCount), nil
}

func (s *ToolService) availabilityForTool(t Tool) toolAvailability {
	availability := classifyToolAvailability(t, hasToolHandler(s.registries, t.Name))
	if availability.Status != ToolAvailabilityAvailable || len(t.RuntimeConfigSchema) == 0 {
		return availability
	}
	if err := s.runtimeSvc.ValidateRuntimeConfig(t); err != nil {
		return toolAvailability{
			IsExecutable: false,
			Status:       ToolAvailabilityNeedsConfig,
			Reason:       err.Error(),
		}
	}
	return availability
}
