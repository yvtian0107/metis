package locales

import (
	"embed"
	"encoding/json"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed zh-CN.json en.json
var localeFS embed.FS

// Service provides message localisation backed by go-i18n.
type Service struct {
	bundle *i18n.Bundle
}

// New creates a Service pre-loaded with the embedded kernel locale files.
func New() (*Service, error) {
	bundle := i18n.NewBundle(language.Chinese)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	for _, name := range []string{"zh-CN.json", "en.json"} {
		data, err := localeFS.ReadFile(name)
		if err != nil {
			return nil, err
		}
		if _, err := bundle.ParseMessageFileBytes(data, name); err != nil {
			return nil, err
		}
	}

	return &Service{bundle: bundle}, nil
}

// LoadAppLocales loads additional locale JSON files from an app's embed.FS.
// The FS should contain files named like "zh-CN.json", "en.json", etc.
func (s *Service) LoadAppLocales(fs embed.FS) error {
	entries, err := fs.ReadDir(".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, readErr := fs.ReadFile(entry.Name())
		if readErr != nil {
			return readErr
		}
		if _, parseErr := s.bundle.ParseMessageFileBytes(data, entry.Name()); parseErr != nil {
			return parseErr
		}
	}
	return nil
}

// T translates a message ID for the given locale.
// If the locale or message is not found, the ID itself is returned.
func (s *Service) T(locale, messageID string) string {
	localizer := i18n.NewLocalizer(s.bundle, locale)
	msg, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: messageID})
	if err != nil {
		return messageID
	}
	return msg
}

// TWithData translates a message ID with template data.
func (s *Service) TWithData(locale, messageID string, data map[string]interface{}) string {
	localizer := i18n.NewLocalizer(s.bundle, locale)
	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: data,
	})
	if err != nil {
		return messageID
	}
	return msg
}
