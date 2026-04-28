package support

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"

	"metis/internal/app"
	"metis/internal/app/itsm/engine"
	org "metis/internal/app/org/domain"
	"metis/internal/model"
)

type noopSubmitter struct{}

func (n *noopSubmitter) SubmitTask(string, json.RawMessage) error { return nil }

var _ engine.TaskSubmitter = (*noopSubmitter)(nil)

type noopDecisionExecutor struct{}

func (n noopDecisionExecutor) Execute(context.Context, uint, app.AIDecisionRequest) (*app.AIDecisionResponse, error) {
	return nil, fmt.Errorf("api bdd noop decision executor should not be used directly")
}

var _ app.AIDecisionExecutor = noopDecisionExecutor{}

type OrgResolver struct {
	db *gorm.DB
}

func (s *OrgResolver) GetUserDeptScope(_ uint, _ bool) ([]uint, error) { return nil, nil }
func (s *OrgResolver) GetUserPositionIDs(userID uint) ([]uint, error) {
	var rows []org.UserPosition
	if err := s.db.Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.PositionID)
	}
	return ids, nil
}
func (s *OrgResolver) GetUserDepartmentIDs(userID uint) ([]uint, error) {
	var rows []org.UserPosition
	if err := s.db.Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.DepartmentID)
	}
	return ids, nil
}
func (s *OrgResolver) GetUserPositions(uint) ([]app.OrgPosition, error)   { return nil, nil }
func (s *OrgResolver) GetUserDepartment(uint) (*app.OrgDepartment, error) { return nil, nil }
func (s *OrgResolver) QueryContext(_, _, _ string, _ bool) (*app.OrgContextResult, error) {
	return nil, nil
}
func (s *OrgResolver) FindUsersByPositionCode(posCode string) ([]uint, error) {
	var pos org.Position
	if err := s.db.Where("code = ?", posCode).First(&pos).Error; err != nil {
		return nil, err
	}
	return s.FindUsersByPositionID(pos.ID)
}
func (s *OrgResolver) FindUsersByDepartmentCode(deptCode string) ([]uint, error) {
	var dept org.Department
	if err := s.db.Where("code = ?", deptCode).First(&dept).Error; err != nil {
		return nil, err
	}
	return s.FindUsersByDepartmentID(dept.ID)
}
func (s *OrgResolver) FindUsersByPositionAndDepartment(posCode, deptCode string) ([]uint, error) {
	var pos org.Position
	if err := s.db.Where("code = ?", posCode).First(&pos).Error; err != nil {
		return nil, err
	}
	var dept org.Department
	if err := s.db.Where("code = ?", deptCode).First(&dept).Error; err != nil {
		return nil, err
	}
	var rows []org.UserPosition
	if err := s.db.Where("position_id = ? AND department_id = ?", pos.ID, dept.ID).Find(&rows).Error; err != nil {
		return nil, err
	}
	return userIDs(rows), nil
}
func (s *OrgResolver) FindUsersByPositionID(positionID uint) ([]uint, error) {
	var rows []org.UserPosition
	if err := s.db.Where("position_id = ?", positionID).Find(&rows).Error; err != nil {
		return nil, err
	}
	return userIDs(rows), nil
}
func (s *OrgResolver) FindUsersByDepartmentID(departmentID uint) ([]uint, error) {
	var rows []org.UserPosition
	if err := s.db.Where("department_id = ?", departmentID).Find(&rows).Error; err != nil {
		return nil, err
	}
	return userIDs(rows), nil
}
func (s *OrgResolver) FindManagerByUserID(userID uint) (uint, error) {
	var user model.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return 0, err
	}
	if user.ManagerID == nil {
		return 0, nil
	}
	return *user.ManagerID, nil
}

var _ app.OrgResolver = (*OrgResolver)(nil)

func userIDs(rows []org.UserPosition) []uint {
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.UserID)
	}
	return ids
}

type UserProvider struct {
	db *gorm.DB
}

func (p *UserProvider) ListActiveUsers() ([]engine.ParticipantCandidate, error) {
	var users []model.User
	if err := p.db.Where("is_active = ?", true).Find(&users).Error; err != nil {
		return nil, err
	}
	candidates := make([]engine.ParticipantCandidate, 0, len(users))
	for _, user := range users {
		candidate := engine.ParticipantCandidate{UserID: user.ID, Name: user.Username}
		var up org.UserPosition
		if err := p.db.Where("user_id = ?", user.ID).First(&up).Error; err == nil {
			var pos org.Position
			if err := p.db.First(&pos, up.PositionID).Error; err == nil {
				candidate.Position = pos.Code
			}
			var dept org.Department
			if err := p.db.First(&dept, up.DepartmentID).Error; err == nil {
				candidate.Department = dept.Code
			}
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

var _ engine.UserProvider = (*UserProvider)(nil)
