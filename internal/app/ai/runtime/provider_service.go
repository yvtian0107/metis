package runtime

import (
	"errors"
	"fmt"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/pkg/crypto"
)

var (
	ErrProviderNotFound    = errors.New("provider not found")
	ErrProviderNameExists  = errors.New("provider name already exists")
	ErrInvalidProviderType = errors.New("invalid provider type")
)

type ProviderService struct {
	repo   *ProviderRepo
	encKey crypto.EncryptionKey
}

func NewProviderService(i do.Injector) (*ProviderService, error) {
	return &ProviderService{
		repo:   do.MustInvoke[*ProviderRepo](i),
		encKey: do.MustInvoke[crypto.EncryptionKey](i),
	}, nil
}

func (s *ProviderService) Create(name, providerType, baseURL, apiKey string) (*Provider, error) {
	if !ValidProviderTypes[providerType] {
		return nil, ErrInvalidProviderType
	}

	encrypted, err := crypto.Encrypt([]byte(apiKey), s.encKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt api key: %w", err)
	}

	p := &Provider{
		Name:            name,
		Type:            providerType,
		Protocol:        ProtocolForType(providerType),
		BaseURL:         baseURL,
		APIKeyEncrypted: encrypted,
		Status:          ProviderStatusInactive,
	}
	if err := s.repo.Create(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *ProviderService) Get(id uint) (*Provider, error) {
	p, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}
	return p, nil
}

func (s *ProviderService) Update(id uint, name, providerType, baseURL, apiKey string) (*Provider, error) {
	if !ValidProviderTypes[providerType] {
		return nil, ErrInvalidProviderType
	}

	p, err := s.repo.FindByID(id)
	if err != nil {
		return nil, ErrProviderNotFound
	}

	// Only reset status when connection-relevant fields change.
	connectionChanged := p.BaseURL != baseURL || apiKey != ""

	p.Name = name
	p.Type = providerType
	p.Protocol = ProtocolForType(providerType)
	p.BaseURL = baseURL
	if connectionChanged {
		p.Status = ProviderStatusInactive
	}

	if apiKey != "" {
		encrypted, err := crypto.Encrypt([]byte(apiKey), s.encKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt api key: %w", err)
		}
		p.APIKeyEncrypted = encrypted
	}

	if err := s.repo.Update(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *ProviderService) Delete(id uint) error {
	return s.repo.DeleteWithModels(id)
}

func (s *ProviderService) DecryptAPIKey(p *Provider) (string, error) {
	if len(p.APIKeyEncrypted) == 0 {
		return "", nil
	}
	plain, err := crypto.Decrypt(p.APIKeyEncrypted, s.encKey)
	if err != nil {
		return "", fmt.Errorf("decrypt api key: %w", err)
	}
	return string(plain), nil
}

func (s *ProviderService) MaskAPIKey(p *Provider) string {
	plain, err := s.DecryptAPIKey(p)
	if err != nil || len(plain) == 0 {
		return ""
	}
	if len(plain) <= 8 {
		return "****"
	}
	return plain[:3] + "****" + plain[len(plain)-4:]
}
