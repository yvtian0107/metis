package engine

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupVarWriterDB creates an in-memory SQLite database with the
// itsm_process_variables table and its unique index ready for testing.
func setupVarWriterDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:varwriter_%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&processVariableModel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Create the unique index that clause.OnConflict relies on.
	if err := db.Exec(
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_proc_var_upsert ON itsm_process_variables (ticket_id, scope_id, key)`,
	).Error; err != nil {
		t.Fatalf("create unique index: %v", err)
	}
	return db
}

// countVariables returns the number of process variable rows for a ticket.
func countVariables(t *testing.T, db *gorm.DB, ticketID uint) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&processVariableModel{}).Where("ticket_id = ?", ticketID).Count(&count).Error; err != nil {
		t.Fatalf("count variables: %v", err)
	}
	return count
}

// lookupVariable fetches one process variable by ticket+scope+key.
func lookupVariable(t *testing.T, db *gorm.DB, ticketID uint, scopeID, key string) *processVariableModel {
	t.Helper()
	var v processVariableModel
	err := db.Where("ticket_id = ? AND scope_id = ? AND key = ?", ticketID, scopeID, key).First(&v).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		t.Fatalf("lookup variable %s: %v", key, err)
	}
	return &v
}

// ---------- Task 1.4: Form validation tests ----------

func TestWriteFormBindings_ValidData_WritesVariables(t *testing.T) {
	db := setupVarWriterDB(t)

	schema := `{"fields":[
		{"key":"name","type":"text","label":"姓名","required":true,"binding":"var_name"},
		{"key":"age","type":"number","label":"年龄","binding":"var_age"}
	]}`
	data := `{"name":"张三","age":25}`

	err := writeFormBindings(db, 1, "root", schema, data, "form_submit", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if n := countVariables(t, db, 1); n != 2 {
		t.Fatalf("expected 2 variables written, got %d", n)
	}

	v := lookupVariable(t, db, 1, "root", "var_name")
	if v == nil {
		t.Fatalf("expected var_name to exist")
	}
	if v.Value != "张三" {
		t.Fatalf("expected var_name = 张三, got %s", v.Value)
	}
	if v.ValueType != "string" {
		t.Fatalf("expected value_type = string, got %s", v.ValueType)
	}
	if v.Source != "form_submit" {
		t.Fatalf("expected source = form_submit, got %s", v.Source)
	}

	vAge := lookupVariable(t, db, 1, "root", "var_age")
	if vAge == nil {
		t.Fatalf("expected var_age to exist")
	}
	if vAge.Value != "25" {
		t.Fatalf("expected var_age = 25, got %s", vAge.Value)
	}
	if vAge.ValueType != "number" {
		t.Fatalf("expected value_type = number, got %s", vAge.ValueType)
	}
}

func TestWriteFormBindings_InvalidData_ReturnsFormValidationError(t *testing.T) {
	db := setupVarWriterDB(t)

	// "name" is required but missing from data
	schema := `{"fields":[
		{"key":"name","type":"text","label":"姓名","required":true,"binding":"var_name"},
		{"key":"age","type":"number","label":"年龄","binding":"var_age"}
	]}`
	data := `{"age":30}`

	err := writeFormBindings(db, 2, "root", schema, data, "form_submit", "")
	if err == nil {
		t.Fatalf("expected error for missing required field, got nil")
	}

	var fve *FormValidationError
	if !errors.As(err, &fve) {
		t.Fatalf("expected *FormValidationError, got %T: %v", err, err)
	}
	if len(fve.Errors) == 0 {
		t.Fatalf("expected at least one validation error")
	}

	// Atomic rejection: NO variables should be written
	if n := countVariables(t, db, 2); n != 0 {
		t.Fatalf("expected 0 variables (atomic rejection), got %d", n)
	}
}

func TestWriteFormBindings_PartialInvalid_EntireBatchRejected(t *testing.T) {
	db := setupVarWriterDB(t)

	// Two fields: "name" is required (will be present), "email" is required (will be missing).
	// Even though "name" is valid, the entire batch must be rejected.
	schema := `{"fields":[
		{"key":"name","type":"text","label":"姓名","required":true,"binding":"var_name"},
		{"key":"email","type":"text","label":"邮箱","required":true,"binding":"var_email"}
	]}`
	data := `{"name":"李四"}`

	err := writeFormBindings(db, 3, "root", schema, data, "form_submit", "")
	if err == nil {
		t.Fatalf("expected FormValidationError, got nil")
	}

	var fve *FormValidationError
	if !errors.As(err, &fve) {
		t.Fatalf("expected *FormValidationError, got %T: %v", err, err)
	}

	// Neither field should be written
	if n := countVariables(t, db, 3); n != 0 {
		t.Fatalf("expected 0 variables (entire batch rejected), got %d", n)
	}
}

// ---------- Task 5.2: Permission enforcement tests ----------

func TestWriteFormBindings_EditablePermission_Written(t *testing.T) {
	db := setupVarWriterDB(t)

	schema := `{"fields":[
		{"key":"name","type":"text","label":"姓名","binding":"var_name",
		 "permissions":{"node1":"editable"}}
	]}`
	data := `{"name":"王五"}`

	err := writeFormBindings(db, 10, "root", schema, data, "form_submit", "node1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	v := lookupVariable(t, db, 10, "root", "var_name")
	if v == nil {
		t.Fatalf("expected var_name to be written for editable permission")
	}
	if v.Value != "王五" {
		t.Fatalf("expected var_name = 王五, got %s", v.Value)
	}
}

func TestWriteFormBindings_ReadonlyPermission_Skipped(t *testing.T) {
	db := setupVarWriterDB(t)

	schema := `{"fields":[
		{"key":"name","type":"text","label":"姓名","binding":"var_name",
		 "permissions":{"node1":"readonly","node2":"editable"}}
	]}`
	data := `{"name":"赵六"}`

	// currentNodeID = "node1" which is readonly
	err := writeFormBindings(db, 11, "root", schema, data, "form_submit", "node1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	v := lookupVariable(t, db, 11, "root", "var_name")
	if v != nil {
		t.Fatalf("expected var_name to be skipped for readonly permission, but found value %q", v.Value)
	}
}

func TestWriteFormBindings_HiddenPermission_Skipped(t *testing.T) {
	db := setupVarWriterDB(t)

	schema := `{"fields":[
		{"key":"name","type":"text","label":"姓名","binding":"var_name",
		 "permissions":{"node1":"hidden"}}
	]}`
	data := `{"name":"孙七"}`

	err := writeFormBindings(db, 12, "root", schema, data, "form_submit", "node1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	v := lookupVariable(t, db, 12, "root", "var_name")
	if v != nil {
		t.Fatalf("expected var_name to be skipped for hidden permission, but found value %q", v.Value)
	}
}

func TestWriteFormBindings_NoPermissionsMap_BackwardsCompatible(t *testing.T) {
	db := setupVarWriterDB(t)

	// No "permissions" key at all — should write normally regardless of currentNodeID.
	schema := `{"fields":[
		{"key":"name","type":"text","label":"姓名","binding":"var_name"}
	]}`
	data := `{"name":"周八"}`

	err := writeFormBindings(db, 13, "root", schema, data, "form_submit", "node1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	v := lookupVariable(t, db, 13, "root", "var_name")
	if v == nil {
		t.Fatalf("expected var_name to be written when no permissions map exists")
	}
	if v.Value != "周八" {
		t.Fatalf("expected var_name = 周八, got %s", v.Value)
	}
}

// ---------- Additional edge-case coverage ----------

func TestWriteFormBindings_PermissionMixedFields(t *testing.T) {
	db := setupVarWriterDB(t)

	// Two fields: one editable, one readonly for the current node.
	// Only the editable one should be written.
	schema := `{"fields":[
		{"key":"title","type":"text","label":"标题","binding":"var_title",
		 "permissions":{"nodeA":"editable"}},
		{"key":"status","type":"text","label":"状态","binding":"var_status",
		 "permissions":{"nodeA":"readonly"}}
	]}`
	data := `{"title":"测试标题","status":"进行中"}`

	err := writeFormBindings(db, 14, "root", schema, data, "form_submit", "nodeA")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	vTitle := lookupVariable(t, db, 14, "root", "var_title")
	if vTitle == nil {
		t.Fatalf("expected var_title to be written (editable)")
	}
	if vTitle.Value != "测试标题" {
		t.Fatalf("expected var_title = 测试标题, got %s", vTitle.Value)
	}

	vStatus := lookupVariable(t, db, 14, "root", "var_status")
	if vStatus != nil {
		t.Fatalf("expected var_status to be skipped (readonly), but found value %q", vStatus.Value)
	}
}

func TestWriteFormBindings_EmptyCurrentNodeID_SkipsPermissionCheck(t *testing.T) {
	db := setupVarWriterDB(t)

	// Even though the field has readonly permission for some node,
	// an empty currentNodeID means permission checks are skipped entirely.
	schema := `{"fields":[
		{"key":"name","type":"text","label":"姓名","binding":"var_name",
		 "permissions":{"node1":"readonly"}}
	]}`
	data := `{"name":"钱九"}`

	err := writeFormBindings(db, 15, "root", schema, data, "form_submit", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	v := lookupVariable(t, db, 15, "root", "var_name")
	if v == nil {
		t.Fatalf("expected var_name to be written when currentNodeID is empty")
	}
	if v.Value != "钱九" {
		t.Fatalf("expected var_name = 钱九, got %s", v.Value)
	}
}

func TestWriteFormBindings_FieldWithoutBinding_Skipped(t *testing.T) {
	db := setupVarWriterDB(t)

	// "desc" has no binding — it should not produce a variable row.
	schema := `{"fields":[
		{"key":"name","type":"text","label":"姓名","binding":"var_name"},
		{"key":"desc","type":"text","label":"描述"}
	]}`
	data := `{"name":"吴十","desc":"一些描述"}`

	err := writeFormBindings(db, 16, "root", schema, data, "form_submit", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if n := countVariables(t, db, 16); n != 1 {
		t.Fatalf("expected 1 variable (only bound field), got %d", n)
	}
}

func TestWriteFormBindings_Upsert_UpdatesExistingVariable(t *testing.T) {
	db := setupVarWriterDB(t)

	schema := `{"fields":[
		{"key":"name","type":"text","label":"姓名","binding":"var_name"}
	]}`

	// First write
	if err := writeFormBindings(db, 20, "root", schema, `{"name":"初始值"}`, "first", ""); err != nil {
		t.Fatalf("first write: %v", err)
	}
	v1 := lookupVariable(t, db, 20, "root", "var_name")
	if v1 == nil || v1.Value != "初始值" {
		t.Fatalf("expected 初始值 after first write")
	}

	// Second write — should update, not insert a duplicate
	if err := writeFormBindings(db, 20, "root", schema, `{"name":"更新值"}`, "second", ""); err != nil {
		t.Fatalf("second write: %v", err)
	}

	if n := countVariables(t, db, 20); n != 1 {
		t.Fatalf("expected 1 variable after upsert, got %d", n)
	}
	v2 := lookupVariable(t, db, 20, "root", "var_name")
	if v2 == nil {
		t.Fatalf("expected var_name to exist after upsert")
	}
	if v2.Value != "更新值" {
		t.Fatalf("expected var_name = 更新值 after upsert, got %s", v2.Value)
	}
	if v2.Source != "second" {
		t.Fatalf("expected source = second after upsert, got %s", v2.Source)
	}
}

func TestWriteFormBindings_EmptyInputs_NilReturn(t *testing.T) {
	db := setupVarWriterDB(t)

	// Empty schema or data should return nil without error.
	if err := writeFormBindings(db, 30, "root", "", `{"name":"test"}`, "src", ""); err != nil {
		t.Fatalf("empty schema: expected nil, got %v", err)
	}
	if err := writeFormBindings(db, 30, "root", `{"fields":[]}`, "", "src", ""); err != nil {
		t.Fatalf("empty data: expected nil, got %v", err)
	}
	if n := countVariables(t, db, 30); n != 0 {
		t.Fatalf("expected 0 variables for empty inputs, got %d", n)
	}
}

func TestWriteFormBindings_FormValidationError_ErrorMethod(t *testing.T) {
	fve := &FormValidationError{}
	// Verify the error interface contract
	var _ error = fve
	msg := fve.Error()
	if msg == "" {
		t.Fatalf("expected non-empty error message")
	}
}

func TestWriteFormBindings_PermissionNodeNotInMap_Written(t *testing.T) {
	db := setupVarWriterDB(t)

	// The permissions map exists but does not contain an entry for the
	// currentNodeID — this means the field is effectively unrestricted
	// for that node, so it should be written.
	schema := `{"fields":[
		{"key":"name","type":"text","label":"姓名","binding":"var_name",
		 "permissions":{"node_other":"readonly"}}
	]}`
	data := `{"name":"郑十一"}`

	err := writeFormBindings(db, 17, "root", schema, data, "form_submit", "node_current")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	v := lookupVariable(t, db, 17, "root", "var_name")
	if v == nil {
		t.Fatalf("expected var_name to be written when node not in permissions map")
	}
	if v.Value != "郑十一" {
		t.Fatalf("expected var_name = 郑十一, got %s", v.Value)
	}
}
