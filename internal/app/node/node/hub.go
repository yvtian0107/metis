package node

import (
	"encoding/json"
	"log/slog"
	"metis/internal/app/node/domain"
	"sync"
	"time"
)

// SSEEvent represents an event to be pushed via SSE.
type SSEEvent struct {
	Event string `json:"event"` // "command", "config", "ping"
	Data  any    `json:"data"`
}

// NodeConn represents an active SSE connection to a sidecar node.
type NodeConn struct {
	EventCh     chan SSEEvent
	DoneCh      chan struct{}
	ConnectedAt time.Time
}

// NodeHub manages active SSE connections to sidecar nodes.
type NodeHub struct {
	mu          sync.RWMutex
	connections map[uint]*NodeConn
	nodeRepo    *NodeRepo
}

func NewNodeHub(nodeRepo *NodeRepo) *NodeHub {
	return &NodeHub{
		connections: make(map[uint]*NodeConn),
		nodeRepo:    nodeRepo,
	}
}

// Register creates a new SSE connection for a node.
// If a previous connection exists, it is closed first.
func (h *NodeHub) Register(nodeID uint) *NodeConn {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Close existing connection if any
	if old, ok := h.connections[nodeID]; ok {
		close(old.DoneCh)
		delete(h.connections, nodeID)
	}

	conn := &NodeConn{
		EventCh:     make(chan SSEEvent, 64),
		DoneCh:      make(chan struct{}),
		ConnectedAt: time.Now(),
	}
	h.connections[nodeID] = conn
	slog.Info("node SSE connected", "nodeId", nodeID)
	return conn
}

// Unregister removes a node's SSE connection and marks the node offline.
func (h *NodeHub) Unregister(nodeID uint) {
	h.mu.Lock()
	conn, ok := h.connections[nodeID]
	if ok {
		delete(h.connections, nodeID)
	}
	h.mu.Unlock()

	if ok {
		select {
		case <-conn.DoneCh:
			// already closed
		default:
			close(conn.DoneCh)
		}
	}

	// Mark node offline in DB
	if err := h.nodeRepo.Update(nodeID, map[string]any{
		"status": domain.NodeStatusOffline,
	}); err != nil {
		slog.Warn("failed to mark node offline on SSE disconnect", "nodeId", nodeID, "error", err)
	}

	slog.Info("node SSE disconnected", "nodeId", nodeID)
}

// Send pushes an event to a specific node. Returns false if node is not connected.
func (h *NodeHub) Send(nodeID uint, event SSEEvent) bool {
	h.mu.RLock()
	conn, ok := h.connections[nodeID]
	h.mu.RUnlock()

	if !ok {
		return false
	}

	select {
	case conn.EventCh <- event:
		return true
	case <-conn.DoneCh:
		return false
	default:
		// Channel full, log warning and drop
		slog.Warn("SSE event channel full, dropping event", "nodeId", nodeID, "event", event.Event)
		return false
	}
}

// SendCommand creates a command event from a domain.NodeCommand.
func (h *NodeHub) SendCommand(nodeID uint, cmd *domain.NodeCommand) bool {
	data, _ := json.Marshal(cmd.ToResponse())
	return h.Send(nodeID, SSEEvent{
		Event: "command",
		Data:  json.RawMessage(data),
	})
}

// Broadcast sends an event to multiple nodes.
func (h *NodeHub) Broadcast(nodeIDs []uint, event SSEEvent) {
	for _, nodeID := range nodeIDs {
		h.Send(nodeID, event)
	}
}

// IsOnline checks if a node has an active SSE connection.
func (h *NodeHub) IsOnline(nodeID uint) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.connections[nodeID]
	return ok
}
