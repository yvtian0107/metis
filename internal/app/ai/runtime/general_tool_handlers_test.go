package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type staticUserFinder struct {
	user *GeneralUserInfo
}

func (f staticUserFinder) FindByID(id uint) (*GeneralUserInfo, error) {
	return f.user, nil
}

func TestCurrentUserProfileRedactsSensitiveFields(t *testing.T) {
	registry := NewGeneralToolRegistry(staticUserFinder{user: &GeneralUserInfo{
		ID:              42,
		Username:        "ada",
		Email:           "ada@example.com",
		Phone:           "123456",
		Avatar:          "avatar.png",
		RoleID:          9,
		RoleName:        "Engineer",
		RoleCode:        "admin",
		ManagerID:       uintPtr(7),
		ManagerUsername: "grace",
	}}, nil)

	raw, err := registry.Execute(context.Background(), "system.current_user_profile", 42, nil)
	if err != nil {
		t.Fatalf("execute profile tool: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	user, ok := result["user"].(map[string]any)
	if !ok {
		t.Fatalf("expected user object, got %#v", result)
	}
	for _, key := range []string{"email", "phone", "avatar", "roleId", "roleCode", "managerId"} {
		if _, exists := user[key]; exists {
			t.Fatalf("sensitive field %q leaked in result: %s", key, string(raw))
		}
	}
	if !strings.Contains(string(raw), "Engineer") || !strings.Contains(string(raw), "grace") {
		t.Fatalf("expected safe context fields in result: %s", string(raw))
	}
}
