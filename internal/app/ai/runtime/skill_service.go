package runtime

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/model"
	"metis/internal/pkg/crypto"
)

var (
	ErrSkillNotFound       = errors.New("skill not found")
	ErrInvalidSkillPackage = errors.New("invalid skill package: manifest.json is required")
	ErrInvalidManifest     = errors.New("invalid manifest.json format")
	ErrInvalidGitHubURL    = errors.New("invalid GitHub URL")
	ErrNotImplemented      = errors.New("GitHub skill import is not yet implemented")
)

type SkillService struct {
	repo   *SkillRepo
	encKey crypto.EncryptionKey
}

func NewSkillService(i do.Injector) (*SkillService, error) {
	return &SkillService{
		repo:   do.MustInvoke[*SkillRepo](i),
		encKey: do.MustInvoke[crypto.EncryptionKey](i),
	}, nil
}

// SkillManifest is the expected manifest.json structure.
type SkillManifest struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Author      string `json:"author"`
}

func (s *SkillService) InstallFromUpload(data io.Reader) (*Skill, error) {
	manifest, instructions, toolsSchema, err := extractSkillPackage(data)
	if err != nil {
		return nil, err
	}

	manifestJSON, _ := json.Marshal(manifest)
	skill := &Skill{
		Name:         manifest.Name,
		DisplayName:  manifest.DisplayName,
		Description:  manifest.Description,
		SourceType:   SkillSourceUpload,
		Manifest:     model.JSONText(manifestJSON),
		Instructions: instructions,
		ToolsSchema:  model.JSONText(toolsSchema),
		AuthType:     AuthTypeNone,
		IsActive:     true,
	}
	if skill.DisplayName == "" {
		skill.DisplayName = skill.Name
	}

	if err := s.repo.Create(skill); err != nil {
		return nil, err
	}
	return skill, nil
}

type ImportGitHubReq struct {
	URL string `json:"url" binding:"required"`
}

func (s *SkillService) InstallFromGitHub(sourceURL string) (*Skill, error) {
	if !strings.Contains(sourceURL, "github.com") {
		return nil, ErrInvalidGitHubURL
	}
	return nil, ErrNotImplemented
}

func (s *SkillService) Get(id uint) (*Skill, error) {
	skill, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSkillNotFound
		}
		return nil, err
	}
	return skill, nil
}

func (s *SkillService) Update(id uint, authType, authConfig string) (*Skill, error) {
	skill, err := s.repo.FindByID(id)
	if err != nil {
		return nil, ErrSkillNotFound
	}

	skill.AuthType = authType
	if authConfig != "" && authType != AuthTypeNone {
		encrypted, err := crypto.Encrypt([]byte(authConfig), s.encKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt auth config: %w", err)
		}
		skill.AuthConfigEncrypted = encrypted
	} else if authType == AuthTypeNone {
		skill.AuthConfigEncrypted = nil
	}

	if err := s.repo.Update(skill); err != nil {
		return nil, err
	}
	return skill, nil
}

func (s *SkillService) ToggleActive(id uint, isActive bool) (*Skill, error) {
	skill, err := s.repo.FindByID(id)
	if err != nil {
		return nil, ErrSkillNotFound
	}
	skill.IsActive = isActive
	if err := s.repo.Update(skill); err != nil {
		return nil, err
	}
	return skill, nil
}

func (s *SkillService) Delete(id uint) error {
	return s.repo.Delete(id)
}

func (s *SkillService) List(params SkillListParams) ([]Skill, int64, error) {
	return s.repo.List(params)
}

// GetPackage returns the skill's full content for Agent download.
func (s *SkillService) GetPackage(id uint) (map[string]any, error) {
	skill, err := s.repo.FindByID(id)
	if err != nil {
		return nil, ErrSkillNotFound
	}

	pkg := map[string]any{
		"id":           skill.ID,
		"name":         skill.Name,
		"instructions": skill.Instructions,
		"toolsSchema":  skill.ToolsSchema,
	}

	if len(skill.AuthConfigEncrypted) > 0 && skill.AuthType != AuthTypeNone {
		plain, err := crypto.Decrypt(skill.AuthConfigEncrypted, s.encKey)
		if err == nil {
			pkg["auth"] = json.RawMessage(plain)
		}
	}

	return pkg, nil
}

// Checksum returns a SHA256 hash of the skill's content for cache validation.
func (s *SkillService) Checksum(skill *Skill) string {
	h := sha256.New()
	h.Write([]byte(skill.Instructions))
	h.Write(skill.ToolsSchema)
	h.Write(skill.Manifest)
	return hex.EncodeToString(h.Sum(nil))
}

// extractSkillPackage extracts manifest, instructions, and tools from a tar.gz reader.
func extractSkillPackage(r io.Reader) (*SkillManifest, string, json.RawMessage, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, "", nil, fmt.Errorf("invalid gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var manifest *SkillManifest
	var instructions string
	var tools []json.RawMessage

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", nil, fmt.Errorf("read tar: %w", err)
		}

		// Normalize path: strip leading directory
		name := strings.TrimPrefix(hdr.Name, "./")
		parts := strings.SplitN(name, "/", 2)
		if len(parts) == 2 {
			name = parts[1] // strip top-level dir
		}

		switch {
		case name == "manifest.json":
			data, _ := io.ReadAll(io.LimitReader(tr, 1<<20)) // 1MB limit
			var m SkillManifest
			if err := json.Unmarshal(data, &m); err != nil {
				return nil, "", nil, ErrInvalidManifest
			}
			if m.Name == "" {
				return nil, "", nil, ErrInvalidManifest
			}
			manifest = &m

		case name == "instructions.md":
			data, _ := io.ReadAll(io.LimitReader(tr, 1<<20))
			instructions = string(data)

		case strings.HasPrefix(name, "tools/") && strings.HasSuffix(name, ".json"):
			data, _ := io.ReadAll(io.LimitReader(tr, 1<<20))
			var tool json.RawMessage
			if json.Unmarshal(data, &tool) == nil {
				tools = append(tools, tool)
			}
		}
	}

	if manifest == nil {
		return nil, "", nil, ErrInvalidSkillPackage
	}

	var toolsSchema json.RawMessage
	if len(tools) > 0 {
		toolsSchema, _ = json.Marshal(tools)
	}

	return manifest, instructions, toolsSchema, nil
}

// extractRepoSkillName extracts a skill name from a GitHub URL.
func extractRepoSkillName(url string) string {
	parts := strings.Split(strings.TrimRight(url, "/"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "unknown-skill"
}
