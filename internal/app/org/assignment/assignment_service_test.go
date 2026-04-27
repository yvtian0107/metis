package assignment

import (
	"metis/internal/app/org/department"
	"metis/internal/app/org/position"
	"metis/internal/app/org/testutil"
	"testing"

	"github.com/samber/do/v2"

	"metis/internal/database"
)

func newAssignmentService(db *database.DB) *AssignmentService {
	injector := do.New()
	do.ProvideValue(injector, db)
	do.Provide(injector, NewAssignmentRepo)
	do.Provide(injector, department.NewDepartmentRepo)
	do.Provide(injector, position.NewPositionRepo)
	do.Provide(injector, NewAssignmentService)
	return do.MustInvoke[*AssignmentService](injector)
}

func TestAssignmentService_GetUserPositions(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	svc := newAssignmentService(db)

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, dept.ID, pos.ID, true)

	resp, err := svc.GetUserPositions(user.ID)
	if err != nil {
		t.Fatalf("get user positions failed: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp))
	}
	if resp[0].Department == nil || resp[0].Department.Code != "eng" {
		t.Fatal("department not preloaded in response")
	}
	if resp[0].Position == nil || resp[0].Position.Code != "se" {
		t.Fatal("position not preloaded in response")
	}
}

func TestAssignmentService_AddUserPosition(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	svc := newAssignmentService(db)

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)

	// first assignment with isPrimary=false gets auto-promoted because autoSetPrimary=true
	up, err := svc.AddUserPosition(user.ID, dept.ID, pos.ID, false)
	if err != nil {
		t.Fatalf("add user position failed: %v", err)
	}
	if !up.IsPrimary {
		t.Fatal("expected auto-promoted primary for first assignment")
	}

	// second assignment with isPrimary=false should remain non-primary
	dept2 := testutil.SeedDepartment(t, db, "Product", "prod", nil, nil, true)
	up2, err := svc.AddUserPosition(user.ID, dept2.ID, pos.ID, false)
	if err != nil {
		t.Fatalf("add second position failed: %v", err)
	}
	if up2.IsPrimary {
		t.Fatal("expected non-primary for second assignment")
	}

	// explicit primary
	dept3 := testutil.SeedDepartment(t, db, "Sales", "sales", nil, nil, true)
	up3, err := svc.AddUserPosition(user.ID, dept3.ID, pos.ID, true)
	if err != nil {
		t.Fatalf("add primary position failed: %v", err)
	}
	if !up3.IsPrimary {
		t.Fatal("expected primary when isPrimary=true")
	}
}

func TestAssignmentService_AddUserPosition_ValidationErrors(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	svc := newAssignmentService(db)

	role := testutil.SeedRole(t, db, "user")
	activeDept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	inactiveDept := testutil.SeedDepartment(t, db, "Inactive", "inactive", nil, nil, false)
	activePos := testutil.SeedPosition(t, db, "SE", "se", true)
	inactivePos := testutil.SeedPosition(t, db, "InactivePos", "inactive_pos", false)
	user := testutil.SeedUser(t, db, "u1", role.ID)

	// department not found
	_, err := svc.AddUserPosition(user.ID, 9999, activePos.ID, false)
	if err != department.ErrDepartmentNotFound {
		t.Fatalf("expected department.ErrDepartmentNotFound, got %v", err)
	}

	// department inactive
	_, err = svc.AddUserPosition(user.ID, inactiveDept.ID, activePos.ID, false)
	if err != ErrDepartmentInactive {
		t.Fatalf("expected ErrDepartmentInactive, got %v", err)
	}

	// position not found
	_, err = svc.AddUserPosition(user.ID, activeDept.ID, 9999, false)
	if err != position.ErrPositionNotFound {
		t.Fatalf("expected position.ErrPositionNotFound, got %v", err)
	}

	// position inactive
	_, err = svc.AddUserPosition(user.ID, activeDept.ID, inactivePos.ID, false)
	if err != ErrPositionInactive {
		t.Fatalf("expected ErrPositionInactive, got %v", err)
	}
}

func TestAssignmentService_AddUserPosition_AlreadyAssigned(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	svc := newAssignmentService(db)

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, dept.ID, pos.ID, true)

	// Same user + dept + position → ErrPositionAlreadyAssigned
	_, err := svc.AddUserPosition(user.ID, dept.ID, pos.ID, false)
	if err != ErrPositionAlreadyAssigned {
		t.Fatalf("expected ErrPositionAlreadyAssigned, got %v", err)
	}

	// Same user + dept + different position → OK (multi-position per dept)
	pos2 := testutil.SeedPosition(t, db, "Manager", "mgr", true)
	_, err = svc.AddUserPosition(user.ID, dept.ID, pos2.ID, false)
	if err != nil {
		t.Fatalf("expected success for different position in same dept, got %v", err)
	}
}

func TestAssignmentService_RemoveUserPosition(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	svc := newAssignmentService(db)

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	up := testutil.SeedAssignment(t, db, user.ID, dept.ID, pos.ID, true)

	if err := svc.RemoveUserPosition(user.ID, up.ID); err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	err := svc.RemoveUserPosition(user.ID, 9999)
	if err != ErrAssignmentNotFound {
		t.Fatalf("expected ErrAssignmentNotFound, got %v", err)
	}
}

func TestAssignmentService_UpdateUserPosition(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	svc := newAssignmentService(db)

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos1 := testutil.SeedPosition(t, db, "SE", "se", true)
	pos2 := testutil.SeedPosition(t, db, "Manager", "mgr", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	up := testutil.SeedAssignment(t, db, user.ID, dept.ID, pos1.ID, false)

	// update position only
	newPosID := pos2.ID
	if err := svc.UpdateUserPosition(user.ID, up.ID, &newPosID, nil); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	found, _ := svc.repo.FindByID(up.ID)
	if found.PositionID != pos2.ID {
		t.Fatalf("expected position_id %d, got %d", pos2.ID, found.PositionID)
	}

	// no-op
	if err := svc.UpdateUserPosition(user.ID, up.ID, nil, nil); err != nil {
		t.Fatalf("no-op update failed: %v", err)
	}

	// not found
	err := svc.UpdateUserPosition(user.ID, 9999, &newPosID, nil)
	if err != ErrAssignmentNotFound {
		t.Fatalf("expected ErrAssignmentNotFound, got %v", err)
	}
}

func TestAssignmentService_SetPrimary(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	svc := newAssignmentService(db)

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	up := testutil.SeedAssignment(t, db, user.ID, dept.ID, pos.ID, false)

	if err := svc.SetPrimary(user.ID, up.ID); err != nil {
		t.Fatalf("set primary failed: %v", err)
	}
	found, _ := svc.repo.FindByID(up.ID)
	if !found.IsPrimary {
		t.Fatal("expected primary")
	}

	err := svc.SetPrimary(user.ID, 9999)
	if err != ErrAssignmentNotFound {
		t.Fatalf("expected ErrAssignmentNotFound, got %v", err)
	}
}

func TestAssignmentService_ListDepartmentMembers(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	svc := newAssignmentService(db)

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	u1 := testutil.SeedUser(t, db, "alice", role.ID)
	u2 := testutil.SeedUser(t, db, "bob", role.ID)
	testutil.SeedAssignment(t, db, u1.ID, dept.ID, pos.ID, true)
	testutil.SeedAssignment(t, db, u2.ID, dept.ID, pos.ID, false)

	items, total, err := svc.ListDepartmentMembers(dept.ID, "", 1, 10)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("expected 2 items, got total=%d len=%d", total, len(items))
	}

	items, total, err = svc.ListDepartmentMembers(dept.ID, "alice", 1, 10)
	if err != nil {
		t.Fatalf("list with keyword failed: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected 1 item, got total=%d len=%d", total, len(items))
	}
}

func TestAssignmentService_GetUserDepartmentIDs(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	svc := newAssignmentService(db)

	role := testutil.SeedRole(t, db, "user")
	dept1 := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	dept2 := testutil.SeedDepartment(t, db, "Product", "prod", nil, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, dept1.ID, pos.ID, true)
	testutil.SeedAssignment(t, db, user.ID, dept2.ID, pos.ID, false)

	ids, err := svc.GetUserDepartmentIDs(user.ID)
	if err != nil {
		t.Fatalf("get user department ids failed: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 ids, got %d", len(ids))
	}
}

func TestAssignmentService_GetUserDepartmentScope(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	svc := newAssignmentService(db)

	role := testutil.SeedRole(t, db, "user")
	parent := testutil.SeedDepartment(t, db, "Parent", "parent", nil, nil, true)
	activeChild := testutil.SeedDepartment(t, db, "ActiveChild", "active_child", &parent.ID, nil, true)
	inactiveChild := testutil.SeedDepartment(t, db, "InactiveChild", "inactive_child", &parent.ID, nil, false)
	_ = testutil.SeedDepartment(t, db, "GrandChild", "grand_child", &activeChild.ID, nil, true)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)
	testutil.SeedAssignment(t, db, user.ID, parent.ID, pos.ID, true)

	scope, err := svc.GetUserDepartmentScope(user.ID)
	if err != nil {
		t.Fatalf("get scope failed: %v", err)
	}
	if len(scope) != 3 {
		t.Fatalf("expected 3 scoped departments, got %d", len(scope))
	}

	// verify inactive branch excluded
	for _, id := range scope {
		if id == inactiveChild.ID {
			t.Fatal("inactive child should not be in scope")
		}
	}

	// no assignments
	u2 := testutil.SeedUser(t, db, "u2", role.ID)
	scope2, err := svc.GetUserDepartmentScope(u2.ID)
	if err != nil {
		t.Fatalf("get scope for no-assignments user failed: %v", err)
	}
	if scope2 != nil {
		t.Fatalf("expected nil scope for user without assignments, got %v", scope2)
	}
}

func TestAssignmentService_SetUserDeptPositions(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	svc := newAssignmentService(db)

	role := testutil.SeedRole(t, db, "user")
	dept := testutil.SeedDepartment(t, db, "Engineering", "eng", nil, nil, true)
	pos1 := testutil.SeedPosition(t, db, "SE", "se", true)
	pos2 := testutil.SeedPosition(t, db, "Manager", "mgr", true)
	pos3 := testutil.SeedPosition(t, db, "Architect", "arch", true)
	user := testutil.SeedUser(t, db, "u1", role.ID)

	// Set initial positions
	err := svc.SetUserDeptPositions(user.ID, dept.ID, []uint{pos1.ID, pos2.ID}, &pos1.ID)
	if err != nil {
		t.Fatalf("set positions failed: %v", err)
	}

	items, _ := svc.repo.FindByUserAndDept(user.ID, dept.ID)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// Update: remove pos2, add pos3, change primary to pos3
	err = svc.SetUserDeptPositions(user.ID, dept.ID, []uint{pos1.ID, pos3.ID}, &pos3.ID)
	if err != nil {
		t.Fatalf("update positions failed: %v", err)
	}

	items, _ = svc.repo.FindByUserAndDept(user.ID, dept.ID)
	if len(items) != 2 {
		t.Fatalf("expected 2 items after update, got %d", len(items))
	}

	// verify primary
	primaryCount := 0
	for _, item := range items {
		if item.IsPrimary {
			primaryCount++
			if item.PositionID != pos3.ID {
				t.Fatalf("expected primary on pos3, got pos %d", item.PositionID)
			}
		}
	}
	if primaryCount != 1 {
		t.Fatalf("expected 1 primary, got %d", primaryCount)
	}

	// Empty positions → removes all
	err = svc.SetUserDeptPositions(user.ID, dept.ID, []uint{}, nil)
	if err != nil {
		t.Fatalf("clear positions failed: %v", err)
	}
	items, _ = svc.repo.FindByUserAndDept(user.ID, dept.ID)
	if len(items) != 0 {
		t.Fatalf("expected 0 items after clear, got %d", len(items))
	}
}

func TestAssignmentService_SetUserDeptPositions_ValidationErrors(t *testing.T) {
	db := testutil.NewOrgTestDB(t)
	svc := newAssignmentService(db)

	role := testutil.SeedRole(t, db, "user")
	user := testutil.SeedUser(t, db, "u1", role.ID)
	pos := testutil.SeedPosition(t, db, "SE", "se", true)

	// dept not found
	err := svc.SetUserDeptPositions(user.ID, 9999, []uint{pos.ID}, nil)
	if err != department.ErrDepartmentNotFound {
		t.Fatalf("expected department.ErrDepartmentNotFound, got %v", err)
	}

	// dept inactive
	inactiveDept := testutil.SeedDepartment(t, db, "Inactive", "inactive", nil, nil, false)
	err = svc.SetUserDeptPositions(user.ID, inactiveDept.ID, []uint{pos.ID}, nil)
	if err != ErrDepartmentInactive {
		t.Fatalf("expected ErrDepartmentInactive, got %v", err)
	}

	// position not found
	activeDept := testutil.SeedDepartment(t, db, "Active", "active", nil, nil, true)
	err = svc.SetUserDeptPositions(user.ID, activeDept.ID, []uint{9999}, nil)
	if err != position.ErrPositionNotFound {
		t.Fatalf("expected position.ErrPositionNotFound, got %v", err)
	}
}
