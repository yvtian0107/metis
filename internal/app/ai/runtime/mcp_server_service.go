package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/samber/do/v2"

	"metis/internal/pkg/crypto"
)

var (
	ErrMCPServerNotFound    = errors.New("mcp server not found")
	ErrInvalidTransport     = errors.New("invalid transport: must be sse or stdio")
	ErrSSERequiresURL       = errors.New("SSE transport requires url")
	ErrSTDIORequiresCommand = errors.New("STDIO transport requires command")
)

type MCPServerService struct {
	repo      *MCPServerRepo
	encKey    crypto.EncryptionKey
	mcpClient MCPRuntimeClient
}

func NewMCPServerService(i do.Injector) (*MCPServerService, error) {
	return &MCPServerService{
		repo:      do.MustInvoke[*MCPServerRepo](i),
		encKey:    do.MustInvoke[crypto.EncryptionKey](i),
		mcpClient: do.MustInvoke[MCPRuntimeClient](i),
	}, nil
}

func (s *MCPServerService) Create(m *MCPServer, authConfig string) error {
	if err := s.validateTransport(m); err != nil {
		return err
	}
	if authConfig != "" && m.AuthType != AuthTypeNone {
		encrypted, err := crypto.Encrypt([]byte(authConfig), s.encKey)
		if err != nil {
			return fmt.Errorf("encrypt auth config: %w", err)
		}
		m.AuthConfigEncrypted = encrypted
	}
	return s.repo.Create(m)
}

func (s *MCPServerService) Get(id uint) (*MCPServer, error) {
	m, err := s.repo.FindByID(id)
	if err != nil {
		return nil, ErrMCPServerNotFound
	}
	return m, nil
}

func (s *MCPServerService) Update(id uint, updates *MCPServer, authConfig string) (*MCPServer, error) {
	m, err := s.repo.FindByID(id)
	if err != nil {
		return nil, ErrMCPServerNotFound
	}

	m.Name = updates.Name
	m.Description = updates.Description
	m.Transport = updates.Transport
	m.URL = updates.URL
	m.Command = updates.Command
	m.Args = updates.Args
	m.Env = updates.Env
	m.AuthType = updates.AuthType
	m.IsActive = updates.IsActive

	if err := s.validateTransport(m); err != nil {
		return nil, err
	}

	if authConfig != "" {
		encrypted, err := crypto.Encrypt([]byte(authConfig), s.encKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt auth config: %w", err)
		}
		m.AuthConfigEncrypted = encrypted
	} else if m.AuthType == AuthTypeNone {
		m.AuthConfigEncrypted = nil
	}

	if err := s.repo.Update(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *MCPServerService) Delete(id uint) error {
	return s.repo.Delete(id)
}

func (s *MCPServerService) List(params MCPServerListParams) ([]MCPServer, int64, error) {
	return s.repo.List(params)
}

func (s *MCPServerService) MaskAuthConfig(m *MCPServer) string {
	if len(m.AuthConfigEncrypted) == 0 || m.AuthType == AuthTypeNone {
		return ""
	}
	plain, err := crypto.Decrypt(m.AuthConfigEncrypted, s.encKey)
	if err != nil {
		return ""
	}
	// Try to parse as JSON and mask values
	var config map[string]string
	if json.Unmarshal(plain, &config) == nil {
		for k := range config {
			v := config[k]
			if len(v) > 8 {
				config[k] = v[:3] + "****" + v[len(v)-4:]
			} else {
				config[k] = "****"
			}
		}
		masked, _ := json.Marshal(config)
		return string(masked)
	}
	return "****"
}

func (s *MCPServerService) DecryptAuthConfig(m *MCPServer) (string, error) {
	if len(m.AuthConfigEncrypted) == 0 {
		return "", nil
	}
	plain, err := crypto.Decrypt(m.AuthConfigEncrypted, s.encKey)
	if err != nil {
		return "", fmt.Errorf("decrypt auth config: %w", err)
	}
	return string(plain), nil
}

func (s *MCPServerService) TestConnection(ctx context.Context, id uint) ([]MCPRuntimeTool, error) {
	m, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if m.Transport != MCPTransportSSE {
		return nil, fmt.Errorf("test connection is only available for SSE transport")
	}
	if s.mcpClient == nil {
		return nil, fmt.Errorf("MCP runtime client unavailable")
	}
	return s.mcpClient.DiscoverTools(ctx, *m)
}

func (s *MCPServerService) validateTransport(m *MCPServer) error {
	switch m.Transport {
	case MCPTransportSSE:
		if m.URL == "" {
			return ErrSSERequiresURL
		}
	case MCPTransportSTDIO:
		if m.Command == "" {
			return ErrSTDIORequiresCommand
		}
	default:
		return ErrInvalidTransport
	}
	return nil
}
