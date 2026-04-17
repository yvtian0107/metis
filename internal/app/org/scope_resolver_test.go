package org

import (
	"testing"
)

func TestOrgResolverImpl_GetUserDeptScope(t *testing.T) {
	db := newOrgTestDB(t)
	svc := newAssignmentService(db)
	repo := &AssignmentRepo{db: db}
	resolver := &OrgResolverImpl{svc: svc, repo: repo, db: db.DB}

	role := seedRole(t, db, "user")
	parent := seedDepartment(t, db, "Parent", "parent", nil, nil, true)
	_ = seedDepartment(t, db, "ActiveChild", "active_child", &parent.ID, nil, true)
	_ = seedDepartment(t, db, "InactiveChild", "inactive_child", &parent.ID, nil, false)
	pos := seedPosition(t, db, "SE", "se", true)
	user := seedUser(t, db, "u1", role.ID)
	seedAssignment(t, db, user.ID, parent.ID, pos.ID, true)

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
	db := newOrgTestDB(t)
	repo := &AssignmentRepo{db: db}
	svc := newAssignmentService(db)
	resolver := &OrgResolverImpl{svc: svc, repo: repo, db: db.DB}

	role := seedRole(t, db, "user")
	dept1 := seedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	dept2 := seedDepartment(t, db, "Product", "prod", nil, nil, true)
	pos1 := seedPosition(t, db, "SE", "se", true)
	pos2 := seedPosition(t, db, "Manager", "mgr", true)
	user := seedUser(t, db, "u1", role.ID)
	seedAssignment(t, db, user.ID, dept1.ID, pos1.ID, true)
	seedAssignment(t, db, user.ID, dept2.ID, pos2.ID, false)

	ids, err := resolver.GetUserPositionIDs(user.ID)
	if err != nil {
		t.Fatalf("get position ids failed: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 position ids, got %d", len(ids))
	}
}

func TestOrgResolverImpl_GetUserDepartmentIDs(t *testing.T) {
	db := newOrgTestDB(t)
	repo := &AssignmentRepo{db: db}
	svc := newAssignmentService(db)
	resolver := &OrgResolverImpl{svc: svc, repo: repo, db: db.DB}

	role := seedRole(t, db, "user")
	dept1 := seedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	dept2 := seedDepartment(t, db, "Product", "prod", nil, nil, true)
	pos := seedPosition(t, db, "SE", "se", true)
	user := seedUser(t, db, "u1", role.ID)
	seedAssignment(t, db, user.ID, dept1.ID, pos.ID, true)
	seedAssignment(t, db, user.ID, dept2.ID, pos.ID, false)

	ids, err := resolver.GetUserDepartmentIDs(user.ID)
	if err != nil {
		t.Fatalf("get department ids failed: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 department ids, got %d", len(ids))
	}
}
