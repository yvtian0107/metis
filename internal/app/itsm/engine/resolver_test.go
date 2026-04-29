package engine

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestParticipantResolverRequesterReturnsTicketRequester(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&ticketModel{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	ticket := ticketModel{RequesterID: 42, Status: "in_progress"}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	ids, err := NewParticipantResolver(nil).Resolve(db, ticket.ID, Participant{Type: "requester"})
	if err != nil {
		t.Fatalf("resolve requester: %v", err)
	}
	if len(ids) != 1 || ids[0] != 42 {
		t.Fatalf("expected requester id 42, got %+v", ids)
	}
}

func TestParticipantResolverOrgTypesSingleSQLiteConnectionUseWorkflowTransaction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := db.AutoMigrate(&ticketModel{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE users (id integer primary key, username text, is_active boolean, manager_id integer)`).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	if err := db.Exec(`CREATE TABLE positions (id integer primary key, code text)`).Error; err != nil {
		t.Fatalf("create positions: %v", err)
	}
	if err := db.Exec(`CREATE TABLE departments (id integer primary key, code text)`).Error; err != nil {
		t.Fatalf("create departments: %v", err)
	}
	if err := db.Exec(`CREATE TABLE user_positions (user_id integer, position_id integer, department_id integer)`).Error; err != nil {
		t.Fatalf("create user_positions: %v", err)
	}
	if err := db.Exec(`ALTER TABLE user_positions ADD COLUMN deleted_at datetime`).Error; err != nil {
		t.Fatalf("add user_positions.deleted_at: %v", err)
	}
	if err := db.Exec(`INSERT INTO users (id, username, is_active, manager_id) VALUES (7, 'requester', true, 9), (9, 'manager', true, NULL), (10, 'inactive', false, NULL)`).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := db.Exec(`INSERT INTO positions (id, code) VALUES (77, 'network_admin')`).Error; err != nil {
		t.Fatalf("seed positions: %v", err)
	}
	if err := db.Exec(`INSERT INTO departments (id, code) VALUES (88, 'it')`).Error; err != nil {
		t.Fatalf("seed departments: %v", err)
	}
	if err := db.Exec(`INSERT INTO user_positions (user_id, position_id, department_id) VALUES (7, 77, 88), (10, 77, 88)`).Error; err != nil {
		t.Fatalf("seed user positions: %v", err)
	}

	ticket := ticketModel{RequesterID: 7, Status: "in_progress"}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	resolver := NewParticipantResolver(&rootDBPositionResolver{db: db})

	for _, tc := range []struct {
		name string
		p    Participant
		want uint
	}{
		{name: "position", p: Participant{Type: "position", Value: "77"}, want: 7},
		{name: "department", p: Participant{Type: "department", Value: "88"}, want: 7},
		{name: "position_department", p: Participant{Type: "position_department", PositionCode: "network_admin", DepartmentCode: "it"}, want: 7},
		{name: "requester_manager", p: Participant{Type: "requester_manager"}, want: 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			type resolveResult struct {
				ids []uint
				err error
			}
			done := make(chan resolveResult, 1)
			go func() {
				var resolved []uint
				err := db.Transaction(func(tx *gorm.DB) error {
					ids, err := resolver.Resolve(tx, ticket.ID, tc.p)
					if err != nil {
						return err
					}
					resolved = ids
					return nil
				})
				done <- resolveResult{ids: resolved, err: err}
			}()

			select {
			case result := <-done:
				if result.err != nil {
					t.Fatalf("resolve participant: %v", result.err)
				}
				if len(result.ids) != 1 || result.ids[0] != tc.want {
					t.Fatalf("expected participant %d, got %+v", tc.want, result.ids)
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("participant resolver %s blocked with a single SQLite connection", tc.name)
			}
		})
	}
}

func TestParticipantResolverIgnoresSoftDeletedUserPositions(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&ticketModel{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE users (id integer primary key, username text, is_active boolean, manager_id integer)`).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	if err := db.Exec(`CREATE TABLE positions (id integer primary key, code text)`).Error; err != nil {
		t.Fatalf("create positions: %v", err)
	}
	if err := db.Exec(`CREATE TABLE departments (id integer primary key, code text)`).Error; err != nil {
		t.Fatalf("create departments: %v", err)
	}
	if err := db.Exec(`CREATE TABLE user_positions (user_id integer, position_id integer, department_id integer, deleted_at datetime)`).Error; err != nil {
		t.Fatalf("create user_positions: %v", err)
	}
	if err := db.Exec(`INSERT INTO users (id, username, is_active, manager_id) VALUES (1, 'admin', true, NULL), (2, 'ops', true, NULL)`).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := db.Exec(`INSERT INTO positions (id, code) VALUES (77, 'ops_admin')`).Error; err != nil {
		t.Fatalf("seed positions: %v", err)
	}
	if err := db.Exec(`INSERT INTO departments (id, code) VALUES (88, 'it')`).Error; err != nil {
		t.Fatalf("seed departments: %v", err)
	}
	if err := db.Exec(`INSERT INTO user_positions (user_id, position_id, department_id, deleted_at) VALUES (1, 77, 88, CURRENT_TIMESTAMP), (2, 77, 88, NULL)`).Error; err != nil {
		t.Fatalf("seed user positions: %v", err)
	}

	resolver := NewParticipantResolver(nil)
	ids, err := resolver.resolveUsersByPositionAndDepartment(db, "ops_admin", "it")
	if err != nil {
		t.Fatalf("resolve position_department: %v", err)
	}
	if len(ids) != 1 || ids[0] != 2 {
		t.Fatalf("expected only active non-deleted assignment user 2, got %+v", ids)
	}

	ids, err = resolver.resolveUsersByPositionID(db, 77)
	if err != nil {
		t.Fatalf("resolve position: %v", err)
	}
	if len(ids) != 1 || ids[0] != 2 {
		t.Fatalf("expected only active non-deleted position user 2, got %+v", ids)
	}

	ids, err = resolver.resolveUsersByDepartmentID(db, 88)
	if err != nil {
		t.Fatalf("resolve department: %v", err)
	}
	if len(ids) != 1 || ids[0] != 2 {
		t.Fatalf("expected only active non-deleted department user 2, got %+v", ids)
	}
}

func TestParticipantResolverUserIgnoresInactiveAndDeletedUsers(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&ticketModel{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE users (id integer primary key, username text, is_active boolean, deleted_at datetime, manager_id integer)`).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	if err := db.Exec(`INSERT INTO users (id, username, is_active, deleted_at, manager_id) VALUES (1, 'active_user', true, NULL, NULL), (2, 'inactive_user', false, NULL, NULL), (3, 'deleted_user', true, CURRENT_TIMESTAMP, NULL)`).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}

	resolver := NewParticipantResolver(nil)
	ids, err := resolver.Resolve(db, 0, Participant{Type: "user", Value: "active_user"})
	if err != nil {
		t.Fatalf("resolve active user by username: %v", err)
	}
	if len(ids) != 1 || ids[0] != 1 {
		t.Fatalf("expected active user 1, got %+v", ids)
	}

	if _, err := resolver.Resolve(db, 0, Participant{Type: "user", Value: "2"}); err == nil {
		t.Fatal("expected inactive user by id to be rejected")
	}
	if _, err := resolver.Resolve(db, 0, Participant{Type: "user", Value: "deleted_user"}); err == nil {
		t.Fatal("expected soft-deleted user by username to be rejected")
	}
}
