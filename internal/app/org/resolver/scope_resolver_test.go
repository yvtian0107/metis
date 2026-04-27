package resolver

import (
	"metis/internal/app/org/assignment"
	"metis/internal/app/org/department"
	"metis/internal/app/org/position"
	"metis/internal/app/org/testutil"
	"metis/internal/database"
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
