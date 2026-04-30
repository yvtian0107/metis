package service

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	casbinpkg "metis/internal/casbin"
	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/repository"
)

func newTestDBForMenu(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := gdb.AutoMigrate(&model.Menu{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return gdb
}

func newMenuServiceForTest(t *testing.T, db *gorm.DB) *MenuService {
	t.Helper()
	wrapped := &database.DB{DB: db}
	injector := do.New()
	do.ProvideValue(injector, wrapped)

	enforcer, err := casbinpkg.NewEnforcerWithDB(db)
	if err != nil {
		t.Fatalf("create casbin enforcer: %v", err)
	}
	do.ProvideValue(injector, enforcer)

	do.Provide(injector, repository.NewMenu)
	do.Provide(injector, NewCasbin)

	menuRepo := do.MustInvoke[*repository.MenuRepo](injector)
	casbinSvc := do.MustInvoke[*CasbinService](injector)

	return &MenuService{
		menuRepo:  menuRepo,
		casbinSvc: casbinSvc,
	}
}

func seedMenu(t *testing.T, db *gorm.DB, menu *model.Menu) *model.Menu {
	t.Helper()
	if err := db.Create(menu).Error; err != nil {
		t.Fatalf("seed menu: %v", err)
	}
	return menu
}

// 1. Tree Retrieval

func TestMenuServiceGetTree_Sorted(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)

	// Root directories with mixed sort
	_ = seedMenu(t, db, &model.Menu{Name: "B-Dir", Type: model.MenuTypeDirectory, Permission: "test:tree:b-dir", Sort: 2})
	dirA := seedMenu(t, db, &model.Menu{Name: "A-Dir", Type: model.MenuTypeDirectory, Permission: "test:tree:a-dir", Sort: 1})

	// Children under dirA with mixed sort
	seedMenu(t, db, &model.Menu{Name: "A-Child2", Type: model.MenuTypeMenu, ParentID: &dirA.ID, Path: "/a2", Permission: "test:tree:a-child2", Sort: 2})
	seedMenu(t, db, &model.Menu{Name: "A-Child1", Type: model.MenuTypeMenu, ParentID: &dirA.ID, Path: "/a1", Permission: "test:tree:a-child1", Sort: 1})

	tree, err := svc.GetTree()
	if err != nil {
		t.Fatalf("get tree: %v", err)
	}
	if len(tree) != 2 {
		t.Fatalf("expected 2 root nodes, got %d", len(tree))
	}
	if tree[0].Name != "A-Dir" {
		t.Fatalf("expected first root A-Dir, got %s", tree[0].Name)
	}
	if tree[1].Name != "B-Dir" {
		t.Fatalf("expected second root B-Dir, got %s", tree[1].Name)
	}
	if len(tree[0].Children) != 2 {
		t.Fatalf("expected 2 children under A-Dir, got %d", len(tree[0].Children))
	}
	if tree[0].Children[0].Name != "A-Child1" {
		t.Fatalf("expected first child A-Child1, got %s", tree[0].Children[0].Name)
	}
	if tree[0].Children[1].Name != "A-Child2" {
		t.Fatalf("expected second child A-Child2, got %s", tree[0].Children[1].Name)
	}
}

// 2. User Tree

func TestMenuServiceGetUserTree_AdminGetsFullTree(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)

	dir := seedMenu(t, db, &model.Menu{Name: "System", Type: model.MenuTypeDirectory, Permission: "test:admin:dir", Sort: 1})
	seedMenu(t, db, &model.Menu{Name: "Users", Type: model.MenuTypeMenu, ParentID: &dir.ID, Path: "/users", Permission: "system:user:list", Sort: 1})

	tree, err := svc.GetUserTree(model.RoleAdmin)
	if err != nil {
		t.Fatalf("get user tree: %v", err)
	}
	if len(tree) != 1 {
		t.Fatalf("expected 1 root for admin, got %d", len(tree))
	}
	if len(tree[0].Children) != 1 {
		t.Fatalf("expected 1 child for admin, got %d", len(tree[0].Children))
	}
}

func TestMenuServiceGetUserTree_RoleSeesOnlyPermittedMenus(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)

	dir := seedMenu(t, db, &model.Menu{Name: "System", Type: model.MenuTypeDirectory, Permission: "test:role:dir", Sort: 1})
	userMenu := seedMenu(t, db, &model.Menu{Name: "Users", Type: model.MenuTypeMenu, ParentID: &dir.ID, Path: "/users", Permission: "system:user:list", Sort: 1})
	seedMenu(t, db, &model.Menu{Name: "Roles", Type: model.MenuTypeMenu, ParentID: &dir.ID, Path: "/roles", Permission: "system:role:list", Sort: 2})
	seedMenu(t, db, &model.Menu{Name: "AddUser", Type: model.MenuTypeButton, ParentID: &userMenu.ID, Permission: "system:user:create", Sort: 1})

	// Grant only system:user:list
	if err := svc.casbinSvc.SetPoliciesForRole("editor", [][]string{{"editor", "system:user:list", "GET"}}); err != nil {
		t.Fatalf("set policies: %v", err)
	}

	tree, err := svc.GetUserTree("editor")
	if err != nil {
		t.Fatalf("get user tree: %v", err)
	}
	if len(tree) != 1 || tree[0].Name != "System" {
		t.Fatalf("expected System directory, got %v", tree)
	}
	if len(tree[0].Children) != 1 || tree[0].Children[0].Name != "Users" {
		t.Fatalf("expected Users menu only, got %v", tree[0].Children)
	}
	// AddUser button should be excluded because role lacks system:user:create
	if len(tree[0].Children[0].Children) != 0 {
		t.Fatalf("expected no button children, got %v", tree[0].Children[0].Children)
	}
}

func TestMenuServiceGetUserTree_ParentDirectoryRetained(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)

	dir := seedMenu(t, db, &model.Menu{Name: "System", Type: model.MenuTypeDirectory, Permission: "test:parent:dir", Sort: 1})
	userMenu := seedMenu(t, db, &model.Menu{Name: "Users", Type: model.MenuTypeMenu, ParentID: &dir.ID, Path: "/users", Permission: "system:user:list", Sort: 1})
	seedMenu(t, db, &model.Menu{Name: "AddUser", Type: model.MenuTypeButton, ParentID: &userMenu.ID, Permission: "system:user:create", Sort: 1})

	// Grant only the button permission, not the directory or menu
	if err := svc.casbinSvc.SetPoliciesForRole("editor", [][]string{{"editor", "system:user:create", "GET"}}); err != nil {
		t.Fatalf("set policies: %v", err)
	}

	tree, err := svc.GetUserTree("editor")
	if err != nil {
		t.Fatalf("get user tree: %v", err)
	}
	if len(tree) != 1 || tree[0].Name != "System" {
		t.Fatalf("expected System directory retained, got %v", tree)
	}
	if len(tree[0].Children) != 1 || tree[0].Children[0].Name != "Users" {
		t.Fatalf("expected Users menu retained, got %v", tree[0].Children)
	}
	if len(tree[0].Children[0].Children) != 1 || tree[0].Children[0].Children[0].Name != "AddUser" {
		t.Fatalf("expected AddUser button, got %v", tree[0].Children[0].Children)
	}
}

func TestMenuServiceGetUserTree_HiddenMenusIncluded(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)

	dir := seedMenu(t, db, &model.Menu{Name: "System", Type: model.MenuTypeDirectory, Permission: "test:hidden:dir", Sort: 1})
	seedMenu(t, db, &model.Menu{Name: "HiddenMenu", Type: model.MenuTypeMenu, ParentID: &dir.ID, Path: "/hidden", Permission: "system:hidden:list", IsHidden: true, Sort: 1})

	if err := svc.casbinSvc.SetPoliciesForRole("editor", [][]string{{"editor", "system:hidden:list", "GET"}}); err != nil {
		t.Fatalf("set policies: %v", err)
	}

	tree, err := svc.GetUserTree("editor")
	if err != nil {
		t.Fatalf("get user tree: %v", err)
	}
	if len(tree) != 1 || len(tree[0].Children) != 1 {
		t.Fatalf("expected hidden menu to be included, got %v", tree)
	}
	if !tree[0].Children[0].IsHidden {
		t.Fatal("expected IsHidden=true to be preserved")
	}
}

// 3. Permissions List

func TestMenuServiceGetUserPermissions_ReturnsPermissions(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)

	// Grant a mix of API paths and permission identifiers
	policies := [][]string{
		{"editor", "system:user:list", "GET"},
		{"editor", "system:user:create", "POST"},
		{"editor", "/api/v1/users", "GET"},
		{"editor", "system:user:create", "POST"}, // duplicate
	}
	if err := svc.casbinSvc.SetPoliciesForRole("editor", policies); err != nil {
		t.Fatalf("set policies: %v", err)
	}

	perms := svc.GetUserPermissions("editor")
	if len(perms) != 2 {
		t.Fatalf("expected 2 unique permissions, got %d: %v", len(perms), perms)
	}
	seen := map[string]bool{}
	for _, p := range perms {
		seen[p] = true
	}
	if !seen["system:user:list"] || !seen["system:user:create"] {
		t.Fatalf("expected system:user:list and system:user:create, got %v", perms)
	}
}

// 4. CRUD

func TestMenuServiceCreate_Success(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)

	parent := seedMenu(t, db, &model.Menu{Name: "System", Type: model.MenuTypeDirectory, Permission: "test:create:parent", Sort: 1})
	menu := &model.Menu{
		Name:       "Logs",
		Type:       model.MenuTypeMenu,
		ParentID:   &parent.ID,
		Path:       "/logs",
		Permission: "system:log:list",
		Sort:       10,
	}
	if err := svc.Create(menu); err != nil {
		t.Fatalf("create: %v", err)
	}
	if menu.ID == 0 {
		t.Fatal("expected menu ID to be set")
	}
	if menu.Path != "/logs" {
		t.Fatalf("expected path /logs, got %s", menu.Path)
	}
}

func TestMenuServiceCreate_RootDirectory(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)

	menu := &model.Menu{
		Name:       "System",
		Type:       model.MenuTypeDirectory,
		Permission: "test:create:root",
		Sort:       1,
	}
	if err := svc.Create(menu); err != nil {
		t.Fatalf("create: %v", err)
	}
	if menu.ParentID != nil {
		t.Fatalf("expected nil parentID, got %v", *menu.ParentID)
	}
}

func TestMenuServiceCreate_RejectsInvalidType(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)

	err := svc.Create(&model.Menu{
		Name:       "Invalid",
		Type:       model.MenuType("invalid"),
		Permission: "test:create:invalid-type",
	})
	if !errors.Is(err, ErrMenuInvalidType) {
		t.Fatalf("expected ErrMenuInvalidType, got %v", err)
	}
}

func TestMenuServiceCreate_RejectsDuplicatePermission(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)

	seedMenu(t, db, &model.Menu{Name: "Old", Type: model.MenuTypeMenu, Permission: "test:create:duplicate", Sort: 1})
	err := svc.Create(&model.Menu{
		Name:       "New",
		Type:       model.MenuTypeMenu,
		Permission: "test:create:duplicate",
	})
	if !errors.Is(err, ErrMenuPermissionExists) {
		t.Fatalf("expected ErrMenuPermissionExists, got %v", err)
	}
}

func TestMenuServiceCreate_RejectsUnknownParent(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)

	missingParentID := uint(999)
	err := svc.Create(&model.Menu{
		Name:       "Child",
		Type:       model.MenuTypeMenu,
		ParentID:   &missingParentID,
		Permission: "test:create:unknown-parent",
	})
	if err == nil {
		t.Fatal("expected create to fail when parent does not exist")
	}
}

func TestMenuServiceCreate_RejectsButtonParent(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)

	button := seedMenu(t, db, &model.Menu{Name: "按钮权限", Type: model.MenuTypeButton, Permission: "test:create:button-parent", Sort: 1})
	err := svc.Create(&model.Menu{
		Name:       "非法子项",
		Type:       model.MenuTypeMenu,
		ParentID:   &button.ID,
		Permission: "test:create:child-under-button",
	})
	if err == nil {
		t.Fatal("expected create to fail when parent is a button")
	}
	
}

func TestMenuServiceUpdate_Success(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)
	menu := seedMenu(t, db, &model.Menu{Name: "OldName", Type: model.MenuTypeMenu, Path: "/old", Permission: "test:update:success", Sort: 1})

	updated, err := svc.Update(menu.ID, map[string]any{
		"name": "NewName",
		"sort": float64(99),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "NewName" {
		t.Fatalf("expected name NewName, got %s", updated.Name)
	}
	if updated.Sort != 99 {
		t.Fatalf("expected sort 99, got %d", updated.Sort)
	}
	if updated.Path != "/old" {
		t.Fatalf("expected path unchanged, got %s", updated.Path)
	}
}

func TestMenuServiceUpdate_Parent(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)
	oldParent := seedMenu(t, db, &model.Menu{Name: "OldParent", Type: model.MenuTypeDirectory, Permission: "test:update:old-parent", Sort: 1})
	newParent := seedMenu(t, db, &model.Menu{Name: "NewParent", Type: model.MenuTypeDirectory, Permission: "test:update:new-parent", Sort: 2})
	menu := seedMenu(t, db, &model.Menu{Name: "Child", Type: model.MenuTypeMenu, ParentID: &oldParent.ID, Path: "/child", Permission: "test:update:child", Sort: 1})

	updated, err := svc.Update(menu.ID, map[string]any{
		"parentId": float64(newParent.ID),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.ParentID == nil || *updated.ParentID != newParent.ID {
		t.Fatalf("expected parentID %d, got %v", newParent.ID, updated.ParentID)
	}
}

func TestMenuServiceUpdate_ClearParent(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)

	parent := seedMenu(t, db, &model.Menu{Name: "Parent", Type: model.MenuTypeDirectory, Permission: "test:update:clear-parent-parent", Sort: 1})
	menu := seedMenu(t, db, &model.Menu{Name: "Child", Type: model.MenuTypeMenu, ParentID: &parent.ID, Permission: "test:update:clear-parent-child", Sort: 1})

	updated, err := svc.Update(menu.ID, map[string]any{"parentId": nil})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.ParentID != nil {
		t.Fatalf("expected parentID nil, got %v", updated.ParentID)
	}
}

func TestMenuServiceUpdate_RejectsInvalidType(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)

	menu := seedMenu(t, db, &model.Menu{Name: "Menu", Type: model.MenuTypeMenu, Permission: "test:update:invalid-type", Sort: 1})

	_, err := svc.Update(menu.ID, map[string]any{"type": "invalid"})
	if !errors.Is(err, ErrMenuInvalidType) {
		t.Fatalf("expected ErrMenuInvalidType, got %v", err)
	}
}

func TestMenuServiceUpdate_RejectsDuplicatePermission(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)

	seedMenu(t, db, &model.Menu{Name: "MenuA", Type: model.MenuTypeMenu, Permission: "test:update:duplicate-a", Sort: 1})
	menuB := seedMenu(t, db, &model.Menu{Name: "MenuB", Type: model.MenuTypeMenu, Permission: "test:update:duplicate-b", Sort: 2})

	_, err := svc.Update(menuB.ID, map[string]any{"permission": "test:update:duplicate-a"})
	if !errors.Is(err, ErrMenuPermissionExists) {
		t.Fatalf("expected ErrMenuPermissionExists, got %v", err)
	}
}

func TestMenuServiceUpdate_RejectsButtonParent(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)

	button := seedMenu(t, db, &model.Menu{Name: "Button", Type: model.MenuTypeButton, Permission: "test:update:button-parent", Sort: 1})
	menu := seedMenu(t, db, &model.Menu{Name: "Menu", Type: model.MenuTypeMenu, Permission: "test:update:button-parent-child", Sort: 2})

	_, err := svc.Update(menu.ID, map[string]any{"parentId": float64(button.ID)})
	if !errors.Is(err, ErrMenuParentNotAllowed) {
		t.Fatalf("expected ErrMenuParentNotAllowed, got %v", err)
	}
}

func TestMenuServiceUpdate_NotFound(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)

	_, err := svc.Update(999, map[string]any{"name": "X"})
	if !errors.Is(err, ErrMenuNotFound) {
		t.Fatalf("expected ErrMenuNotFound, got %v", err)
	}
}

func TestMenuServiceUpdate_RejectsDescendantParent(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)

	root := seedMenu(t, db, &model.Menu{Name: "Root", Type: model.MenuTypeDirectory, Permission: "test:update:cycle-root", Sort: 1})
	child := seedMenu(t, db, &model.Menu{Name: "Child", Type: model.MenuTypeMenu, ParentID: &root.ID, Permission: "test:update:cycle-child", Sort: 1})

	_, err := svc.Update(root.ID, map[string]any{"parentId": float64(child.ID)})
	if err == nil {
		t.Fatal("expected update to fail when moving a node under its descendant")
	}
	
}

func TestMenuServiceReorderMenus_Success(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)
	m1 := seedMenu(t, db, &model.Menu{Name: "M1", Type: model.MenuTypeMenu, Permission: "test:reorder:m1", Sort: 1})
	m2 := seedMenu(t, db, &model.Menu{Name: "M2", Type: model.MenuTypeMenu, Permission: "test:reorder:m2", Sort: 2})

	if err := svc.ReorderMenus([]repository.SortItem{
		{ID: m1.ID, Sort: 20},
		{ID: m2.ID, Sort: 10},
	}); err != nil {
		t.Fatalf("reorder: %v", err)
	}

	found1, _ := svc.menuRepo.FindByID(m1.ID)
	found2, _ := svc.menuRepo.FindByID(m2.ID)
	if found1.Sort != 20 {
		t.Fatalf("expected m1 sort 20, got %d", found1.Sort)
	}
	if found2.Sort != 10 {
		t.Fatalf("expected m2 sort 10, got %d", found2.Sort)
	}
}

func TestMenuServiceReorderMenus_RejectsUnknownID(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)

	seedMenu(t, db, &model.Menu{Name: "M1", Type: model.MenuTypeMenu, Permission: "test:reorder:unknown-existing", Sort: 1})
	err := svc.ReorderMenus([]repository.SortItem{{ID: 999, Sort: 10}})
	if !errors.Is(err, ErrMenuNotFound) {
		t.Fatalf("expected ErrMenuNotFound, got %v", err)
	}
}

func TestMenuServiceDelete_LeafMenu(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)
	menu := seedMenu(t, db, &model.Menu{Name: "Leaf", Type: model.MenuTypeMenu, Permission: "test:delete:leaf", Sort: 1})

	if err := svc.Delete(menu.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := svc.menuRepo.FindByID(menu.ID)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found after delete, got %v", err)
	}
}

func TestMenuServiceDelete_PreventsChildren(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)
	parent := seedMenu(t, db, &model.Menu{Name: "Parent", Type: model.MenuTypeDirectory, Permission: "test:delete:parent", Sort: 1})
	seedMenu(t, db, &model.Menu{Name: "Child", Type: model.MenuTypeMenu, ParentID: &parent.ID, Permission: "test:delete:child", Sort: 1})

	err := svc.Delete(parent.ID)
	if !errors.Is(err, ErrMenuHasChildren) {
		t.Fatalf("expected ErrMenuHasChildren, got %v", err)
	}
}

func TestMenuServiceDelete_NotFound(t *testing.T) {
	db := newTestDBForMenu(t)
	svc := newMenuServiceForTest(t, db)

	err := svc.Delete(999)
	if !errors.Is(err, ErrMenuNotFound) {
		t.Fatalf("expected ErrMenuNotFound, got %v", err)
	}
}
