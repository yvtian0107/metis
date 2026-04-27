package process

import (
	"encoding/json"
	"errors"
	"fmt"
	"metis/internal/app/node/domain"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

var (
	ErrNodeProcessNotFound = errors.New("node process binding not found")
	ErrNodeProcessExists   = errors.New("process already bound to this node")
	ErrNodeNotFound        = errors.New("node not found")
	ErrProcessDefNotFound  = errors.New("process definition not found")
)

type NodeReader interface {
	FindByID(id uint) (*domain.Node, error)
}

type ProcessDefReader interface {
	FindByID(id uint) (*domain.ProcessDef, error)
}

type CommandCreator interface {
	Create(cmd *domain.NodeCommand) error
}

type CommandPusher interface {
	SendCommand(nodeID uint, cmd *domain.NodeCommand) bool
}

type NodeProcessService struct {
	nodeRepo        NodeReader
	processDefRepo  ProcessDefReader
	nodeProcessRepo *NodeProcessRepo
	commandRepo     CommandCreator
	hub             CommandPusher
}

func NewNodeProcessService(i do.Injector) (*NodeProcessService, error) {
	return &NodeProcessService{
		nodeRepo:        do.MustInvoke[NodeReader](i),
		processDefRepo:  do.MustInvoke[ProcessDefReader](i),
		nodeProcessRepo: do.MustInvoke[*NodeProcessRepo](i),
		commandRepo:     do.MustInvoke[CommandCreator](i),
		hub:             do.MustInvoke[CommandPusher](i),
	}, nil
}

// createAndPushCommand persists a command to DB and pushes it via SSE if the node is online.
func (s *NodeProcessService) createAndPushCommand(cmd *domain.NodeCommand) error {
	if err := s.commandRepo.Create(cmd); err != nil {
		return err
	}
	// Best-effort push via SSE; if offline, command stays in DB for delivery on reconnect
	s.hub.SendCommand(cmd.NodeID, cmd)
	return nil
}

// buildStartPayload builds the full process.start command payload.
func (s *NodeProcessService) buildStartPayload(np *domain.NodeProcess, pd *domain.ProcessDef) domain.JSONMap {
	payload, _ := json.Marshal(map[string]any{
		"process_def_id":  pd.ID,
		"node_process_id": np.ID,
		"process_def": map[string]any{
			"id":            pd.ID,
			"name":          pd.Name,
			"startCommand":  pd.StartCommand,
			"stopCommand":   pd.StopCommand,
			"reloadCommand": pd.ReloadCommand,
			"env":           json.RawMessage(pd.Env),
			"configFiles":   json.RawMessage(pd.ConfigFiles),
			"probeType":     pd.ProbeType,
			"probeConfig":   json.RawMessage(pd.ProbeConfig),
			"restartPolicy": pd.RestartPolicy,
			"maxRestarts":   pd.MaxRestarts,
		},
	})
	return domain.JSONMap(payload)
}

func (s *NodeProcessService) Bind(nodeID, processDefID uint) (*domain.NodeProcess, error) {
	if _, err := s.nodeRepo.FindByID(nodeID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNodeNotFound
		}
		return nil, err
	}
	pd, err := s.processDefRepo.FindByID(processDefID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProcessDefNotFound
		}
		return nil, err
	}

	// Check if already bound
	if _, err := s.nodeProcessRepo.FindByNodeAndProcessDef(nodeID, processDefID); err == nil {
		return nil, ErrNodeProcessExists
	}

	np := &domain.NodeProcess{
		NodeID:       nodeID,
		ProcessDefID: processDefID,
		Status:       domain.ProcessStatusPendingConfig,
	}
	if err := s.nodeProcessRepo.Create(np); err != nil {
		return nil, err
	}

	cmd := &domain.NodeCommand{
		NodeID:  nodeID,
		Type:    domain.CommandTypeProcessStart,
		Payload: s.buildStartPayload(np, pd),
		Status:  domain.CommandStatusPending,
	}
	if err := s.createAndPushCommand(cmd); err != nil {
		return np, fmt.Errorf("failed to enqueue start command: %w", err)
	}

	return np, nil
}

func (s *NodeProcessService) ListByNodeID(nodeID uint) ([]NodeProcessDetail, error) {
	return s.nodeProcessRepo.ListByNodeID(nodeID)
}

// lookupBinding finds the domain.NodeProcess binding or returns ErrNodeProcessNotFound.
func (s *NodeProcessService) lookupBinding(nodeID, processDefID uint) (*domain.NodeProcess, error) {
	np, err := s.nodeProcessRepo.FindByNodeAndProcessDef(nodeID, processDefID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNodeProcessNotFound
		}
		return nil, err
	}
	return np, nil
}

func (s *NodeProcessService) Unbind(nodeID, processDefID uint) error {
	np, err := s.lookupBinding(nodeID, processDefID)
	if err != nil {
		return err
	}

	var processName string
	if pd, err := s.processDefRepo.FindByID(processDefID); err == nil {
		processName = pd.Name
	}

	payload, _ := json.Marshal(map[string]any{
		"process_def_id":  processDefID,
		"node_process_id": np.ID,
		"process_name":    processName,
	})
	cmd := &domain.NodeCommand{
		NodeID:  nodeID,
		Type:    domain.CommandTypeProcessStop,
		Payload: domain.JSONMap(payload),
		Status:  domain.CommandStatusPending,
	}
	if err := s.createAndPushCommand(cmd); err != nil {
		return fmt.Errorf("failed to enqueue stop command: %w", err)
	}

	return s.nodeProcessRepo.Delete(np.ID)
}

func (s *NodeProcessService) Start(nodeID, processDefID uint) error {
	np, err := s.lookupBinding(nodeID, processDefID)
	if err != nil {
		return err
	}

	pd, err := s.processDefRepo.FindByID(processDefID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProcessDefNotFound
		}
		return err
	}

	cmd := &domain.NodeCommand{
		NodeID:  nodeID,
		Type:    domain.CommandTypeProcessStart,
		Payload: s.buildStartPayload(np, pd),
		Status:  domain.CommandStatusPending,
	}
	return s.createAndPushCommand(cmd)
}

func (s *NodeProcessService) Stop(nodeID, processDefID uint) error {
	np, err := s.lookupBinding(nodeID, processDefID)
	if err != nil {
		return err
	}

	var processName string
	if pd, err := s.processDefRepo.FindByID(processDefID); err == nil {
		processName = pd.Name
	}

	payload, _ := json.Marshal(map[string]any{
		"process_def_id":  processDefID,
		"node_process_id": np.ID,
		"process_name":    processName,
	})
	cmd := &domain.NodeCommand{
		NodeID:  nodeID,
		Type:    domain.CommandTypeProcessStop,
		Payload: domain.JSONMap(payload),
		Status:  domain.CommandStatusPending,
	}
	return s.createAndPushCommand(cmd)
}

func (s *NodeProcessService) Restart(nodeID, processDefID uint) error {
	np, err := s.lookupBinding(nodeID, processDefID)
	if err != nil {
		return err
	}

	var processName string
	if pd, err := s.processDefRepo.FindByID(processDefID); err == nil {
		processName = pd.Name
	}

	payload, _ := json.Marshal(map[string]any{
		"process_def_id":  processDefID,
		"node_process_id": np.ID,
		"process_name":    processName,
	})
	cmd := &domain.NodeCommand{
		NodeID:  nodeID,
		Type:    domain.CommandTypeProcessRestart,
		Payload: domain.JSONMap(payload),
		Status:  domain.CommandStatusPending,
	}
	return s.createAndPushCommand(cmd)
}

func (s *NodeProcessService) Reload(nodeID, processDefID uint) error {
	_, err := s.lookupBinding(nodeID, processDefID)
	if err != nil {
		return err
	}

	pd, err := s.processDefRepo.FindByID(processDefID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProcessDefNotFound
		}
		return err
	}

	payload, _ := json.Marshal(map[string]any{
		"process_def_id": processDefID,
		"process_name":   pd.Name,
	})
	cmd := &domain.NodeCommand{
		NodeID:  nodeID,
		Type:    domain.CommandTypeConfigUpdate,
		Payload: domain.JSONMap(payload),
		Status:  domain.CommandStatusPending,
	}
	return s.createAndPushCommand(cmd)
}

func (s *NodeProcessService) ListNodesByProcessDefID(processDefID uint) ([]ProcessDefNodeDetail, error) {
	return s.nodeProcessRepo.ListNodesByProcessDefID(processDefID)
}
