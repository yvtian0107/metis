package assignment

import (
	"metis/internal/app/org/domain"
	"metis/internal/app/org/testutil"
	"testing"
)

func TestAssignmentRepo_FindByID(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	up := testutil.SeedAssignment(t, db, user.ID, dept.ID, pos.ID, true)

	found, err := repo.FindByID(up.ID)
	if err != nil {
		t.Fatalf("find by id failed: %v", err)
	}
	if found.ID != up.ID {
		t.Fatal("id mismatch")
	}
	if found.Department.Code != "eng" {
		t.Fatal("department not preloaded")
	}
	if found.Position.Code != "se" {
		t.Fatal("position not preloaded")
	}

	_, err = repo.FindByID(9999)
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestAssignmentRepo_FindByUserID(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, dept.ID, pos.ID, true)

	items, err := repo.FindByUserID(user.ID)
	if err != nil {
		t.Fatalf("find by user id failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Department.Code != "eng" {
		t.Fatal("department not preloaded")
	}
}

func TestAssignmentRepo_FindByDepartmentID(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, dept.ID, pos.ID, true)

	items, err := repo.FindByDepartmentID(dept.ID)
	if err != nil {
		t.Fatalf("find by department id failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Position.Code != "se" {
		t.Fatal("position not preloaded")
	}
}

func TestAssignmentRepo_AddPosition(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)

	up := &domain.UserPosition{UserID: user.ID, DepartmentID: dept.ID, PositionID: pos.ID}
	if err := repo.AddPosition(up); err != nil {
		t.Fatalf("add position failed: %v", err)
	}
	if up.ID == 0 {
		t.Fatal("expected non-zero id")
	}
}

func TestAssignmentRepo_AddPositionWithPrimary_DemoteExisting(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept1 := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	dept2 := testutil.SeedDepartment(t, db, "Product", "prod", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, dept1.ID, pos.ID, true)

	up := &domain.UserPosition{UserID: user.ID, DepartmentID: dept2.ID, PositionID: pos.ID}
	if err := repo.AddPositionWithPrimary(up, true, false); err != nil {
		t.Fatalf("add position with primary failed: %v", err)
	}
	if !up.IsPrimary {
		t.Fatal("expected new assignment to be primary")
	}

	// verify old one demoted
	old, _ := repo.FindByUserID(user.ID)
	primaryCount := 0
	for _, a := range old {
		if a.IsPrimary {
			primaryCount++
		}
	}
	if primaryCount != 1 {
		t.Fatalf("expected 1 primary, got %d", primaryCount)
	}
}

func TestAssignmentRepo_AddPositionWithPrimary_AutoSetPrimaryFirst(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)

	up := &domain.UserPosition{UserID: user.ID, DepartmentID: dept.ID, PositionID: pos.ID}
	if err := repo.AddPositionWithPrimary(up, false, true); err != nil {
		t.Fatalf("add position failed: %v", err)
	}
	if !up.IsPrimary {
		t.Fatal("expected first assignment to be auto-promoted to primary")
	}
}

func TestAssignmentRepo_AddPositionWithPrimary_AutoSetPrimaryNonFirst(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept1 := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	dept2 := testutil.SeedDepartment(t, db, "Product", "prod", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, dept1.ID, pos.ID, true)

	up := &domain.UserPosition{UserID: user.ID, DepartmentID: dept2.ID, PositionID: pos.ID}
	if err := repo.AddPositionWithPrimary(up, false, true); err != nil {
		t.Fatalf("add position failed: %v", err)
	}
	if up.IsPrimary {
		t.Fatal("expected non-first assignment to remain non-primary")
	}
}

func TestAssignmentRepo_ExistsByUserAndDept(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, dept.ID, pos.ID, true)

	exists, err := repo.ExistsByUserAndDept(user.ID, dept.ID)
	if err != nil {
		t.Fatalf("exists failed: %v", err)
	}
	if !exists {
		t.Fatal("expected exists true")
	}

	exists, _ = repo.ExistsByUserAndDept(user.ID, 9999)
	if exists {
		t.Fatal("expected exists false")
	}
}

func TestAssignmentRepo_RemovePosition(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	up := testutil.SeedAssignment(t, db, user.ID, dept.ID, pos.ID, false)

	if err := repo.RemovePosition(up.ID, user.ID); err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	_, err := repo.FindByID(up.ID)
	if err == nil {
		t.Fatal("expected not found after remove")
	}

	// not found error
	err = repo.RemovePosition(9999, user.ID)
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestAssignmentRepo_RemovePosition_AutoPromote(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept1 := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	dept2 := testutil.SeedDepartment(t, db, "Product", "prod", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	primary := testutil.SeedAssignment(t, db, user.ID, dept1.ID, pos.ID, true)
	secondary := testutil.SeedAssignment(t, db, user.ID, dept2.ID, pos.ID, false)
	// set sort on secondary to test ordering
	db.Model(secondary).Update("sort", 1)

	if err := repo.RemovePosition(primary.ID, user.ID); err != nil {
		t.Fatalf("remove primary failed: %v", err)
	}

	next, _ := repo.FindByID(secondary.ID)
	if !next.IsPrimary {
		t.Fatal("expected secondary to be promoted to primary")
	}
}

func TestAssignmentRepo_RemovePosition_LastAssignment(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	primary := testutil.SeedAssignment(t, db, user.ID, dept.ID, pos.ID, true)

	if err := repo.RemovePosition(primary.ID, user.ID); err != nil {
		t.Fatalf("remove last assignment failed: %v", err)
	}

	items, _ := repo.FindByUserID(user.ID)
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestAssignmentRepo_UpdatePositionWithPrimary(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos1 := testutil.SeedPosition(t, db, "SE", "se", true)
	pos2 := testutil.SeedPosition(t, db, "Manager", "mgr", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	up := testutil.SeedAssignment(t, db, user.ID, dept.ID, pos1.ID, false)

	if err := repo.UpdatePositionWithPrimary(up.ID, user.ID, map[string]any{"position_id": pos2.ID}, false); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	found, _ := repo.FindByID(up.ID)
	if found.PositionID != pos2.ID {
		t.Fatalf("expected position_id %d, got %d", pos2.ID, found.PositionID)
	}
}

func TestAssignmentRepo_UpdatePositionWithPrimary_SetPrimary(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept1 := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	dept2 := testutil.SeedDepartment(t, db, "Product", "prod", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, dept1.ID, pos.ID, true)
	up2 := testutil.SeedAssignment(t, db, user.ID, dept2.ID, pos.ID, false)

	if err := repo.UpdatePositionWithPrimary(up2.ID, user.ID, map[string]any{}, true); err != nil {
		t.Fatalf("update with primary failed: %v", err)
	}

	found, _ := repo.FindByID(up2.ID)
	if !found.IsPrimary {
		t.Fatal("expected target to become primary")
	}

	all, _ := repo.FindByUserID(user.ID)
	primaryCount := 0
	for _, a := range all {
		if a.IsPrimary {
			primaryCount++
		}
	}
	if primaryCount != 1 {
		t.Fatalf("expected exactly 1 primary, got %d", primaryCount)
	}
}

func TestAssignmentRepo_UpdatePositionWithPrimary_NotFound(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	user := testutil.SeedUser(t, db, "u1", role.ID)

	err := repo.UpdatePositionWithPrimary(9999, user.ID, map[string]any{"sort": 1}, false)
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestAssignmentRepo_SetPrimary(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept1 := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	dept2 := testutil.SeedDepartment(t, db, "Product", "prod", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, dept1.ID, pos.ID, true)
	up2 := testutil.SeedAssignment(t, db, user.ID, dept2.ID, pos.ID, false)

	if err := repo.SetPrimary(user.ID, up2.ID); err != nil {
		t.Fatalf("set primary failed: %v", err)
	}

	found, _ := repo.FindByID(up2.ID)
	if !found.IsPrimary {
		t.Fatal("expected target to be primary")
	}

	err := repo.SetPrimary(user.ID, 9999)
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestAssignmentRepo_ListUsersByDepartment(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	u1 := testutil.SeedUser(t, db, "alice", role.ID)
	u2 := testutil.SeedUser(t, db, "bob", role.ID)
	testutil.SeedAssignment(t, db, u1.ID, dept.ID, pos.ID, true)
	testutil.SeedAssignment(t, db, u2.ID, dept.ID, pos.ID, false)

	items, total, err := repo.ListUsersByDepartment(dept.ID, "", 1, 10)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Username != "alice" {
		t.Fatalf("expected primary user first, got %s", items[0].Username)
	}

	// keyword filter
	items, total, err = repo.ListUsersByDepartment(dept.ID, "bob", 1, 10)
	if err != nil {
		t.Fatalf("list with keyword failed: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected 1 item for bob, got total=%d len=%d", total, len(items))
	}

	// pagination
	items, total, err = repo.ListUsersByDepartment(dept.ID, "", 1, 1)
	if err != nil {
		t.Fatalf("list pagination failed: %v", err)
	}
	if total != 2 || len(items) != 1 {
		t.Fatalf("expected 1 item on page 1, got total=%d len=%d", total, len(items))
	}
}

func TestAssignmentRepo_CountByDepartments(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept1 := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	dept2 := testutil.SeedDepartment(t, db, "Product", "prod", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	u1 := testutil.SeedUser(t, db, "u1", role.ID)
	u2 := testutil.SeedUser(t, db, "u2", role.ID)
	testutil.SeedAssignment(t, db, u1.ID, dept1.ID, pos.ID, true)
	testutil.SeedAssignment(t, db, u2.ID, dept1.ID, pos.ID, false)
	testutil.SeedAssignment(t, db, u1.ID, dept2.ID, pos.ID, true)

	counts, err := repo.CountByDepartments()
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if counts[dept1.ID] != 2 {
		t.Fatalf("expected dept1 count 2, got %d", counts[dept1.ID])
	}
	if counts[dept2.ID] != 1 {
		t.Fatalf("expected dept2 count 1, got %d", counts[dept2.ID])
	}
}

func TestAssignmentRepo_GetUserDepartmentIDs(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept1 := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	dept2 := testutil.SeedDepartment(t, db, "Product", "prod", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, dept1.ID, pos.ID, true)
	testutil.SeedAssignment(t, db, user.ID, dept2.ID, pos.ID, false)

	ids, err := repo.GetUserDepartmentIDs(user.ID)
	if err != nil {
		t.Fatalf("get dept ids failed: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 dept ids, got %d", len(ids))
	}
}

func TestAssignmentRepo_GetUserPositionIDs(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept1 := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	dept2 := testutil.SeedDepartment(t, db, "Product", "prod", nil, nil, true)
	pos1 := testutil.SeedPosition(t, db, "SE", "se", true)
	pos2 := testutil.SeedPosition(t, db, "Manager", "mgr", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, dept1.ID, pos1.ID, true)
	testutil.SeedAssignment(t, db, user.ID, dept2.ID, pos2.ID, false)

	ids, err := repo.GetUserPositionIDs(user.ID)
	if err != nil {
		t.Fatalf("get pos ids failed: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 pos ids, got %d", len(ids))
	}
}

func TestAssignmentRepo_GetSubDepartmentIDs(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	parent := testutil.SeedDepartment(t, db, "Parent", "parent", nil, nil, true)
	_ = testutil.SeedDepartment(t, db, "ActiveChild", "active_child", &parent.ID, nil, true)
	_ = testutil.SeedDepartment(t, db, "InactiveChild", "inactive_child", &parent.ID, nil, false)

	all, err := repo.GetSubDepartmentIDs([]uint{parent.ID}, false)
	if err != nil {
		t.Fatalf("get sub dept ids failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 items, got %d", len(all))
	}

	active, err := repo.GetSubDepartmentIDs([]uint{parent.ID}, true)
	if err != nil {
		t.Fatalf("get active sub dept ids failed: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active item, got %d", len(active))
	}

	empty, err := repo.GetSubDepartmentIDs([]uint{}, true)
	if err != nil {
		t.Fatalf("empty parent ids failed: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected 0 items for empty parent ids, got %d", len(empty))
	}
}

func TestAssignmentRepo_GetUserPrimaryPosition(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, dept.ID, pos.ID, true)

	primary, err := repo.GetUserPrimaryPosition(user.ID)
	if err != nil {
		t.Fatalf("get primary failed: %v", err)
	}
	if primary == nil || !primary.IsPrimary {
		t.Fatal("expected primary assignment")
	}
	if primary.Department.Code != "eng" {
		t.Fatal("department not preloaded")
	}

	// no primary
	u2 := testutil.SeedUser(t, db, "u2", role.ID)
	_, err = repo.GetUserPrimaryPosition(u2.ID)
	if err == nil {
		t.Fatal("expected error when no primary")
	}
}

func TestAssignmentRepo_DeleteByID(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	up := testutil.SeedAssignment(t, db, user.ID, dept.ID, pos.ID, true)

	if err := repo.DeleteByID(up.ID); err != nil {
		t.Fatalf("delete by id failed: %v", err)
	}

	_, err := repo.FindByID(up.ID)
	if err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestAssignmentRepo_ExistsByUserDeptAndPosition(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos1 := testutil.SeedPosition(t, db, "SE", "se", true)
	pos2 := testutil.SeedPosition(t, db, "Manager", "mgr", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, dept.ID, pos1.ID, true)

	exists, err := repo.ExistsByUserDeptAndPosition(user.ID, dept.ID, pos1.ID)
	if err != nil {
		t.Fatalf("exists failed: %v", err)
	}
	if !exists {
		t.Fatal("expected exists true for assigned position")
	}

	exists, _ = repo.ExistsByUserDeptAndPosition(user.ID, dept.ID, pos2.ID)
	if exists {
		t.Fatal("expected exists false for unassigned position")
	}
}

func TestAssignmentRepo_FindByUserAndDept(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos1 := testutil.SeedPosition(t, db, "SE", "se", true)
	pos2 := testutil.SeedPosition(t, db, "Manager", "mgr", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, dept.ID, pos1.ID, true)
	testutil.SeedAssignment(t, db, user.ID, dept.ID, pos2.ID, false)

	items, err := repo.FindByUserAndDept(user.ID, dept.ID)
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	// Primary should be first
	if !items[0].IsPrimary {
		t.Fatal("expected primary first")
	}
}

func TestAssignmentRepo_SetUserDeptPositions(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos1 := testutil.SeedPosition(t, db, "SE", "se", true)
	pos2 := testutil.SeedPosition(t, db, "Manager", "mgr", true)
	pos3 := testutil.SeedPosition(t, db, "Architect", "arch", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)

	// Initial set
	primaryID := pos1.ID
	if err := repo.SetUserDeptPositions(user.ID, dept.ID, []uint{pos1.ID, pos2.ID}, &primaryID); err != nil {
		t.Fatalf("set positions failed: %v", err)
	}

	items, _ := repo.FindByUserAndDept(user.ID, dept.ID)
	if len(items) != 2 {
		t.Fatalf("expected 2, got %d", len(items))
	}

	// Update: keep pos1, remove pos2, add pos3
	primaryID = pos3.ID
	if err := repo.SetUserDeptPositions(user.ID, dept.ID, []uint{pos1.ID, pos3.ID}, &primaryID); err != nil {
		t.Fatalf("update positions failed: %v", err)
	}

	items, _ = repo.FindByUserAndDept(user.ID, dept.ID)
	if len(items) != 2 {
		t.Fatalf("expected 2 after update, got %d", len(items))
	}

	posIDs := map[uint]bool{}
	for _, item := range items {
		posIDs[item.PositionID] = true
	}
	if posIDs[pos2.ID] {
		t.Fatal("pos2 should have been removed")
	}
	if !posIDs[pos3.ID] {
		t.Fatal("pos3 should have been added")
	}
}

func TestAssignmentRepo_MultiPositionPerDept(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	repo := &AssignmentRepo{db: db}

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos1 := testutil.SeedPosition(t, db, "SE", "se", true)
	pos2 := testutil.SeedPosition(t, db, "Manager", "mgr", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)

	// Can add two different positions in same dept
	up1 := &domain.UserPosition{UserID: user.ID, DepartmentID: dept.ID, PositionID: pos1.ID, IsPrimary: true}
	if err := repo.AddPosition(up1); err != nil {
		t.Fatalf("add pos1 failed: %v", err)
	}
	up2 := &domain.UserPosition{UserID: user.ID, DepartmentID: dept.ID, PositionID: pos2.ID}
	if err := repo.AddPosition(up2); err != nil {
		t.Fatalf("add pos2 in same dept failed: %v", err)
	}

	items, _ := repo.FindByUserAndDept(user.ID, dept.ID)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// Cannot add same position twice
	up3 := &domain.UserPosition{UserID: user.ID, DepartmentID: dept.ID, PositionID: pos1.ID}
	err := repo.AddPosition(up3)
	if err == nil {
		t.Fatal("expected unique constraint error for duplicate user+dept+position")
	}
}

func TestGroupAssignmentsByUser(t *testing.T) {
	items := []domain.AssignmentItem{
		{UserID: 1, Username: "alice", Email: "alice@test.com", DepartmentID: 10, PositionID: 100, PositionName: "SE", IsPrimary: true, AssignmentID: 1},
		{UserID: 1, Username: "alice", Email: "alice@test.com", DepartmentID: 10, PositionID: 200, PositionName: "Manager", IsPrimary: false, AssignmentID: 2},
		{UserID: 2, Username: "bob", Email: "bob@test.com", DepartmentID: 10, PositionID: 100, PositionName: "SE", IsPrimary: false, AssignmentID: 3},
	}

	grouped := GroupAssignmentsByUser(items)
	if len(grouped) != 2 {
		t.Fatalf("expected 2 members, got %d", len(grouped))
	}
	if grouped[0].Username != "alice" {
		t.Fatal("expected alice first (order preserved)")
	}
	if len(grouped[0].Positions) != 2 {
		t.Fatalf("expected 2 positions for alice, got %d", len(grouped[0].Positions))
	}
	if len(grouped[1].Positions) != 1 {
		t.Fatalf("expected 1 position for bob, got %d", len(grouped[1].Positions))
	}

	// Empty input
	empty := GroupAssignmentsByUser(nil)
	if empty != nil {
		t.Fatal("expected nil for empty input")
	}
}
