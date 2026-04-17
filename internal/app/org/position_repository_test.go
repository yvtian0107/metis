package org

import (
	"testing"
)

func TestPositionRepo_CreateFindByIDFindByCode(t *testing.T) {
	db := newOrgTestDB(t)
	repo := &PositionRepo{db: db}

	pos := &Position{Name: "Senior Engineer", Code: "se", IsActive: true}
	if err := repo.Create(pos); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if pos.ID == 0 {
		t.Fatal("expected non-zero id")
	}

	found, err := repo.FindByID(pos.ID)
	if err != nil {
		t.Fatalf("find by id failed: %v", err)
	}
	if found.Code != "se" {
		t.Fatalf("expected code se, got %s", found.Code)
	}

	foundByCode, err := repo.FindByCode("se")
	if err != nil {
		t.Fatalf("find by code failed: %v", err)
	}
	if foundByCode.ID != pos.ID {
		t.Fatalf("id mismatch")
	}
}

func TestPositionRepo_FindByID_NotFound(t *testing.T) {
	db := newOrgTestDB(t)
	repo := &PositionRepo{db: db}

	_, err := repo.FindByID(9999)
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestPositionRepo_Update(t *testing.T) {
	db := newOrgTestDB(t)
	repo := &PositionRepo{db: db}

	pos := seedPosition(t, db, "Engineer", "eng", true)

	if err := repo.Update(pos.ID, map[string]any{"name": "Staff Engineer"}); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	found, _ := repo.FindByID(pos.ID)
	if found.Name != "Staff Engineer" {
		t.Fatalf("update not applied: %+v", found)
	}
}

func TestPositionRepo_Delete(t *testing.T) {
	db := newOrgTestDB(t)
	repo := &PositionRepo{db: db}

	pos := seedPosition(t, db, "Engineer", "eng", true)

	if err := repo.Delete(pos.ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, err := repo.FindByID(pos.ID)
	if err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestPositionRepo_List(t *testing.T) {
	db := newOrgTestDB(t)
	repo := &PositionRepo{db: db}

	seedPosition(t, db, "Senior Engineer", "se", true)
	seedPosition(t, db, "Junior Engineer", "je", true)
	seedPosition(t, db, "Product Manager", "pm", true)

	// keyword filter
	items, total, err := repo.List(PositionListParams{Keyword: "engineer", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// pagination
	items, total, err = repo.List(PositionListParams{Keyword: "", Page: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("list pagination failed: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items on page 1, got %d", len(items))
	}

	// pageSize=0 returns all
	items, total, err = repo.List(PositionListParams{Keyword: "", Page: 1, PageSize: 0})
	if err != nil {
		t.Fatalf("list all failed: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items when pageSize=0, got %d", len(items))
	}
}

func TestPositionRepo_ListActive(t *testing.T) {
	db := newOrgTestDB(t)
	repo := &PositionRepo{db: db}

	seedPosition(t, db, "Active", "active", true)
	seedPosition(t, db, "Inactive", "inactive", false)

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

func TestPositionRepo_InUse(t *testing.T) {
	db := newOrgTestDB(t)
	repo := &PositionRepo{db: db}

	role := seedRole(t, db, "user")
	dept := seedDepartment(t, db, "Dept", "dept", nil, nil, true)
	user := seedUser(t, db, "u1", role.ID)
	pos := seedPosition(t, db, "Engineer", "eng", true)
	seedAssignment(t, db, user.ID, dept.ID, pos.ID, true)

	inUse, err := repo.InUse(pos.ID)
	if err != nil {
		t.Fatalf("in use failed: %v", err)
	}
	if !inUse {
		t.Fatal("expected in use true")
	}

	unusedPos := seedPosition(t, db, "Manager", "mgr", true)
	inUse, _ = repo.InUse(unusedPos.ID)
	if inUse {
		t.Fatal("expected in use false")
	}
}
