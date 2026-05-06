//go:build dev

package main

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"metis/internal/config"
	"metis/internal/model"
	"metis/internal/pkg/crypto"
)

const (
	devAIConfigPath = ".env.dev"

	devProviderTypeOpenAI    = "openai"
	devProviderTypeAnthropic = "anthropic"
	devProviderTypeOllama    = "ollama"

	devProviderStatusActive = "active"
	devModelTypeLLM         = "llm"
	devModelStatusActive    = "active"
)

type devAIConfig struct {
	ProviderName string
	ProviderType string
	BaseURL      string
	APIKey       string
	ModelID      string
}

func runDevBootstrap(db *gorm.DB, cfg *config.MetisConfig, envPath string) error {
	devCfg, ok, err := loadDevAIConfig(envPath)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if cfg == nil || cfg.SecretKey == "" {
		return fmt.Errorf("%s requires config.yml secret_key for API key encryption", envPath)
	}
	if !db.Migrator().HasTable("ai_providers") || !db.Migrator().HasTable("ai_models") || !db.Migrator().HasTable("ai_agents") {
		return fmt.Errorf("%s is present, but AI tables are not available in this build", envPath)
	}

	modelID, err := upsertDevAIProviderAndModel(db, cfg, devCfg)
	if err != nil {
		return err
	}
	if err := bindDevITSMEngine(db, modelID); err != nil {
		return err
	}
	slog.Info("dev bootstrap: AI provider, model, and ITSM engine initialized",
		"provider", devCfg.ProviderName,
		"model", devCfg.ModelID,
	)
	return nil
}

func loadDevAIConfig(path string) (devAIConfig, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return devAIConfig{}, false, nil
		}
		return devAIConfig{}, false, fmt.Errorf("read %s: %w", path, err)
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return devAIConfig{}, true, fmt.Errorf("%s contains invalid line %q", path, line)
		}
		values[strings.TrimSpace(key)] = trimEnvValue(value)
	}
	if err := scanner.Err(); err != nil {
		return devAIConfig{}, true, fmt.Errorf("scan %s: %w", path, err)
	}

	cfg := devAIConfig{
		ProviderName: values["METIS_DEV_AI_PROVIDER_NAME"],
		ProviderType: valueOrDefault(values["METIS_DEV_AI_PROVIDER_TYPE"], devProviderTypeOpenAI),
		BaseURL:      values["METIS_DEV_AI_BASE_URL"],
		APIKey:       values["METIS_DEV_AI_API_KEY"],
		ModelID:      valueOrDefault(values["METIS_DEV_AI_MODEL"], "gpt-4o"),
	}
	var missing []string
	if cfg.ProviderName == "" {
		missing = append(missing, "METIS_DEV_AI_PROVIDER_NAME")
	}
	if cfg.BaseURL == "" {
		missing = append(missing, "METIS_DEV_AI_BASE_URL")
	}
	if cfg.APIKey == "" {
		missing = append(missing, "METIS_DEV_AI_API_KEY")
	}
	if len(missing) > 0 {
		return devAIConfig{}, true, fmt.Errorf("%s missing required values: %s", path, strings.Join(missing, ", "))
	}
	switch cfg.ProviderType {
	case devProviderTypeOpenAI, devProviderTypeAnthropic, devProviderTypeOllama:
	default:
		return devAIConfig{}, true, fmt.Errorf("%s has unsupported METIS_DEV_AI_PROVIDER_TYPE %q", path, cfg.ProviderType)
	}
	return cfg, true, nil
}

func trimEnvValue(value string) string {
	v := strings.TrimSpace(value)
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}

func valueOrDefault(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func upsertDevAIProviderAndModel(db *gorm.DB, cfg *config.MetisConfig, devCfg devAIConfig) (uint, error) {
	var modelID uint
	err := db.Transaction(func(tx *gorm.DB) error {
		key := crypto.EncryptionKey(crypto.DeriveKey(cfg.SecretKey))
		encrypted, err := crypto.Encrypt([]byte(devCfg.APIKey), key)
		if err != nil {
			return fmt.Errorf("encrypt dev AI API key: %w", err)
		}

		now := time.Now()
		protocol := "openai"
		if devCfg.ProviderType == devProviderTypeAnthropic {
			protocol = "anthropic"
		}

		var provider struct{ ID uint }
		providerUpdates := map[string]any{
			"name":              devCfg.ProviderName,
			"type":              devCfg.ProviderType,
			"protocol":          protocol,
			"base_url":          devCfg.BaseURL,
			"api_key_encrypted": encrypted,
			"status":            devProviderStatusActive,
			"health_checked_at": now,
			"updated_at":        now,
		}
		if err := tx.Table("ai_providers").Where("name = ?", devCfg.ProviderName).Select("id").First(&provider).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("load dev AI provider: %w", err)
			}
			providerUpdates["created_at"] = now
			if err := tx.Table("ai_providers").Create(providerUpdates).Error; err != nil {
				return fmt.Errorf("create dev AI provider: %w", err)
			}
			if err := tx.Table("ai_providers").Where("name = ?", devCfg.ProviderName).Select("id").First(&provider).Error; err != nil {
				return fmt.Errorf("reload dev AI provider: %w", err)
			}
		} else if err := tx.Table("ai_providers").Where("id = ?", provider.ID).Updates(providerUpdates).Error; err != nil {
			return fmt.Errorf("update dev AI provider: %w", err)
		}

		if err := tx.Table("ai_models").
			Where("provider_id = ? AND type = ? AND is_default = ?", provider.ID, devModelTypeLLM, true).
			Update("is_default", false).Error; err != nil {
			return fmt.Errorf("clear dev AI default model: %w", err)
		}

		modelUpdates := map[string]any{
			"model_id":     devCfg.ModelID,
			"display_name": devCfg.ModelID,
			"provider_id":  provider.ID,
			"type":         devModelTypeLLM,
			"capabilities": `["tool_use"]`,
			"is_default":   true,
			"status":       devModelStatusActive,
			"updated_at":   now,
		}
		var row struct{ ID uint }
		if err := tx.Table("ai_models").Where("provider_id = ? AND model_id = ?", provider.ID, devCfg.ModelID).Select("id").First(&row).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("load dev AI model: %w", err)
			}
			modelUpdates["created_at"] = now
			if err := tx.Table("ai_models").Create(modelUpdates).Error; err != nil {
				return fmt.Errorf("create dev AI model: %w", err)
			}
			if err := tx.Table("ai_models").Where("provider_id = ? AND model_id = ?", provider.ID, devCfg.ModelID).Select("id").First(&row).Error; err != nil {
				return fmt.Errorf("reload dev AI model: %w", err)
			}
		} else if err := tx.Table("ai_models").Where("id = ?", row.ID).Updates(modelUpdates).Error; err != nil {
			return fmt.Errorf("update dev AI model: %w", err)
		}
		modelID = row.ID
		return nil
	})
	return modelID, err
}

func bindDevITSMEngine(db *gorm.DB, modelID uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		agentIDs := map[string]uint{}
		for _, code := range []string{"itsm.servicedesk", "itsm.decision", "itsm.sla_assurance"} {
			var row struct{ ID uint }
			if err := tx.Table("ai_agents").Where("code = ?", code).Select("id").First(&row).Error; err != nil {
				return fmt.Errorf("load ITSM preset agent %s: %w", code, err)
			}
			agentIDs[code] = row.ID
			if err := tx.Table("ai_agents").Where("id = ?", row.ID).Updates(map[string]any{
				"model_id":   modelID,
				"is_active":  true,
				"updated_at": time.Now(),
			}).Error; err != nil {
				return fmt.Errorf("bind model to ITSM agent %s: %w", code, err)
			}
		}

		values := map[string]string{
			"itsm.smart_ticket.intake.agent_id":                 strconv.FormatUint(uint64(agentIDs["itsm.servicedesk"]), 10),
			"itsm.smart_ticket.decision.agent_id":               strconv.FormatUint(uint64(agentIDs["itsm.decision"]), 10),
			"itsm.smart_ticket.sla_assurance.agent_id":          strconv.FormatUint(uint64(agentIDs["itsm.sla_assurance"]), 10),
			"itsm.smart_ticket.decision.mode":                   "direct_first",
			"itsm.smart_ticket.service_matcher.model_id":        strconv.FormatUint(uint64(modelID), 10),
			"itsm.smart_ticket.service_matcher.temperature":     "0.2",
			"itsm.smart_ticket.service_matcher.max_tokens":      "1024",
			"itsm.smart_ticket.service_matcher.timeout_seconds": "30",
			"itsm.smart_ticket.path.model_id":                   strconv.FormatUint(uint64(modelID), 10),
			"itsm.smart_ticket.path.temperature":                "0.3",
			"itsm.smart_ticket.path.max_retries":                "1",
			"itsm.smart_ticket.path.timeout_seconds":            "60",
			"itsm.smart_ticket.session_title.model_id":          strconv.FormatUint(uint64(modelID), 10),
			"itsm.smart_ticket.session_title.temperature":       "0.3",
			"itsm.smart_ticket.session_title.max_retries":       "1",
			"itsm.smart_ticket.session_title.timeout_seconds":   "60",
			"itsm.smart_ticket.publish_health.model_id":         strconv.FormatUint(uint64(modelID), 10),
			"itsm.smart_ticket.publish_health.temperature":      "0.3",
			"itsm.smart_ticket.publish_health.max_retries":      "1",
			"itsm.smart_ticket.publish_health.timeout_seconds":  "60",
		}
		for key, value := range values {
			if err := upsertSystemConfig(tx, key, value); err != nil {
				return err
			}
		}
		return nil
	})
}

func upsertSystemConfig(tx *gorm.DB, key string, value string) error {
	cfg := model.SystemConfig{Key: key, Value: value}
	var existing model.SystemConfig
	if err := tx.Where("\"key\" = ?", key).First(&existing).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("load system config %s: %w", key, err)
		}
		if err := tx.Create(&cfg).Error; err != nil {
			return fmt.Errorf("create system config %s: %w", key, err)
		}
		return nil
	}
	existing.Value = value
	if err := tx.Save(&existing).Error; err != nil {
		return fmt.Errorf("update system config %s: %w", key, err)
	}
	return nil
}
