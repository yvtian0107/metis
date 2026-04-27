package runtime

import (
	"context"
	"fmt"
	"sync/atomic"
)

// RemoteCodingExecutor delegates execution to a remote Node via Sidecar SSE.
// It sends a run command via NodeHub and receives events from the sidecar.
type RemoteCodingExecutor struct {
	// nodeHub will be injected when the full Node integration is wired
	// For now, this is a skeleton that will be connected to the existing NodeHub.
}

func NewRemoteCodingExecutor() *RemoteCodingExecutor {
	return &RemoteCodingExecutor{}
}

func (e *RemoteCodingExecutor) Execute(ctx context.Context, req ExecuteRequest) (<-chan Event, error) {
	ch := make(chan Event, 64)

	cfg := req.AgentConfig
	if cfg.NodeID == nil {
		return nil, fmt.Errorf("node_id is required for remote coding execution")
	}

	go func() {
		defer close(ch)

		var seq atomic.Int32
		emit := func(evt Event) {
			evt.Sequence = int(seq.Add(1))
			select {
			case ch <- evt:
			case <-ctx.Done():
			}
		}

		// TODO: Check node status via NodeRepo
		// TODO: Send run command via NodeHub SSE
		// TODO: Create a channel to receive events from sidecar POST endpoint
		// TODO: Forward events from sidecar to this channel

		// For now, return an informational error indicating this is not yet fully wired
		emit(Event{
			Type:    EventTypeError,
			Message: fmt.Sprintf("remote coding execution on node %d is not yet implemented — use local exec mode", *cfg.NodeID),
		})
	}()

	return ch, nil
}
