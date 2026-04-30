package domain

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// ServiceActionHTTPConfig is the normalized runtime configuration for http actions.
type ServiceActionHTTPConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    json.RawMessage   `json:"body,omitempty"`
	Timeout int               `json:"timeout"`
	Retries int               `json:"retries"`
}

// NormalizeServiceActionConfig validates and fills defaults for persisted action configs.
func NormalizeServiceActionConfig(actionType string, raw JSONField) (JSONField, error) {
	if actionType != "http" {
		return nil, fmt.Errorf("unsupported action type %q", actionType)
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, fmt.Errorf("configJson is required")
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("configJson must be a JSON object: %w", err)
	}
	if fields == nil {
		return nil, fmt.Errorf("configJson must be a JSON object")
	}

	var cfg ServiceActionHTTPConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("invalid configJson: %w", err)
	}
	parsed, err := url.Parse(cfg.URL)
	if err != nil || cfg.URL == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("configJson.url must be http/https URL with host")
	}

	if _, ok := fields["method"]; !ok || strings.TrimSpace(cfg.Method) == "" {
		cfg.Method = "POST"
	} else {
		cfg.Method = strings.ToUpper(strings.TrimSpace(cfg.Method))
	}
	if !allowedHTTPMethod(cfg.Method) {
		return nil, fmt.Errorf("configJson.method is not allowed")
	}

	if _, ok := fields["timeout"]; !ok {
		cfg.Timeout = 30
	}
	if cfg.Timeout < 1 || cfg.Timeout > 120 {
		return nil, fmt.Errorf("configJson.timeout must be between 1 and 120")
	}

	if _, ok := fields["retries"]; !ok {
		cfg.Retries = 3
	}
	if cfg.Retries < 0 || cfg.Retries > 5 {
		return nil, fmt.Errorf("configJson.retries must be between 0 and 5")
	}

	for key, value := range cfg.Headers {
		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("configJson.headers contains empty key")
		}
		if strings.ContainsAny(key, "\r\n") || strings.ContainsAny(value, "\r\n") {
			return nil, fmt.Errorf("configJson.headers must not contain CR/LF")
		}
	}

	normalized, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("normalize configJson: %w", err)
	}
	return JSONField(normalized), nil
}

func allowedHTTPMethod(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}
