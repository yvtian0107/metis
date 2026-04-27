package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"metis/internal/model"
)

// JSONMap is a map wrapper that handles SQLite TEXT columns for JSON objects.
type JSONMap json.RawMessage

func (j JSONMap) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "{}", nil
	}
	return string(j), nil
}

func (j *JSONMap) Scan(src any) error {
	switch v := src.(type) {
	case string:
		*j = JSONMap(v)
	case []byte:
		*j = append(JSONMap(nil), v...)
	case nil:
		*j = JSONMap("{}")
	default:
		return fmt.Errorf("JSONMap.Scan: unsupported type %T", src)
	}
	return nil
}

func (j JSONMap) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("{}"), nil
	}
	return []byte(j), nil
}

func (j *JSONMap) UnmarshalJSON(data []byte) error {
	*j = append(JSONMap(nil), data...)
	return nil
}

// JSONArray is a JSON array wrapper for SQLite TEXT columns.
type JSONArray json.RawMessage

func (j JSONArray) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "[]", nil
	}
	return string(j), nil
}

func (j *JSONArray) Scan(src any) error {
	switch v := src.(type) {
	case string:
		*j = JSONArray(v)
	case []byte:
		*j = append(JSONArray(nil), v...)
	case nil:
		*j = JSONArray("[]")
	default:
		return fmt.Errorf("JSONArray.Scan: unsupported type %T", src)
	}
	return nil
}

func (j JSONArray) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("[]"), nil
	}
	return []byte(j), nil
}

func (j *JSONArray) UnmarshalJSON(data []byte) error {
	*j = append(JSONArray(nil), data...)
	return nil
}

// --- Node statuses ---

const (
	NodeStatusPending = "pending"
	NodeStatusOnline  = "online"
	NodeStatusOffline = "offline"
)

// --- Process statuses ---

const (
	ProcessStatusRunning       = "running"
	ProcessStatusStopped       = "stopped"
	ProcessStatusError         = "error"
	ProcessStatusPendingConfig = "pending_config"
)

// --- Command types ---

const (
	CommandTypeProcessStart   = "process.start"
	CommandTypeProcessStop    = "process.stop"
	CommandTypeProcessRestart = "process.restart"
	CommandTypeConfigUpdate   = "config.update"
)

// --- Command statuses ---

const (
	CommandStatusPending = "pending"
	CommandStatusAcked   = "acked"
	CommandStatusFailed  = "failed"
)

// --- Restart policies ---

const (
	RestartPolicyAlways    = "always"
	RestartPolicyOnFailure = "on_failure"
	RestartPolicyNever     = "never"
)

// --- Probe types ---

const (
	ProbeTypeNone = "none"
	ProbeTypeHTTP = "http"
	ProbeTypeTCP  = "tcp"
	ProbeTypeExec = "exec"
)

// --- Data models ---

type Node struct {
	model.BaseModel
	Name          string     `json:"name" gorm:"size:128;not null;uniqueIndex"`
	TokenHash     string     `json:"-" gorm:"size:256;not null"`
	TokenPrefix   string     `json:"-" gorm:"size:8;not null;index"`
	Status        string     `json:"status" gorm:"size:16;not null;default:pending"`
	Labels        JSONMap    `json:"labels" gorm:"type:text"`
	SystemInfo    JSONMap    `json:"systemInfo" gorm:"type:text"`
	Capabilities  JSONMap    `json:"capabilities" gorm:"type:text"`
	Version       string     `json:"version" gorm:"size:64"`
	LastHeartbeat *time.Time `json:"lastHeartbeat"`
}

func (Node) TableName() string { return "nodes" }

type NodeResponse struct {
	ID            uint            `json:"id"`
	Name          string          `json:"name"`
	Status        string          `json:"status"`
	Labels        json.RawMessage `json:"labels"`
	SystemInfo    json.RawMessage `json:"systemInfo"`
	Capabilities  json.RawMessage `json:"capabilities"`
	Version       string          `json:"version"`
	LastHeartbeat *time.Time      `json:"lastHeartbeat"`
	ProcessCount  int             `json:"processCount,omitempty"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

func (n *Node) ToResponse() NodeResponse {
	return NodeResponse{
		ID:            n.ID,
		Name:          n.Name,
		Status:        n.Status,
		Labels:        json.RawMessage(n.Labels),
		SystemInfo:    json.RawMessage(n.SystemInfo),
		Capabilities:  json.RawMessage(n.Capabilities),
		Version:       n.Version,
		LastHeartbeat: n.LastHeartbeat,
		CreatedAt:     n.CreatedAt,
		UpdatedAt:     n.UpdatedAt,
	}
}

type ProcessDef struct {
	model.BaseModel
	Name           string    `json:"name" gorm:"size:128;not null;uniqueIndex"`
	DisplayName    string    `json:"displayName" gorm:"size:128;not null"`
	Description    string    `json:"description" gorm:"type:text"`
	StartCommand   string    `json:"startCommand" gorm:"size:512;not null"`
	StopCommand    string    `json:"stopCommand" gorm:"size:512"`
	ReloadCommand  string    `json:"reloadCommand" gorm:"size:512"`
	Env            JSONMap   `json:"env" gorm:"type:text"`
	ConfigFiles    JSONArray `json:"configFiles" gorm:"type:text"`
	ProbeType      string    `json:"probeType" gorm:"size:16;not null;default:none"`
	ProbeConfig    JSONMap   `json:"probeConfig" gorm:"type:text"`
	RestartPolicy  string    `json:"restartPolicy" gorm:"size:16;not null;default:always"`
	MaxRestarts    int       `json:"maxRestarts" gorm:"not null;default:10"`
	ResourceLimits JSONMap   `json:"resourceLimits" gorm:"type:text"`
}

func (ProcessDef) TableName() string { return "process_defs" }

type ProcessDefResponse struct {
	ID             uint            `json:"id"`
	Name           string          `json:"name"`
	DisplayName    string          `json:"displayName"`
	Description    string          `json:"description"`
	StartCommand   string          `json:"startCommand"`
	StopCommand    string          `json:"stopCommand"`
	ReloadCommand  string          `json:"reloadCommand"`
	Env            json.RawMessage `json:"env"`
	ConfigFiles    json.RawMessage `json:"configFiles"`
	ProbeType      string          `json:"probeType"`
	ProbeConfig    json.RawMessage `json:"probeConfig"`
	RestartPolicy  string          `json:"restartPolicy"`
	MaxRestarts    int             `json:"maxRestarts"`
	ResourceLimits json.RawMessage `json:"resourceLimits"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

func (p *ProcessDef) ToResponse() ProcessDefResponse {
	return ProcessDefResponse{
		ID:             p.ID,
		Name:           p.Name,
		DisplayName:    p.DisplayName,
		Description:    p.Description,
		StartCommand:   p.StartCommand,
		StopCommand:    p.StopCommand,
		ReloadCommand:  p.ReloadCommand,
		Env:            json.RawMessage(p.Env),
		ConfigFiles:    json.RawMessage(p.ConfigFiles),
		ProbeType:      p.ProbeType,
		ProbeConfig:    json.RawMessage(p.ProbeConfig),
		RestartPolicy:  p.RestartPolicy,
		MaxRestarts:    p.MaxRestarts,
		ResourceLimits: json.RawMessage(p.ResourceLimits),
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

type NodeProcess struct {
	model.BaseModel
	NodeID        uint    `json:"nodeId" gorm:"not null;index;uniqueIndex:idx_node_process"`
	ProcessDefID  uint    `json:"processDefId" gorm:"not null;index;uniqueIndex:idx_node_process"`
	Status        string  `json:"status" gorm:"size:16;not null;default:pending_config"`
	PID           int     `json:"pid" gorm:"column:pid;default:0"`
	ConfigVersion string  `json:"configVersion" gorm:"size:64"`
	LastProbe     JSONMap `json:"lastProbe" gorm:"type:text"`
	OverrideVars  JSONMap `json:"overrideVars" gorm:"type:text"`
}

func (NodeProcess) TableName() string { return "node_processes" }

type NodeProcessResponse struct {
	ID            uint            `json:"id"`
	NodeID        uint            `json:"nodeId"`
	ProcessDefID  uint            `json:"processDefId"`
	Status        string          `json:"status"`
	PID           int             `json:"pid"`
	ConfigVersion string          `json:"configVersion"`
	LastProbe     json.RawMessage `json:"lastProbe"`
	OverrideVars  json.RawMessage `json:"overrideVars"`
	ProcessName   string          `json:"processName,omitempty"`
	DisplayName   string          `json:"displayName,omitempty"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

func (np *NodeProcess) ToResponse() NodeProcessResponse {
	return NodeProcessResponse{
		ID:            np.ID,
		NodeID:        np.NodeID,
		ProcessDefID:  np.ProcessDefID,
		Status:        np.Status,
		PID:           np.PID,
		ConfigVersion: np.ConfigVersion,
		LastProbe:     json.RawMessage(np.LastProbe),
		OverrideVars:  json.RawMessage(np.OverrideVars),
		CreatedAt:     np.CreatedAt,
		UpdatedAt:     np.UpdatedAt,
	}
}

type NodeCommand struct {
	model.BaseModel
	NodeID  uint       `json:"nodeId" gorm:"not null;index"`
	Type    string     `json:"type" gorm:"size:32;not null"`
	Payload JSONMap    `json:"payload" gorm:"type:text"`
	Status  string     `json:"status" gorm:"size:16;not null;default:pending;index"`
	Result  string     `json:"result" gorm:"type:text"`
	AckedAt *time.Time `json:"ackedAt"`
}

func (NodeCommand) TableName() string { return "node_commands" }

type NodeCommandResponse struct {
	ID        uint            `json:"id"`
	NodeID    uint            `json:"nodeId"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Status    string          `json:"status"`
	Result    string          `json:"result"`
	AckedAt   *time.Time      `json:"ackedAt"`
	CreatedAt time.Time       `json:"createdAt"`
}

func (nc *NodeCommand) ToResponse() NodeCommandResponse {
	return NodeCommandResponse{
		ID:        nc.ID,
		NodeID:    nc.NodeID,
		Type:      nc.Type,
		Payload:   json.RawMessage(nc.Payload),
		Status:    nc.Status,
		Result:    nc.Result,
		AckedAt:   nc.AckedAt,
		CreatedAt: nc.CreatedAt,
	}
}

type NodeProcessLog struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeID       uint      `json:"nodeId" gorm:"not null;index:idx_npl_query"`
	ProcessDefID uint      `json:"processDefId" gorm:"index:idx_npl_query"`
	ProcessName  string    `json:"processName" gorm:"size:128"`
	Stream       string    `json:"stream" gorm:"size:8"` // stdout, stderr
	Content      string    `json:"content" gorm:"type:text"`
	Timestamp    time.Time `json:"timestamp" gorm:"not null;index"`
}

func (NodeProcessLog) TableName() string { return "node_process_logs" }
