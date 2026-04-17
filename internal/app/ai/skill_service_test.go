package ai

import (
	"testing"
)

func TestSkillService_ImportGitHub(t *testing.T) {
	db := setupTestDB(t)
	svc := newSkillServiceForTest(t, db)

	_, err := svc.InstallFromGitHub("https://github.com/example/skill")
	if err != ErrNotImplemented {
		t.Errorf("expected %v, got %v", ErrNotImplemented, err)
	}
}

func TestSkillService_ImportGitHub_InvalidURL(t *testing.T) {
	db := setupTestDB(t)
	svc := newSkillServiceForTest(t, db)

	_, err := svc.InstallFromGitHub("https://example.com/skill")
	if err != ErrInvalidGitHubURL {
		t.Errorf("expected %v, got %v", ErrInvalidGitHubURL, err)
	}
}

func TestSkillResponse_ToolCount(t *testing.T) {
	skill := &Skill{
		Name:        "test-skill",
		DisplayName: "Test Skill",
		ToolsSchema: []byte(`[{"name":"tool1"},{"name":"tool2"}]`),
	}

	resp := skill.ToResponse()
	if resp.ToolCount != 2 {
		t.Errorf("toolCount: expected 2, got %d", resp.ToolCount)
	}
	if resp.HasInstructions {
		t.Error("hasInstructions should be false when empty")
	}
}
