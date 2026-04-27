package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type skillEndpointSpec struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
}

type skillToolSpec struct {
	Name             string             `json:"name"`
	Description      string             `json:"description"`
	Parameters       json.RawMessage    `json:"parameters"`
	ParametersSchema json.RawMessage    `json:"parametersSchema"`
	Endpoint         *skillEndpointSpec `json:"endpoint"`
	URL              string             `json:"url"`
	Method           string             `json:"method"`
	Headers          map[string]string  `json:"headers"`
}

type skillToolOwner struct {
	skill    Skill
	toolName string
	endpoint skillEndpointSpec
}

type SkillToolRegistry struct {
	httpClient *http.Client
	tools      map[string]skillToolOwner
}

func NewSkillToolRegistry() *SkillToolRegistry {
	return &SkillToolRegistry{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		tools:      map[string]skillToolOwner{},
	}
}

func (r *SkillToolRegistry) Register(exposedName string, skill Skill, spec skillToolSpec) {
	endpoint := endpointFromSkillToolSpec(spec)
	r.tools[exposedName] = skillToolOwner{
		skill:    skill,
		toolName: spec.Name,
		endpoint: endpoint,
	}
}

func (r *SkillToolRegistry) HasTool(name string) bool {
	_, ok := r.tools[name]
	return ok
}

func (r *SkillToolRegistry) Execute(ctx context.Context, toolName string, _ uint, args json.RawMessage) (json.RawMessage, error) {
	owner, ok := r.tools[toolName]
	if !ok {
		return nil, fmt.Errorf("unknown skill tool: %s", toolName)
	}
	if owner.endpoint.URL == "" {
		return nil, fmt.Errorf("skill tool %s has no executable endpoint", toolName)
	}
	method := strings.ToUpper(owner.endpoint.Method)
	if method == "" {
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, owner.endpoint.URL, bytes.NewReader(args))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range owner.endpoint.Headers {
		req.Header.Set(k, v)
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("skill tool HTTP %d: %s", resp.StatusCode, string(body))
	}
	if len(body) == 0 {
		return json.RawMessage("{}"), nil
	}
	return json.RawMessage(body), nil
}

func (r *SkillToolRegistry) Len() int {
	return len(r.tools)
}

func parseSkillTools(skill Skill) []skillToolSpec {
	if len(skill.ToolsSchema) == 0 {
		return nil
	}
	var specs []skillToolSpec
	if err := json.Unmarshal(skill.ToolsSchema, &specs); err != nil {
		slog.Warn("invalid skill tools schema", "skillID", skill.ID, "skill", skill.Name, "error", err)
		return nil
	}
	return specs
}

func endpointFromSkillToolSpec(spec skillToolSpec) skillEndpointSpec {
	if spec.Endpoint != nil {
		return *spec.Endpoint
	}
	return skillEndpointSpec{
		URL:     spec.URL,
		Method:  spec.Method,
		Headers: spec.Headers,
	}
}

func skillToolParameters(spec skillToolSpec) json.RawMessage {
	if len(spec.Parameters) > 0 {
		return spec.Parameters
	}
	if len(spec.ParametersSchema) > 0 {
		return spec.ParametersSchema
	}
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
