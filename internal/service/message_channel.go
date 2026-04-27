package service

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/channel"
	"metis/internal/model"
	"metis/internal/repository"
)

var (
	ErrChannelNotFound = errors.New("error.channel.not_found")
	ErrChannelDisabled = errors.New("message channel is disabled")
)

type MessageChannelService struct {
	repo           *repository.MessageChannelRepo
	DriverResolver func(string) (channel.Driver, error)
}

func NewMessageChannel(i do.Injector) (*MessageChannelService, error) {
	repo := do.MustInvoke[*repository.MessageChannelRepo](i)
	return &MessageChannelService{
		repo:           repo,
		DriverResolver: channel.GetDriver,
	}, nil
}

func (s *MessageChannelService) Create(name, channelType, config string) (*model.MessageChannel, error) {
	if _, err := s.DriverResolver(channelType); err != nil {
		return nil, err
	}

	ch := &model.MessageChannel{
		Name:    name,
		Type:    channelType,
		Config:  config,
		Enabled: true,
	}
	if err := s.repo.Create(ch); err != nil {
		return nil, err
	}
	return ch, nil
}

func (s *MessageChannelService) Get(id uint) (*model.MessageChannelResponse, error) {
	ch, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChannelNotFound
		}
		return nil, err
	}
	resp := ch.ToResponse()
	resp.Config = repository.MaskConfig(resp.Config)
	return &resp, nil
}

func (s *MessageChannelService) List(params repository.ListParams) ([]model.MessageChannelResponse, int64, error) {
	items, total, err := s.repo.List(params)
	if err != nil {
		return nil, 0, err
	}

	results := make([]model.MessageChannelResponse, len(items))
	for i, item := range items {
		resp := item.ToResponse()
		resp.Config = repository.MaskConfig(resp.Config)
		results[i] = resp
	}
	return results, total, nil
}

func (s *MessageChannelService) Update(id uint, name, config string) (*model.MessageChannelResponse, error) {
	ch, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChannelNotFound
		}
		return nil, err
	}

	ch.Name = name

	// Handle masked password preservation
	var newCfg map[string]any
	if err := json.Unmarshal([]byte(config), &newCfg); err != nil {
		return nil, fmt.Errorf("invalid config JSON: %w", err)
	}
	if pwd, ok := newCfg["password"]; ok {
		if pwd == "******" {
			// Preserve original password
			var oldCfg map[string]any
			if err := json.Unmarshal([]byte(ch.Config), &oldCfg); err == nil {
				newCfg["password"] = oldCfg["password"]
			}
			merged, _ := json.Marshal(newCfg)
			config = string(merged)
		}
	}

	ch.Config = config
	if err := s.repo.Update(ch); err != nil {
		return nil, err
	}

	resp := ch.ToResponse()
	resp.Config = repository.MaskConfig(resp.Config)
	return &resp, nil
}

func (s *MessageChannelService) Delete(id uint) error {
	if err := s.repo.Delete(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrChannelNotFound
		}
		return err
	}
	return nil
}

func (s *MessageChannelService) ToggleEnabled(id uint) (*model.MessageChannelResponse, error) {
	ch, err := s.repo.ToggleEnabled(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChannelNotFound
		}
		return nil, err
	}
	resp := ch.ToResponse()
	resp.Config = repository.MaskConfig(resp.Config)
	return &resp, nil
}

func (s *MessageChannelService) TestChannel(id uint) error {
	ch, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrChannelNotFound
		}
		return err
	}

	driver, err := s.DriverResolver(ch.Type)
	if err != nil {
		return err
	}

	var cfg map[string]any
	if err := json.Unmarshal([]byte(ch.Config), &cfg); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	return driver.Test(cfg)
}

func (s *MessageChannelService) SendTest(id uint, to []string, subject, body string) error {
	return s.send(id, to, subject, body, false)
}

func (s *MessageChannelService) Send(id uint, to []string, subject, body string) error {
	return s.send(id, to, subject, body, true)
}

func (s *MessageChannelService) send(id uint, to []string, subject, body string, requireEnabled bool) error {
	ch, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrChannelNotFound
		}
		return err
	}
	if requireEnabled && !ch.Enabled {
		return ErrChannelDisabled
	}

	driver, err := s.DriverResolver(ch.Type)
	if err != nil {
		return err
	}

	var cfg map[string]any
	if err := json.Unmarshal([]byte(ch.Config), &cfg); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	return driver.Send(cfg, channel.Payload{
		To:      to,
		Subject: subject,
		Body:    body,
	})
}
