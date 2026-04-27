package node

import (
	"encoding/json"
	"errors"
	"log/slog"
	"metis/internal/app/node/command"
	"metis/internal/app/node/domain"
	"metis/internal/app/node/process"
	"metis/internal/app/node/processdef"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

var (
	ErrNodeNotFound   = errors.New("node not found")
	ErrNodeNameExists = errors.New("node name already exists")
)

type NodeService struct {
	nodeRepo       *NodeRepo
	processRepo    *process.NodeProcessRepo
	processDefRepo *processdef.ProcessDefRepo
	commandRepo    *command.NodeCommandRepo
}

func NewNodeService(i do.Injector) (*NodeService, error) {
	return &NodeService{
		nodeRepo:       do.MustInvoke[*NodeRepo](i),
		processRepo:    do.MustInvoke[*process.NodeProcessRepo](i),
		processDefRepo: do.MustInvoke[*processdef.ProcessDefRepo](i),
		commandRepo:    do.MustInvoke[*command.NodeCommandRepo](i),
	}, nil
}

type CreateNodeResult struct {
	Node  *domain.Node
	Token string // raw token, display once
}

func (s *NodeService) Create(name string, labels domain.JSONMap) (*CreateNodeResult, error) {
	if _, err := s.nodeRepo.FindByName(name); err == nil {
		return nil, ErrNodeNameExists
	}

	raw, hash, prefix, err := domain.GenerateNodeToken()
	if err != nil {
		return nil, err
	}

	node := &domain.Node{
		Name:        name,
		TokenHash:   hash,
		TokenPrefix: prefix,
		Status:      domain.NodeStatusPending,
		Labels:      labels,
	}
	if err := s.nodeRepo.Create(node); err != nil {
		return nil, err
	}

	return &CreateNodeResult{Node: node, Token: raw}, nil
}

func (s *NodeService) Get(id uint) (*domain.Node, error) {
	node, err := s.nodeRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNodeNotFound
		}
		return nil, err
	}
	return node, nil
}

func (s *NodeService) List(params NodeListParams) ([]NodeListItem, int64, error) {
	return s.nodeRepo.List(params)
}

func (s *NodeService) Update(id uint, name *string, labels *domain.JSONMap) (*domain.Node, error) {
	node, err := s.nodeRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNodeNotFound
		}
		return nil, err
	}

	updates := map[string]any{}
	if name != nil {
		// Check uniqueness
		existing, err := s.nodeRepo.FindByName(*name)
		if err == nil && existing.ID != id {
			return nil, ErrNodeNameExists
		}
		updates["name"] = *name
	}
	if labels != nil {
		updates["labels"] = *labels
	}

	if len(updates) > 0 {
		if err := s.nodeRepo.Update(id, updates); err != nil {
			return nil, err
		}
		node, _ = s.nodeRepo.FindByID(id)
	}
	return node, nil
}

func (s *NodeService) Delete(id uint) error {
	_, err := s.nodeRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNodeNotFound
		}
		return err
	}

	// Enqueue process.stop for all bound processes
	processes, _ := s.processRepo.ListByNodeID(id)
	for _, np := range processes {
		var processName string
		if pd, err := s.processDefRepo.FindByID(np.ProcessDefID); err == nil {
			processName = pd.Name
		}
		payload, _ := json.Marshal(map[string]any{
			"process_def_id":  np.ProcessDefID,
			"node_process_id": np.ID,
			"process_name":    processName,
		})
		cmd := &domain.NodeCommand{
			NodeID:  id,
			Type:    domain.CommandTypeProcessStop,
			Payload: domain.JSONMap(payload),
			Status:  domain.CommandStatusPending,
		}
		if err := s.commandRepo.Create(cmd); err != nil {
			slog.Warn("failed to enqueue stop command on delete", "nodeId", id, "error", err)
		}
	}

	// Mark all processes as stopped
	_ = s.processRepo.BatchUpdateStatusByNodeID(id, domain.ProcessStatusStopped)

	// Cleanup pending commands (except the stop commands we just created)
	_ = s.commandRepo.FailPendingByNodeID(id, "node_deleted")

	return s.nodeRepo.Delete(id)
}

func (s *NodeService) RotateToken(id uint) (string, error) {
	_, err := s.nodeRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrNodeNotFound
		}
		return "", err
	}

	raw, hash, prefix, err := domain.GenerateNodeToken()
	if err != nil {
		return "", err
	}

	if err := s.nodeRepo.UpdateToken(id, hash, prefix); err != nil {
		return "", err
	}
	return raw, nil
}
