package department

import (
	"metis/internal/app/org/testutil"
	"testing"

	"metis/internal/database"
)

func newDepartmentService(db *database.DB) *DepartmentService {
	return &DepartmentService{
		repo: &DepartmentRepo{db: db},
	}
}

func TestDepartmentService_Create(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	svc := newDepartmentService(db)

	dept, err := svc.Create("Engineering", "eng", nil, nil, 1, "R&D")
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if dept.Code != "eng" {
		t.Fatalf("expected code eng, got %s", dept.Code)
	}

	_, err = svc.Create("Engineering 2", "eng", nil, nil, 2, "")
	if err != ErrDepartmentCodeExists {
		t.Fatalf("expected ErrDepartmentCodeExists, got %v", err)
	}
}

func TestDepartmentService_Get(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	svc := newDepartmentService(db)

	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)

	found, err := svc.Get(dept.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if found.ID != dept.ID {
		t.Fatal("id mismatch")
	}

	_, err = svc.Get(9999)
	if err != ErrDepartmentNotFound {
		t.Fatalf("expected ErrDepartmentNotFound, got %v", err)
	}
}

func TestDepartmentService_ListAll(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	svc := newDepartmentService(db)

	testutil.SeedDepartment(t, db, "A", "a", nil, nil, true)
	testutil.SeedDepartment(t, db, "B", "b", nil, nil, true)

	list, err := svc.ListAll()
	if err != nil {
		t.Fatalf("list all failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 items, got %d", len(list))
	}
}

func TestDepartmentService_Tree(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	svc := newDepartmentService(db)

	parent := testutil.SeedDepartment(t, db, "Parent", "parent", nil, nil, true)
	child := testutil.SeedDepartment(t, db, "Child", "child", &parent.ID, nil, true)

	role := testutil.SeedRole(t, db, "user")
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	u1 := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, u1.ID, parent.ID, pos.ID, true)
	testutil.SeedAssignment(t, db, u1.ID, child.ID, pos.ID, false)

	tree, err := svc.Tree()
	if err != nil {
		t.Fatalf("tree failed: %v", err)
	}
	if len(tree) != 1 {
		t.Fatalf("expected 1 root, got %d", len(tree))
	}
	if tree[0].Code != "parent" {
		t.Fatalf("expected root parent, got %s", tree[0].Code)
	}
	if tree[0].MemberCount != 1 {
		t.Fatalf("expected parent member count 1, got %d", tree[0].MemberCount)
	}
	if len(tree[0].Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(tree[0].Children))
	}
	if tree[0].Children[0].MemberCount != 1 {
		t.Fatalf("expected child member count 1, got %d", tree[0].Children[0].MemberCount)
	}
}

func TestDepartmentService_Update(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	svc := newDepartmentService(db)

	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	other := testutil.SeedDepartment(t, db, "Product", "prod", nil, nil, true)

	name := "R&D"
	updated, err := svc.Update(dept.ID, UpdateDepartmentInput{Name: &name})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if updated.Name != "R&D" {
		t.Fatalf("expected name R&D, got %s", updated.Name)
	}

	// code collision
	_, err = svc.Update(other.ID, UpdateDepartmentInput{Code: &dept.Code})
	if err != ErrDepartmentCodeExists {
		t.Fatalf("expected ErrDepartmentCodeExists, got %v", err)
	}
}

func TestDepartmentService_Delete(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	svc := newDepartmentService(db)

	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	if err := svc.Delete(dept.ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// not found
	err := svc.Delete(9999)
	if err != ErrDepartmentNotFound {
		t.Fatalf("expected ErrDepartmentNotFound, got %v", err)
	}

	// has children
	parent := testutil.SeedDepartment(t, db, "Parent", "parent", nil, nil, true)
	_ = testutil.SeedDepartment(t, db, "Child", "child", &parent.ID, nil, true)
	err = svc.Delete(parent.ID)
	if err != ErrDepartmentHasChildren {
		t.Fatalf("expected ErrDepartmentHasChildren, got %v", err)
	}

	// has members
	dept2 := testutil.SeedDepartment(t, db, "Dept2", "dept2", nil, nil, true)
	role := testutil.SeedRole(t, db, "user")
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, dept2.ID, pos.ID, true)
	err = svc.Delete(dept2.ID)
	if err != ErrDepartmentHasMembers {
		t.Fatalf("expected ErrDepartmentHasMembers, got %v", err)
	}
}
