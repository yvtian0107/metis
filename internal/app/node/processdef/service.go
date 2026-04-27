package processdef

import (
	"encoding/json"
	"errors"
	"log/slog"
	"metis/internal/app/node/domain"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

var (
	ErrProcessDefNotFound   = errors.New("process definition not found")
	ErrProcessDefNameExists = errors.New("process definition name already exists")
)

type NodeProcessLister interface {
	ListByProcessDefID(processDefID uint) ([]domain.NodeProcess, error)
}

type CommandCreator interface {
	Create(cmd *domain.NodeCommand) error
}

type CommandPusher interface {
	SendCommand(nodeID uint, cmd *domain.NodeCommand) bool
}

type ProcessDefService struct {
	processDefRepo  *ProcessDefRepo
	nodeProcessRepo NodeProcessLister
	commandRepo     CommandCreator
	hub             CommandPusher
}

func NewProcessDefService(i do.Injector) (*ProcessDefService, error) {
	return &ProcessDefService{
		processDefRepo:  do.MustInvoke[*ProcessDefRepo](i),
		nodeProcessRepo: do.MustInvoke[NodeProcessLister](i),
		commandRepo:     do.MustInvoke[CommandCreator](i),
		hub:             do.MustInvoke[CommandPusher](i),
	}, nil
}

func (s *ProcessDefService) Create(pd *domain.ProcessDef) error {
	if _, err := s.processDefRepo.FindByName(pd.Name); err == nil {
		return ErrProcessDefNameExists
	}
	return s.processDefRepo.Create(pd)
}

func (s *ProcessDefService) Get(id uint) (*domain.ProcessDef, error) {
	pd, err := s.processDefRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProcessDefNotFound
		}
		return nil, err
	}
	return pd, nil
}

func (s *ProcessDefService) List(params ProcessDefListParams) ([]domain.ProcessDef, int64, error) {
	return s.processDefRepo.List(params)
}

func (s *ProcessDefService) Update(id uint, updates map[string]any) (*domain.ProcessDef, error) {
	pd, err := s.processDefRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProcessDefNotFound
		}
		return nil, err
	}

	if err := s.processDefRepo.Update(id, updates); err != nil {
		return nil, err
	}

	// Push config.update to all nodes running this process
	nodeProcesses, _ := s.nodeProcessRepo.ListByProcessDefID(id)
	payload, _ := json.Marshal(map[string]any{
		"process_def_id": id,
		"process_name":   pd.Name,
	})
	for _, np := range nodeProcesses {
		cmd := &domain.NodeCommand{
			NodeID:  np.NodeID,
			Type:    domain.CommandTypeConfigUpdate,
			Payload: domain.JSONMap(payload),
			Status:  domain.CommandStatusPending,
		}
		if err := s.commandRepo.Create(cmd); err != nil {
			slog.Warn("failed to enqueue config.update", "nodeId", np.NodeID, "processDef", pd.Name, "error", err)
			continue
		}
		// Push via SSE
		s.hub.SendCommand(np.NodeID, cmd)
	}

	return s.processDefRepo.FindByID(id)
}

func (s *ProcessDefService) Delete(id uint) error {
	_, err := s.processDefRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProcessDefNotFound
		}
		return err
	}

	// Enqueue stop commands for all nodes running this process
	nodeProcesses, _ := s.nodeProcessRepo.ListByProcessDefID(id)
	payload, _ := json.Marshal(map[string]any{"process_def_id": id})
	for _, np := range nodeProcesses {
		cmd := &domain.NodeCommand{
			NodeID:  np.NodeID,
			Type:    domain.CommandTypeProcessStop,
			Payload: domain.JSONMap(payload),
			Status:  domain.CommandStatusPending,
		}
		if err := s.commandRepo.Create(cmd); err != nil {
			slog.Warn("failed to enqueue stop on delete", "nodeId", np.NodeID, "error", err)
			continue
		}
		s.hub.SendCommand(np.NodeID, cmd)
	}

	return s.processDefRepo.Delete(id)
}
