package resolver

import (
	"encoding/json"
	"fmt"
	"metis/internal/app/org/assignment"
	"metis/internal/app/org/department"
	"metis/internal/app/org/domain"
	"metis/internal/app/org/position"
	"metis/internal/app/org/testutil"
	"metis/internal/database"
	"metis/internal/model"
	"strings"
	"testing"

	"github.com/samber/do/v2"
)

func newResolverForTest(db *database.DB) *OrgResolverImpl {
	injector := do.New()
	do.ProvideValue(injector, db)
	do.Provide(injector, assignment.NewAssignmentRepo)
	do.Provide(injector, assignment.NewAssignmentService)
	do.Provide(injector, department.NewDepartmentRepo)
	do.Provide(injector, position.NewPositionRepo)
	return NewOrgResolver(
		do.MustInvoke[*assignment.AssignmentService](injector),
		do.MustInvoke[*assignment.AssignmentRepo](injector),
		db.DB,
	)
}

func TestOrgResolverImpl_GetUserDeptScope(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	resolver := newResolverForTest(db)

	role := testutil.SeedRole(t, db, "user")
	parent := testutil.SeedDepartment(t, db, "Parent", "parent", nil, nil, true)
	_ = testutil.SeedDepartment(t, db, "ActiveChild", "active_child", &parent.ID, nil, true)
	_ = testutil.SeedDepartment(t, db, "InactiveChild", "inactive_child", &parent.ID, nil, false)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, parent.ID, pos.ID, true)

	// includeSubDepts=true
	scope, err := resolver.GetUserDeptScope(user.ID, true)
	if err != nil {
		t.Fatalf("get scope with sub-depts failed: %v", err)
	}
	if len(scope) != 2 {
		t.Fatalf("expected 2 scoped departments, got %d", len(scope))
	}

	// includeSubDepts=false
	direct, err := resolver.GetUserDeptScope(user.ID, false)
	if err != nil {
		t.Fatalf("get scope without sub-depts failed: %v", err)
	}
	if len(direct) != 1 {
		t.Fatalf("expected 1 direct department, got %d", len(direct))
	}
}

func TestOrgResolverImpl_GetUserPositionIDs(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	resolver := newResolverForTest(db)

	role := testutil.SeedRole(t, db, "user")
	dept1 := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	dept2 := testutil.SeedDepartment(t, db, "Product", "prod", nil, nil, true)
	pos1 := testutil.SeedPosition(t, db, "SE", "se", true)
	pos2 := testutil.SeedPosition(t, db, "Manager", "mgr", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, dept1.ID, pos1.ID, true)
	testutil.SeedAssignment(t, db, user.ID, dept2.ID, pos2.ID, false)

	ids, err := resolver.GetUserPositionIDs(user.ID)
	if err != nil {
		t.Fatalf("get position ids failed: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 position ids, got %d", len(ids))
	}
}

func TestOrgResolverImpl_GetUserDepartmentIDs(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	resolver := newResolverForTest(db)

	role := testutil.SeedRole(t, db, "user")
	dept1 := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	dept2 := testutil.SeedDepartment(t, db, "Product", "prod", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, dept1.ID, pos.ID, true)
	testutil.SeedAssignment(t, db, user.ID, dept2.ID, pos.ID, false)

	ids, err := resolver.GetUserDepartmentIDs(user.ID)
	if err != nil {
		t.Fatalf("get department ids failed: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 department ids, got %d", len(ids))
	}
}

func TestOrgResolverImpl_QueryContextWithoutFiltersReturnsOrgVocabularyOnly(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	resolver := newResolverForTest(db)

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "信息部", "it", nil, nil, true)
	_ = testutil.SeedDepartment(t, db, "停用部门", "inactive_dept", nil, nil, false)
	pos := testutil.SeedPosition(t, db, "网络管理员", "network_admin", true)
	_ = testutil.SeedPosition(t, db, "停用岗位", "inactive_pos", false)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, dept.ID, pos.ID, true)

	got, err := resolver.QueryContext("", "", "", false)
	if err != nil {
		t.Fatalf("query org context: %v", err)
	}
	if len(got.Users) != 0 {
		t.Fatalf("expected no users in unfiltered org vocabulary context, got %+v", got.Users)
	}
	if len(got.Departments) != 1 || got.Departments[0].Code != "it" || got.Departments[0].Name != "信息部" {
		t.Fatalf("expected active department vocabulary, got %+v", got.Departments)
	}
	if len(got.Positions) != 1 || got.Positions[0].Code != "network_admin" || got.Positions[0].Name != "网络管理员" {
		t.Fatalf("expected active position vocabulary, got %+v", got.Positions)
	}
}

func TestOrgResolverImpl_SearchOrgStructureMatchesNameAndCodeWithoutUsers(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	resolver := newResolverForTest(db)

	it := testutil.SeedDepartment(t, db, "信息部", "it", nil, nil, true)
	_ = it
	_ = testutil.SeedDepartment(t, db, "停用信息部", "it_inactive", nil, nil, false)
	_ = testutil.SeedPosition(t, db, "网络管理员", "network_admin", true)
	_ = testutil.SeedPosition(t, db, "停用网络管理员", "network_admin_inactive", false)
	role := testutil.SeedRole(t, db, "user")
	user := testutil.SeedUser(t, db, "network_admin_user", role.ID)
	testutil.SeedAssignment(t, db, user.ID, it.ID, testutil.SeedPosition(t, db, "其他岗位", "other_position", true).ID, true)

	byName, err := resolver.SearchOrgStructure("信息部", []string{"department"}, 10)
	if err != nil {
		t.Fatalf("search department by name: %v", err)
	}
	if len(byName.Departments) != 1 || byName.Departments[0].Code != "it" {
		t.Fatalf("expected active department match by name, got %+v", byName.Departments)
	}

	byCode, err := resolver.SearchOrgStructure("security_admin", []string{"position"}, 10)
	if err != nil {
		t.Fatalf("search missing position by code: %v", err)
	}
	if len(byCode.Positions) != 0 {
		t.Fatalf("expected no security_admin position yet, got %+v", byCode.Positions)
	}

	network, err := resolver.SearchOrgStructure("network_admin", []string{"position"}, 10)
	if err != nil {
		t.Fatalf("search position by code: %v", err)
	}
	if len(network.Positions) != 1 || network.Positions[0].Code != "network_admin" || network.Positions[0].Name != "网络管理员" {
		t.Fatalf("expected active position match by code, got %+v", network.Positions)
	}

	payload, err := json.Marshal(network)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	text := string(payload)
	for _, forbidden := range []string{"users", "user_id", "username", "email", "network_admin_user"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("structure search result should not expose user detail %q: %s", forbidden, text)
		}
	}
}

func TestOrgResolverImpl_SearchOrgStructureFindsTargetBeyondDefaultVocabularyLimit(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	resolver := newResolverForTest(db)

	for i := 0; i < 60; i++ {
		testutil.SeedPosition(t, db, "普通岗位", "generic_position_"+string(rune('a'+i%26))+string(rune('a'+i/26)), true)
	}
	target := testutil.SeedPosition(t, db, "信息安全管理员", "security_admin", true)
	if err := db.Model(target).Update("sort", 999).Error; err != nil {
		t.Fatalf("update target sort: %v", err)
	}

	vocab, err := resolver.QueryContext("", "", "", false)
	if err != nil {
		t.Fatalf("query default vocabulary: %v", err)
	}
	for _, pos := range vocab.Positions {
		if pos.Code == "security_admin" {
			t.Fatalf("target should not rely on default 50-item vocabulary, got %+v", vocab.Positions)
		}
	}

	got, err := resolver.SearchOrgStructure("信息安全管理员", []string{"position"}, 10)
	if err != nil {
		t.Fatalf("search target: %v", err)
	}
	if len(got.Positions) != 1 || got.Positions[0].Code != "security_admin" {
		t.Fatalf("expected targeted search to find security_admin, got %+v", got.Positions)
	}
}

func TestOrgResolverImpl_ResolveOrgParticipantReturnsPositionDepartmentCandidate(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	resolver := newResolverForTest(db)

	dept := testutil.SeedDepartment(t, db, "信息部", "it", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "网络管理员", "network_admin", true)
	role := testutil.SeedRole(t, db, "user")
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, dept.ID, pos.ID, true)
	if err := db.Create(&domain.DepartmentPosition{DepartmentID: dept.ID, PositionID: pos.ID}).Error; err != nil {
		t.Fatalf("seed department position: %v", err)
	}

	got, err := resolver.ResolveOrgParticipant("信息部", "网络管理员", 10)
	if err != nil {
		t.Fatalf("resolve participant: %v", err)
	}
	if len(got.Candidates) != 1 {
		t.Fatalf("expected one candidate, got %+v", got.Candidates)
	}
	candidate := got.Candidates[0]
	if candidate.Type != "position_department" ||
		candidate.DepartmentCode != "it" ||
		candidate.PositionCode != "network_admin" ||
		candidate.CandidateCount != 1 {
		t.Fatalf("unexpected candidate: %+v", candidate)
	}
}

func TestOrgResolverImpl_ResolveOrgParticipantDoesNotExposeUsersWithManyUsers(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	resolver := newResolverForTest(db)

	dept := testutil.SeedDepartment(t, db, "信息部", "it", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "网络管理员", "network_admin", true)
	role := testutil.SeedRole(t, db, "user")
	users := make([]model.User, 10000)
	for i := range users {
		users[i] = model.User{Username: fmt.Sprintf("bulk_user_%05d", i), RoleID: role.ID, IsActive: true}
	}
	if err := db.CreateInBatches(&users, 500).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	assignments := make([]domain.UserPosition, len(users))
	for i, user := range users {
		assignments[i] = domain.UserPosition{UserID: user.ID, DepartmentID: dept.ID, PositionID: pos.ID, IsPrimary: i == 0}
	}
	if err := db.CreateInBatches(&assignments, 500).Error; err != nil {
		t.Fatalf("seed assignments: %v", err)
	}

	got, err := resolver.ResolveOrgParticipant("信息部", "网络管理员", 10)
	if err != nil {
		t.Fatalf("resolve participant: %v", err)
	}
	if len(got.Candidates) != 1 || got.Candidates[0].CandidateCount != 10000 {
		t.Fatalf("expected aggregate candidate count only, got %+v", got.Candidates)
	}
	payload, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	text := string(payload)
	for _, forbidden := range []string{"users", "user_id", "username", "email", "bulk_user"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("participant resolve result should not expose user detail %q: %s", forbidden, text)
		}
	}
}
