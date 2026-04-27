package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/samber/do/v2"
)

// MCPRuntimeClient discovers and executes MCP tools for assistant runtime use.
type MCPRuntimeClient interface {
	DiscoverTools(ctx context.Context, server MCPServer) ([]MCPRuntimeTool, error)
	CallTool(ctx context.Context, server MCPServer, toolName string, args json.RawMessage) (json.RawMessage, error)
}

type MCPRuntimeTool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

type defaultMCPRuntimeClient struct {
	httpClient *http.Client
}

func NewDefaultMCPRuntimeClient(i do.Injector) (MCPRuntimeClient, error) {
	return &defaultMCPRuntimeClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *defaultMCPRuntimeClient) DiscoverTools(ctx context.Context, server MCPServer) ([]MCPRuntimeTool, error) {
	raw, err := c.callJSONRPC(ctx, server, "tools/list", nil)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
			Parameters  json.RawMessage `json:"parameters"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse MCP tools/list result: %w", err)
	}
	tools := make([]MCPRuntimeTool, 0, len(parsed.Tools))
	for _, t := range parsed.Tools {
		params := t.InputSchema
		if len(params) == 0 {
			params = t.Parameters
		}
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		if strings.TrimSpace(t.Name) == "" {
			continue
		}
		tools = append(tools, MCPRuntimeTool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		})
	}
	return tools, nil
}

func (c *defaultMCPRuntimeClient) CallTool(ctx context.Context, server MCPServer, toolName string, args json.RawMessage) (json.RawMessage, error) {
	var arguments any = map[string]any{}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &arguments)
	}
	return c.callJSONRPC(ctx, server, "tools/call", map[string]any{
		"name":      toolName,
		"arguments": arguments,
	})
}

func (c *defaultMCPRuntimeClient) callJSONRPC(ctx context.Context, server MCPServer, method string, params any) (json.RawMessage, error) {
	switch server.Transport {
	case MCPTransportSSE:
		return c.callHTTPJSONRPC(ctx, server.URL, method, params)
	case MCPTransportSTDIO:
		return callStdioJSONRPC(ctx, server, method, params)
	default:
		return nil, ErrInvalidTransport
	}
}

func (c *defaultMCPRuntimeClient) callHTTPJSONRPC(ctx context.Context, url, method string, params any) (json.RawMessage, error) {
	if url == "" {
		return nil, ErrSSERequiresURL
	}
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      time.Now().UnixNano(),
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("MCP HTTP %d: %s", resp.StatusCode, string(body))
	}
	return parseJSONRPCResult(body)
}

func callStdioJSONRPC(ctx context.Context, server MCPServer, method string, params any) (json.RawMessage, error) {
	if server.Command == "" {
		return nil, ErrSTDIORequiresCommand
	}
	var args []string
	if len(server.Args) > 0 {
		_ = json.Unmarshal(server.Args, &args)
	}
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      time.Now().UnixNano(),
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, server.Command, args...)
	cmd.Stdin = bytes.NewReader(append(reqBody, '\n'))
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseJSONRPCResult(out)
}

func parseJSONRPCResult(body []byte) (json.RawMessage, error) {
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  any             `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse JSON-RPC response: %w", err)
	}
	if envelope.Error != nil {
		raw, _ := json.Marshal(envelope.Error)
		return nil, fmt.Errorf("MCP JSON-RPC error: %s", string(raw))
	}
	if len(envelope.Result) == 0 {
		return json.RawMessage("{}"), nil
	}
	return envelope.Result, nil
}

type mcpToolOwner struct {
	server   MCPServer
	toolName string
}

type MCPToolRegistry struct {
	client MCPRuntimeClient
	tools  map[string]mcpToolOwner
}

func NewMCPToolRegistry(client MCPRuntimeClient) *MCPToolRegistry {
	return &MCPToolRegistry{client: client, tools: map[string]mcpToolOwner{}}
}

func (r *MCPToolRegistry) Register(exposedName string, server MCPServer, toolName string) {
	r.tools[exposedName] = mcpToolOwner{server: server, toolName: toolName}
}

func (r *MCPToolRegistry) HasTool(name string) bool {
	_, ok := r.tools[name]
	return ok
}

func (r *MCPToolRegistry) Execute(ctx context.Context, toolName string, _ uint, args json.RawMessage) (json.RawMessage, error) {
	owner, ok := r.tools[toolName]
	if !ok {
		return nil, fmt.Errorf("unknown MCP tool: %s", toolName)
	}
	if r.client == nil {
		return nil, fmt.Errorf("MCP runtime client unavailable")
	}
	return r.client.CallTool(ctx, owner.server, owner.toolName, args)
}

func (r *MCPToolRegistry) Len() int {
	return len(r.tools)
}
