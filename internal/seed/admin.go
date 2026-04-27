package seed

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"metis/internal/model"
	"metis/internal/pkg/token"
)

type UserOrgIdentity struct {
	DeptCode string
	PosCode  string
	Primary  bool
}

// UpsertLocalUser creates or updates a local password user.
func UpsertLocalUser(db *gorm.DB, username, password, email string, roleID uint) error {
	hashed, err := token.HashPassword(password)
	if err != nil {
		return err
	}

	now := time.Now()
	updates := map[string]any{
		"password":            hashed,
		"email":               email,
		"role_id":             roleID,
		"is_active":           true,
		"password_changed_at": &now,
	}

	var existing model.User
	if err := db.Where("username = ?", username).First(&existing).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		user := model.User{
			Username:          username,
			Password:          hashed,
			Email:             email,
			RoleID:            roleID,
			IsActive:          true,
			PasswordChangedAt: &now,
		}
		return db.Create(&user).Error
	}

	return db.Model(&model.User{}).Where("id = ?", existing.ID).Updates(updates).Error
}

// UpsertInstallAdmin creates or updates the development/install admin account.
func UpsertInstallAdmin(db *gorm.DB, username, password, email string, roleID uint) error {
	return UpsertLocalUser(db, username, password, email, roleID)
}

func AssignUserOrgIdentities(db *gorm.DB, username string, identities []UserOrgIdentity) error {
	var user struct{ ID uint }
	if err := db.Table("users").Where("username = ?", username).Select("id").First(&user).Error; err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var primaryReady bool
		for _, identity := range identities {
			if !identity.Primary {
				continue
			}
			if _, _, ok, err := lookupInstallOrgIdentity(tx, identity.DeptCode, identity.PosCode); err != nil {
				return err
			} else if ok {
				primaryReady = true
			}
		}
		if primaryReady {
			if err := tx.Table("user_positions").Where("user_id = ?", user.ID).Update("is_primary", false).Error; err != nil {
				return err
			}
		}

		for _, identity := range identities {
			deptID, posID, ok, err := lookupInstallOrgIdentity(tx, identity.DeptCode, identity.PosCode)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}

			type userPositionRow struct {
				ID        uint
				IsPrimary bool
			}
			var existing userPositionRow
			err = tx.Table("user_positions").
				Where("user_id = ? AND department_id = ? AND position_id = ?", user.ID, deptID, posID).
				Select("id, is_primary").
				First(&existing).Error
			switch {
			case err == nil:
				if existing.IsPrimary != identity.Primary {
					if err := tx.Table("user_positions").Where("id = ?", existing.ID).Update("is_primary", identity.Primary).Error; err != nil {
						return err
					}
				}
			case errors.Is(err, gorm.ErrRecordNotFound):
				if err := tx.Table("user_positions").Create(map[string]any{
					"user_id":       user.ID,
					"department_id": deptID,
					"position_id":   posID,
					"is_primary":    identity.Primary,
				}).Error; err != nil {
					return err
				}
			default:
				return err
			}
		}
		return nil
	})
}

// AssignInstallAdminOrgIdentity binds the install admin to built-in IT org posts.
func AssignInstallAdminOrgIdentity(db *gorm.DB, username string) error {
	return AssignUserOrgIdentities(db, username, []UserOrgIdentity{
		{DeptCode: "it", PosCode: "it_admin", Primary: true},
		{DeptCode: "it", PosCode: "db_admin"},
		{DeptCode: "it", PosCode: "network_admin"},
		{DeptCode: "it", PosCode: "security_admin"},
		{DeptCode: "it", PosCode: "ops_admin"},
		{DeptCode: "headquarters", PosCode: "serial_reviewer"},
	})
}

func lookupInstallOrgIdentity(db *gorm.DB, deptCode, posCode string) (uint, uint, bool, error) {
	type row struct{ ID uint }
	var dept row
	if err := db.Table("departments").Where("code = ?", deptCode).Select("id").First(&dept).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, 0, false, nil
		}
		return 0, 0, false, err
	}
	var pos row
	if err := db.Table("positions").Where("code = ?", posCode).Select("id").First(&pos).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, 0, false, nil
		}
		return 0, 0, false, err
	}
	return dept.ID, pos.ID, true, nil
}
