package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync/atomic"
	"syscall"
	"time"
)

// LocalCodingExecutor spawns coding tools (claude-code, codex, etc.) as subprocesses.
type LocalCodingExecutor struct{}

func NewLocalCodingExecutor() *LocalCodingExecutor {
	return &LocalCodingExecutor{}
}

func (e *LocalCodingExecutor) Execute(ctx context.Context, req ExecuteRequest) (<-chan Event, error) {
	ch := make(chan Event, 64)

	cfg := req.AgentConfig
	cmdName, args, env := buildRuntimeCommand(cfg.Runtime, cfg.RuntimeConfig, cfg.Workspace, cfg.Instructions)
	if cmdName == "" {
		return nil, fmt.Errorf("unsupported coding runtime: %s", cfg.Runtime)
	}

	// Get the user's last message as input
	var userInput string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			userInput = req.Messages[i].Content
			break
		}
	}

	args = appendInputArgs(cfg.Runtime, args, userInput)

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

		emit(Event{Type: EventTypeLLMStart, Turn: 1, Model: ""})

		cmd := exec.CommandContext(ctx, cmdName, args...)
		if cfg.Workspace != "" {
			cmd.Dir = cfg.Workspace
		}
		cmd.Env = append(cmd.Environ(), env...)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			emit(Event{Type: EventTypeError, Message: fmt.Sprintf("stdout pipe: %v", err)})
			return
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			emit(Event{Type: EventTypeError, Message: fmt.Sprintf("stderr pipe: %v", err)})
			return
		}

		if err := cmd.Start(); err != nil {
			emit(Event{Type: EventTypeError, Message: fmt.Sprintf("start failed: %v", err)})
			return
		}

		// Stream stdout as content_delta events
		go func() {
			scanner := bufio.NewScanner(stdout)
			scanner.Buffer(make([]byte, 64*1024), 1024*1024)
			for scanner.Scan() {
				emit(Event{Type: EventTypeContentDelta, Text: scanner.Text() + "\n"})
			}
		}()

		// Collect stderr
		var stderrBuf []byte
		go func() {
			scanner := bufio.NewScanner(stderr)
			scanner.Buffer(make([]byte, 64*1024), 1024*1024)
			for scanner.Scan() {
				stderrBuf = append(stderrBuf, scanner.Bytes()...)
				stderrBuf = append(stderrBuf, '\n')
			}
		}()

		err = cmd.Wait()

		if ctx.Err() != nil {
			emit(Event{Type: EventTypeCancelled, Message: "cancelled by user"})
			return
		}

		if err != nil {
			errMsg := err.Error()
			if len(stderrBuf) > 0 {
				errMsg = string(stderrBuf)
			}
			emit(Event{Type: EventTypeError, Message: errMsg})
			return
		}

		emit(Event{Type: EventTypeDone, TotalTurns: 1})
	}()

	return ch, nil
}

// buildRuntimeCommand returns the command, args, and env for a given runtime.
func buildRuntimeCommand(runtime string, runtimeConfig json.RawMessage, workspace, instructions string) (string, []string, []string) {
	var config map[string]any
	if len(runtimeConfig) > 0 {
		_ = json.Unmarshal(runtimeConfig, &config)
	}

	switch runtime {
	case AgentRuntimeClaudeCode:
		args := []string{"--print"}
		if instructions != "" {
			args = append(args, "--system-prompt", instructions)
		}
		var env []string
		if apiKey, ok := config["api_key"].(string); ok && apiKey != "" {
			env = append(env, "ANTHROPIC_API_KEY="+apiKey)
		}
		return "claude", args, env

	case AgentRuntimeCodex:
		args := []string{}
		var env []string
		if apiKey, ok := config["api_key"].(string); ok && apiKey != "" {
			env = append(env, "OPENAI_API_KEY="+apiKey)
		}
		return "codex", args, env

	case AgentRuntimeOpenCode:
		return "opencode", []string{}, nil

	case AgentRuntimeAider:
		args := []string{"--no-auto-commits", "--yes"}
		if instructions != "" {
			args = append(args, "--message-file", "-")
		}
		return "aider", args, nil

	default:
		return "", nil, nil
	}
}

// appendInputArgs adds the user message as input to the command args.
func appendInputArgs(runtime string, args []string, input string) []string {
	switch runtime {
	case AgentRuntimeClaudeCode:
		return append(args, input)
	case AgentRuntimeCodex:
		return append(args, input)
	case AgentRuntimeOpenCode:
		return append(args, input)
	case AgentRuntimeAider:
		return append(args, "--message", input)
	default:
		return args
	}
}

// GracefulStop sends SIGTERM then SIGKILL after timeout.
func GracefulStop(cmd *exec.Cmd, timeout time.Duration) {
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
	}
}
