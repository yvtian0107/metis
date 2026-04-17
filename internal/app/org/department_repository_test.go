package org

import (
	"testing"
)

func TestDepartmentRepo_CreateFindByIDFindByCode(t *testing.T) {
	db := newOrgTestDB(t)
	repo := &DepartmentRepo{db: db}

	dept := &Department{Name: "Engineering", Code: "eng", IsActive: true}
	if err := repo.Create(dept); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if dept.ID == 0 {
		t.Fatal("expected non-zero id")
	}

	found, err := repo.FindByID(dept.ID)
	if err != nil {
		t.Fatalf("find by id failed: %v", err)
	}
	if found.Code != "eng" {
		t.Fatalf("expected code eng, got %s", found.Code)
	}

	foundByCode, err := repo.FindByCode("eng")
	if err != nil {
		t.Fatalf("find by code failed: %v", err)
	}
	if foundByCode.ID != dept.ID {
		t.Fatalf("id mismatch")
	}
}

func TestDepartmentRepo_FindByID_NotFound(t *testing.T) {
	db := newOrgTestDB(t)
	repo := &DepartmentRepo{db: db}

	_, err := repo.FindByID(9999)
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestDepartmentRepo_Update(t *testing.T) {
	db := newOrgTestDB(t)
	repo := &DepartmentRepo{db: db}

	dept := seedDepartment(t, db, "Engineering", "eng", nil, nil, true)

	if err := repo.Update(dept.ID, map[string]any{"name": "R&D", "sort": 10}); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	found, _ := repo.FindByID(dept.ID)
	if found.Name != "R&D" || found.Sort != 10 {
		t.Fatalf("update not applied: %+v", found)
	}
}

func TestDepartmentRepo_Delete(t *testing.T) {
	db := newOrgTestDB(t)
	repo := &DepartmentRepo{db: db}

	dept := seedDepartment(t, db, "Engineering", "eng", nil, nil, true)

	if err := repo.Delete(dept.ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, err := repo.FindByID(dept.ID)
	if err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestDepartmentRepo_ListAll(t *testing.T) {
	db := newOrgTestDB(t)
	repo := &DepartmentRepo{db: db}

	seedDepartment(t, db, "B", "b", nil, nil, true)
	seedDepartment(t, db, "A", "a", nil, nil, true)

	list, err := repo.ListAll()
	if err != nil {
		t.Fatalf("list all failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 items, got %d", len(list))
	}
	// default ordering: sort ASC, id ASC
	if list[0].Code != "b" || list[1].Code != "a" {
		t.Fatalf("unexpected order: %v", list)
	}
}

func TestDepartmentRepo_ListActive(t *testing.T) {
	db := newOrgTestDB(t)
	repo := &DepartmentRepo{db: db}

	seedDepartment(t, db, "Active", "active", nil, nil, true)
	seedDepartment(t, db, "Inactive", "inactive", nil, nil, false)

	list, err := repo.ListActive()
	if err != nil {
		t.Fatalf("list active failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 active item, got %d", len(list))
	}
	if list[0].Code != "active" {
		t.Fatalf("unexpected code: %s", list[0].Code)
	}
}

func TestDepartmentRepo_HasChildren(t *testing.T) {
	db := newOrgTestDB(t)
	repo := &DepartmentRepo{db: db}

	parent := seedDepartment(t, db, "Parent", "parent", nil, nil, true)
	_ = seedDepartment(t, db, "Child", "child", &parent.ID, nil, true)

	has, err := repo.HasChildren(parent.ID)
	if err != nil {
		t.Fatalf("has children failed: %v", err)
	}
	if !has {
		t.Fatal("expected has children true")
	}

	has, _ = repo.HasChildren(9999)
	if has {
		t.Fatal("expected has children false")
	}
}

func TestDepartmentRepo_HasMembers(t *testing.T) {
	db := newOrgTestDB(t)
	repo := &DepartmentRepo{db: db}

	role := seedRole(t, db, "user")
	dept := seedDepartment(t, db, "Dept", "dept", nil, nil, true)
	user := seedUser(t, db, "u1", role.ID)
	seedAssignment(t, db, user.ID, dept.ID, 1, false)

	has, err := repo.HasMembers(dept.ID)
	if err != nil {
		t.Fatalf("has members failed: %v", err)
	}
	if !has {
		t.Fatal("expected has members true")
	}

	has, _ = repo.HasMembers(9999)
	if has {
		t.Fatal("expected has members false")
	}
}

func TestDepartmentRepo_ListAllIDsWithParent(t *testing.T) {
	db := newOrgTestDB(t)
	repo := &DepartmentRepo{db: db}

	parent := seedDepartment(t, db, "Parent", "parent", nil, nil, true)
	seedDepartment(t, db, "ActiveChild", "active_child", &parent.ID, nil, true)
	seedDepartment(t, db, "InactiveChild", "inactive_child", &parent.ID, nil, false)

	all, err := repo.ListAllIDsWithParent(false)
	if err != nil {
		t.Fatalf("list all ids failed: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 items, got %d", len(all))
	}

	active, err := repo.ListAllIDsWithParent(true)
	if err != nil {
		t.Fatalf("list active ids failed: %v", err)
	}
	if len(active) != 2 {
		t.Fatalf("expected 2 active items, got %d", len(active))
	}
}
