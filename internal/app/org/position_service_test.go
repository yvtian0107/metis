package org

import (
	"testing"
)

func TestPositionService_Create(t *testing.T) {
	db := newOrgTestDB(t)
	svc := &PositionService{repo: &PositionRepo{db: db}}

	pos, err := svc.Create("Senior Engineer", "se", "L5")
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if pos.Code != "se" {
		t.Fatalf("expected code se, got %s", pos.Code)
	}

	_, err = svc.Create("Staff Engineer", "se", "L6")
	if err != ErrPositionCodeExists {
		t.Fatalf("expected ErrPositionCodeExists, got %v", err)
	}
}

func TestPositionService_Get(t *testing.T) {
	db := newOrgTestDB(t)
	svc := &PositionService{repo: &PositionRepo{db: db}}

	pos := seedPosition(t, db, "Senior Engineer", "se", true)

	found, err := svc.Get(pos.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if found.ID != pos.ID {
		t.Fatal("id mismatch")
	}

	_, err = svc.Get(9999)
	if err != ErrPositionNotFound {
		t.Fatalf("expected ErrPositionNotFound, got %v", err)
	}
}

func TestPositionService_List(t *testing.T) {
	db := newOrgTestDB(t)
	svc := &PositionService{repo: &PositionRepo{db: db}}

	seedPosition(t, db, "Senior Engineer", "se", true)
	seedPosition(t, db, "Junior Engineer", "je", true)
	seedPosition(t, db, "Product Manager", "pm", true)

	items, total, err := svc.List(PositionListParams{Keyword: "engineer", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}

func TestPositionService_ListActive(t *testing.T) {
	db := newOrgTestDB(t)
	svc := &PositionService{repo: &PositionRepo{db: db}}

	seedPosition(t, db, "Active", "active", true)
	seedPosition(t, db, "Inactive", "inactive", false)

	list, err := svc.ListActive()
	if err != nil {
		t.Fatalf("list active failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 active item, got %d", len(list))
	}
}

func TestPositionService_Update(t *testing.T) {
	db := newOrgTestDB(t)
	svc := &PositionService{repo: &PositionRepo{db: db}}

	pos := seedPosition(t, db, "Senior Engineer", "se", true)
	other := seedPosition(t, db, "Manager", "mgr", true)

	name := "Staff Engineer"
	updated, err := svc.Update(pos.ID, &name, nil, nil, nil)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if updated.Name != "Staff Engineer" {
		t.Fatalf("expected name Staff Engineer, got %s", updated.Name)
	}

	// code collision
	_, err = svc.Update(other.ID, nil, &pos.Code, nil, nil)
	if err != ErrPositionCodeExists {
		t.Fatalf("expected ErrPositionCodeExists, got %v", err)
	}
}

func TestPositionService_Delete(t *testing.T) {
	db := newOrgTestDB(t)
	svc := &PositionService{repo: &PositionRepo{db: db}}

	pos := seedPosition(t, db, "Senior Engineer", "se", true)
	if err := svc.Delete(pos.ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// not found
	err := svc.Delete(9999)
	if err != ErrPositionNotFound {
		t.Fatalf("expected ErrPositionNotFound, got %v", err)
	}

	// in use
	pos2 := seedPosition(t, db, "Engineer", "eng", true)
	role := seedRole(t, db, "user")
	dept := seedDepartment(t, db, "Engineering", "dept", nil, nil, true)
	user := seedUser(t, db, "u1", role.ID)
	seedAssignment(t, db, user.ID, dept.ID, pos2.ID, true)
	err = svc.Delete(pos2.ID)
	if err != ErrPositionInUse {
		t.Fatalf("expected ErrPositionInUse, got %v", err)
	}
}
