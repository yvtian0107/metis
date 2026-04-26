package sidecar

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"metis/internal/app/node/command"
	"metis/internal/app/node/domain"
	nodenode "metis/internal/app/node/node"
	nodeprocess "metis/internal/app/node/process"
	"metis/internal/app/node/processdef"
	"text/template"
	"time"

	"github.com/samber/do/v2"
)

const (
	heartbeatTimeout = 60 * time.Second
	commandTimeout   = 5 * time.Minute
)

type SidecarService struct {
	nodeRepo        *nodenode.NodeRepo
	processDefRepo  *processdef.ProcessDefRepo
	nodeProcessRepo *nodeprocess.NodeProcessRepo
	commandRepo     *command.NodeCommandRepo
	hub             *nodenode.NodeHub
}

func NewSidecarService(i do.Injector) (*SidecarService, error) {
	return &SidecarService{
		nodeRepo:        do.MustInvoke[*nodenode.NodeRepo](i),
		processDefRepo:  do.MustInvoke[*processdef.ProcessDefRepo](i),
		nodeProcessRepo: do.MustInvoke[*nodeprocess.NodeProcessRepo](i),
		commandRepo:     do.MustInvoke[*command.NodeCommandRepo](i),
		hub:             do.MustInvoke[*nodenode.NodeHub](i),
	}, nil
}

type RegisterRequest struct {
	SystemInfo   json.RawMessage `json:"systemInfo"`
	Capabilities json.RawMessage `json:"capabilities"`
	Version      string          `json:"version"`
}

func (s *SidecarService) Register(nodeID uint, req RegisterRequest) error {
	now := time.Now()
	if err := s.nodeRepo.Update(nodeID, map[string]any{
		"status":         domain.NodeStatusOnline,
		"system_info":    domain.JSONMap(req.SystemInfo),
		"capabilities":   domain.JSONMap(req.Capabilities),
		"version":        req.Version,
		"last_heartbeat": &now,
	}); err != nil {
		return err
	}

	// Enqueue process.start commands for all bound processes
	s.enqueueStartCommandsForNode(nodeID)

	return nil
}

// enqueueStartCommandsForNode sends a process.start command for every process
// currently bound to the given node so the sidecar picks them up after (re-)registration.
func (s *SidecarService) enqueueStartCommandsForNode(nodeID uint) {
	nodeProcesses, err := s.nodeProcessRepo.ListByNodeID(nodeID)
	if err != nil || len(nodeProcesses) == 0 {
		return
	}

	for _, np := range nodeProcesses {
		pd, err := s.processDefRepo.FindByID(np.ProcessDefID)
		if err != nil {
			continue
		}

		payload, _ := json.Marshal(map[string]any{
			"process_def_id":  pd.ID,
			"node_process_id": np.ID,
			"override_vars":   json.RawMessage(np.OverrideVars),
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
		cmd := &domain.NodeCommand{
			NodeID:  nodeID,
			Type:    domain.CommandTypeProcessStart,
			Payload: domain.JSONMap(payload),
			Status:  domain.CommandStatusPending,
		}
		if err := s.commandRepo.Create(cmd); err != nil {
			slog.Warn("failed to enqueue start command on register", "nodeId", nodeID, "processDef", pd.Name, "error", err)
			continue
		}
		s.hub.SendCommand(nodeID, cmd)
	}
}

type HeartbeatRequest struct {
	Processes []ProcessStatus `json:"processes"`
	Version   string          `json:"version"`
}

type ProcessStatus struct {
	ProcessDefID  uint            `json:"processDefId"`
	Status        string          `json:"status"`
	PID           int             `json:"pid"`
	ConfigVersion string          `json:"configVersion"`
	ProbeResult   json.RawMessage `json:"probeResult,omitempty"`
}

func (s *SidecarService) Heartbeat(nodeID uint, req HeartbeatRequest) error {
	now := time.Now()
	if err := s.nodeRepo.Update(nodeID, map[string]any{
		"status":         domain.NodeStatusOnline,
		"last_heartbeat": &now,
		"version":        req.Version,
	}); err != nil {
		return err
	}

	// Sync process statuses
	for _, ps := range req.Processes {
		np, err := s.nodeProcessRepo.FindByNodeAndProcessDef(nodeID, ps.ProcessDefID)
		if err != nil {
			continue
		}
		_ = s.nodeProcessRepo.UpdateStatus(np.ID, ps.Status, ps.PID)
		if ps.ConfigVersion != "" {
			_ = s.nodeProcessRepo.UpdateConfigVersion(np.ID, ps.ConfigVersion)
		}
		if len(ps.ProbeResult) > 0 {
			_ = s.nodeProcessRepo.UpdateProbe(np.ID, domain.JSONMap(ps.ProbeResult))
		}
	}
	return nil
}

func (s *SidecarService) PollCommands(nodeID uint) ([]domain.NodeCommand, error) {
	return s.commandRepo.FindPendingByNodeID(nodeID)
}

func (s *SidecarService) AckCommand(commandID uint, nodeID uint, success bool, result string) error {
	cmd, err := s.commandRepo.FindByID(commandID)
	if err != nil {
		return err
	}
	if cmd.NodeID != nodeID {
		return fmt.Errorf("command does not belong to this node")
	}

	if success {
		return s.commandRepo.Ack(commandID, result)
	}
	return s.commandRepo.Fail(commandID, result)
}

type ConfigFile struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

func (s *SidecarService) RenderConfig(nodeID uint, processName string, filename string) (string, string, error) {
	node, err := s.nodeRepo.FindByID(nodeID)
	if err != nil {
		return "", "", err
	}

	pd, err := s.processDefRepo.FindByName(processName)
	if err != nil {
		return "", "", fmt.Errorf("process definition %q not found", processName)
	}

	// Parse config files from domain.ProcessDef
	var configFiles []ConfigFile
	if err := json.Unmarshal([]byte(pd.ConfigFiles), &configFiles); err != nil || len(configFiles) == 0 {
		return "", "", fmt.Errorf("no config files defined for process %q", processName)
	}

	// Find the target config file
	var targetFile *ConfigFile
	if filename != "" {
		for i := range configFiles {
			if configFiles[i].Filename == filename {
				targetFile = &configFiles[i]
				break
			}
		}
		if targetFile == nil {
			return "", "", fmt.Errorf("config file %q not found for process %q", filename, processName)
		}
	} else {
		// Backward compatible: render first file
		targetFile = &configFiles[0]
	}

	// Find the node process for override vars
	var overrideVars map[string]any
	if np, err := s.nodeProcessRepo.FindByNodeAndProcessDef(nodeID, pd.ID); err == nil {
		_ = json.Unmarshal([]byte(np.OverrideVars), &overrideVars)
	}

	// Build template context
	var labels map[string]any
	_ = json.Unmarshal([]byte(node.Labels), &labels)

	templateCtx := map[string]any{
		"domain.Node": labels,
		"Override":    overrideVars,
	}

	// Render the target config file
	tmpl, err := template.New("config").Parse(targetFile.Content)
	if err != nil {
		return "", "", fmt.Errorf("template parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateCtx); err != nil {
		return "", "", fmt.Errorf("template render error: %w", err)
	}

	rendered := buf.String()
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(rendered)))

	return rendered, hash, nil
}

func (s *SidecarService) DetectOfflineNodes(_ context.Context, _ json.RawMessage) error {
	affected, err := s.nodeRepo.MarkOffline(heartbeatTimeout)
	if err != nil {
		return err
	}
	if affected > 0 {
		slog.Info("node offline detection", "marked_offline", affected)
	}
	return nil
}

func (s *SidecarService) CleanupExpiredCommands(_ context.Context, _ json.RawMessage) error {
	affected, err := s.commandRepo.CleanupExpired(commandTimeout)
	if err != nil {
		return err
	}
	if affected > 0 {
		slog.Info("node command cleanup", "expired_commands", affected)
	}
	return nil
}
