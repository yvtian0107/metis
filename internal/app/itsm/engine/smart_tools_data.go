package engine

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
	"metis/internal/app/itsm/domain"
)

// --- Result structs for DecisionDataProvider queries ---

// DecisionTicketData holds the full ticket context returned by GetTicketContext.
type DecisionTicketData struct {
	Code                  string
	Title                 string
	Description           string
	Status                string
	Outcome               string
	Source                string
	FormData              string
	SLAResponseDeadline   *time.Time
	SLAResolutionDeadline *time.Time
}

// ExecutedActionInfo holds info about a successfully executed service action.
type ExecutedActionInfo struct {
	ActionName string
	ActionCode string
	Status     string
}

// CurrentAssignmentInfo holds the current ticket assignment.
type CurrentAssignmentInfo struct {
	AssigneeID   uint
	AssigneeName string
}

// ActivityAssignmentInfo holds assignment facts for a ticket activity.
type ActivityAssignmentInfo struct {
	ParticipantType string
	UserID          *uint
	PositionID      *uint
	DepartmentID    *uint
	AssigneeID      *uint
	Status          string
	FinishedAt      *time.Time
}

// CurrentActivityInfo holds a non-terminal activity that may block the next decision.
type CurrentActivityInfo struct {
	ID              uint
	Name            string
	ActivityType    string
	NodeID          string
	Status          string
	ExecutionMode   string
	ActivityGroupID string
	AIConfidence    float64
}

// ParallelGroupInfo holds aggregated counters for a parallel activity group.
type ParallelGroupInfo struct {
	ActivityGroupID string
	Total           int64
	Completed       int64
}

// UserBasicInfo holds basic user details.
type UserBasicInfo struct {
	ID       uint
	Username string
	IsActive bool
}

// TicketHistoryRow holds a completed ticket for similar-history queries.
type TicketHistoryRow struct {
	ID         uint
	Code       string
	Title      string
	Status     string
	CreatedAt  time.Time
	FinishedAt *time.Time
}

// SLATicketData holds SLA-related fields for a ticket.
type SLATicketData struct {
	SLAStatus             string
	SLAResponseDeadline   *time.Time
	SLAResolutionDeadline *time.Time
}

// ServiceActionRow holds a service action definition.
type ServiceActionRow struct {
	ID          uint
	Code        string
	Name        string
	Description string
	ActionType  string
	ConfigJSON  string
	IsActive    bool
}

// DecisionDataProvider abstracts all data queries used by decision tools.
type DecisionDataProvider interface {
	// GetTicketContext returns the full ticket context for the given ticket ID.
	GetTicketContext(ticketID uint) (*DecisionTicketData, error)

	// GetDecisionHistory returns completed and cancelled activities for a ticket, ordered by ID ascending.
	GetDecisionHistory(ticketID uint) ([]activityModel, error)

	// GetActivityByID returns one activity for the ticket.
	GetActivityByID(ticketID, activityID uint) (*activityModel, error)

	// GetActivityAssignments returns assignment facts for one activity.
	GetActivityAssignments(activityID uint) ([]ActivityAssignmentInfo, error)

	// GetCurrentActivities returns non-terminal activities for a ticket, ordered by ID ascending.
	GetCurrentActivities(ticketID uint) ([]CurrentActivityInfo, error)

	// GetExecutedActions returns successfully executed actions for a ticket.
	GetExecutedActions(ticketID uint) ([]ExecutedActionInfo, error)

	// CountActiveServiceActions returns the number of active service actions for a ticket's runtime service version.
	CountActiveServiceActions(ticketID, serviceID uint) (int64, error)

	// GetCurrentAssignment returns the current assignment for a ticket, or nil if none.
	GetCurrentAssignment(ticketID uint) (*CurrentAssignmentInfo, error)

	// GetParallelGroups returns parallel group info for a ticket.
	GetParallelGroups(ticketID uint) ([]ParallelGroupInfo, error)

	// GetPendingActivityNames returns names of non-completed/non-cancelled activities in a group.
	GetPendingActivityNames(ticketID uint, groupID string) ([]string, error)

	// GetUserBasicInfo returns basic info for a user by ID.
	GetUserBasicInfo(userID uint) (*UserBasicInfo, error)

	// CountUserPendingActivities counts pending/in-progress activities assigned to a user.
	CountUserPendingActivities(userID uint) (int64, error)

	// GetSimilarHistory returns recently completed tickets for the same service.
	GetSimilarHistory(serviceID, excludeTicketID uint, limit int) ([]TicketHistoryRow, error)

	// CountCompletedTickets returns the total number of completed tickets for a service.
	CountCompletedTickets(serviceID uint) (int64, error)

	// CountTicketActivities returns the number of activities for a ticket.
	CountTicketActivities(ticketID uint) (int64, error)

	// GetSLAData returns SLA-related fields for a ticket.
	GetSLAData(ticketID uint) (*SLATicketData, error)

	// ListActiveServiceActions lists active service actions for a ticket's runtime service version.
	ListActiveServiceActions(ticketID, serviceID uint) ([]ServiceActionRow, error)

	// GetServiceAction returns a specific service action by ID and service ID for a ticket's runtime version.
	GetServiceAction(ticketID, actionID, serviceID uint) (*ServiceActionRow, error)

	// ResolveForTool delegates to ParticipantResolver.ResolveForTool, which needs raw DB access.
	ResolveForTool(resolver *ParticipantResolver, ticketID uint, toolArgs json.RawMessage) ([]uint, error)
}

// --- Default implementation using *gorm.DB ---

// decisionDataStore implements DecisionDataProvider backed by GORM.
type decisionDataStore struct {
	db *gorm.DB
}

// NewDecisionDataStore creates a DecisionDataProvider backed by a GORM DB handle.
func NewDecisionDataStore(db *gorm.DB) DecisionDataProvider {
	return &decisionDataStore{db: db}
}

func (s *decisionDataStore) GetTicketContext(ticketID uint) (*DecisionTicketData, error) {
	var row DecisionTicketData
	err := s.db.Table("itsm_tickets").Where("id = ?", ticketID).
		Select("code, title, description, status, outcome, source, form_data, sla_response_deadline, sla_resolution_deadline").
		First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *decisionDataStore) GetDecisionHistory(ticketID uint) ([]activityModel, error) {
	var activities []activityModel
	err := s.db.Where("ticket_id = ? AND status IN ?", ticketID, CompletedActivityStatuses()).
		Order("id ASC").Find(&activities).Error
	return activities, err
}

func (s *decisionDataStore) GetActivityByID(ticketID, activityID uint) (*activityModel, error) {
	var activity activityModel
	err := s.db.Where("ticket_id = ? AND id = ?", ticketID, activityID).First(&activity).Error
	if err != nil {
		return nil, err
	}
	return &activity, nil
}

func (s *decisionDataStore) GetActivityAssignments(activityID uint) ([]ActivityAssignmentInfo, error) {
	var assignments []ActivityAssignmentInfo
	err := s.db.Table("itsm_ticket_assignments").
		Where("activity_id = ?", activityID).
		Select("participant_type, user_id, position_id, department_id, assignee_id, status, finished_at").
		Order("id ASC").
		Find(&assignments).Error
	return assignments, err
}

func (s *decisionDataStore) GetCurrentActivities(ticketID uint) ([]CurrentActivityInfo, error) {
	var activities []CurrentActivityInfo
	err := s.db.Table("itsm_ticket_activities").
		Where("ticket_id = ? AND status IN ?", ticketID, []string{ActivityPending, ActivityInProgress}).
		Select("id, name, activity_type, node_id, status, execution_mode, activity_group_id, ai_confidence").
		Order("id ASC").
		Find(&activities).Error
	return activities, err
}

func (s *decisionDataStore) GetExecutedActions(ticketID uint) ([]ExecutedActionInfo, error) {
	var rows []ExecutedActionInfo
	err := s.db.Table("itsm_ticket_action_executions").
		Joins("JOIN itsm_service_actions ON itsm_service_actions.id = itsm_ticket_action_executions.service_action_id").
		Where("itsm_ticket_action_executions.ticket_id = ? AND itsm_ticket_action_executions.status = ?", ticketID, "success").
		Select("itsm_service_actions.name AS action_name, itsm_service_actions.code AS action_code, itsm_ticket_action_executions.status").
		Find(&rows).Error
	return rows, err
}

func (s *decisionDataStore) CountActiveServiceActions(ticketID, serviceID uint) (int64, error) {
	if rows, ok, err := s.loadSnapshotActions(ticketID, serviceID); err != nil {
		return 0, err
	} else if ok {
		var count int64
		for _, row := range rows {
			if row.IsActive {
				count++
			}
		}
		return count, nil
	}
	var count int64
	err := s.db.Table("itsm_service_actions").
		Where("service_id = ? AND is_active = ? AND deleted_at IS NULL", serviceID, true).
		Count(&count).Error
	return count, err
}

func (s *decisionDataStore) GetCurrentAssignment(ticketID uint) (*CurrentAssignmentInfo, error) {
	var assignments []struct {
		AssigneeID *uint
	}
	if err := s.db.Table("itsm_ticket_assignments").
		Where("ticket_id = ? AND is_current = ?", ticketID, true).
		Select("assignee_id").
		Limit(1).
		Find(&assignments).Error; err != nil {
		return nil, err
	}
	if len(assignments) == 0 {
		return nil, nil
	}
	assignment := assignments[0]
	if assignment.AssigneeID == nil {
		return nil, nil
	}

	var user struct {
		Username string
	}
	if err := s.db.Table("users").Where("id = ?", *assignment.AssigneeID).
		Select("username").First(&user).Error; err != nil {
		return nil, err
	}

	return &CurrentAssignmentInfo{
		AssigneeID:   *assignment.AssigneeID,
		AssigneeName: user.Username,
	}, nil
}

func (s *decisionDataStore) GetParallelGroups(ticketID uint) ([]ParallelGroupInfo, error) {
	var groups []ParallelGroupInfo
	err := s.db.Table("itsm_ticket_activities").
		Select("activity_group_id, COUNT(*) as total, SUM(CASE WHEN status IN ('completed','approved','rejected','cancelled') THEN 1 ELSE 0 END) as completed").
		Where("ticket_id = ? AND activity_group_id != ''", ticketID).
		Group("activity_group_id").
		Find(&groups).Error
	return groups, err
}

func (s *decisionDataStore) GetPendingActivityNames(ticketID uint, groupID string) ([]string, error) {
	var names []string
	err := s.db.Table("itsm_ticket_activities").
		Where("ticket_id = ? AND activity_group_id = ? AND status NOT IN ?",
			ticketID, groupID, CompletedActivityStatuses()).
		Pluck("name", &names).Error
	return names, err
}

func (s *decisionDataStore) GetUserBasicInfo(userID uint) (*UserBasicInfo, error) {
	var user UserBasicInfo
	err := s.db.Table("users").Where("id = ?", userID).
		Select("id, username, is_active").First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *decisionDataStore) CountUserPendingActivities(userID uint) (int64, error) {
	var count int64
	err := s.db.Table("itsm_ticket_assignments").
		Joins("JOIN itsm_ticket_activities ON itsm_ticket_activities.id = itsm_ticket_assignments.activity_id").
		Where("itsm_ticket_assignments.assignee_id = ? AND itsm_ticket_activities.status IN ?",
			userID, []string{ActivityPending, ActivityInProgress}).
		Count(&count).Error
	return count, err
}

func (s *decisionDataStore) GetSimilarHistory(serviceID, excludeTicketID uint, limit int) ([]TicketHistoryRow, error) {
	var rows []TicketHistoryRow
	err := s.db.Table("itsm_tickets").
		Where("service_id = ? AND status = ? AND id != ? AND deleted_at IS NULL",
			serviceID, "completed", excludeTicketID).
		Select("id, code, title, status, created_at, finished_at").
		Order("finished_at DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func (s *decisionDataStore) CountCompletedTickets(serviceID uint) (int64, error) {
	var count int64
	err := s.db.Table("itsm_tickets").
		Where("service_id = ? AND status = ? AND deleted_at IS NULL", serviceID, "completed").
		Count(&count).Error
	return count, err
}

func (s *decisionDataStore) CountTicketActivities(ticketID uint) (int64, error) {
	var count int64
	err := s.db.Table("itsm_ticket_activities").Where("ticket_id = ?", ticketID).Count(&count).Error
	return count, err
}

func (s *decisionDataStore) GetSLAData(ticketID uint) (*SLATicketData, error) {
	var row SLATicketData
	err := s.db.Table("itsm_tickets").Where("id = ?", ticketID).
		Select("sla_status, sla_response_deadline, sla_resolution_deadline").
		First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *decisionDataStore) ListActiveServiceActions(ticketID, serviceID uint) ([]ServiceActionRow, error) {
	if rows, ok, err := s.loadSnapshotActions(ticketID, serviceID); err != nil {
		return nil, err
	} else if ok {
		active := make([]ServiceActionRow, 0, len(rows))
		for _, row := range rows {
			if row.IsActive {
				active = append(active, row)
			}
		}
		return active, nil
	}
	var rows []ServiceActionRow
	err := s.db.Table("itsm_service_actions").
		Where("service_id = ? AND is_active = ? AND deleted_at IS NULL", serviceID, true).
		Select("id, code, name, description, action_type, config_json, is_active").
		Order("id ASC").
		Find(&rows).Error
	return rows, err
}

func (s *decisionDataStore) GetServiceAction(ticketID, actionID, serviceID uint) (*ServiceActionRow, error) {
	if rows, ok, err := s.loadSnapshotActions(ticketID, serviceID); err != nil {
		return nil, err
	} else if ok {
		for _, row := range rows {
			if row.ID == actionID {
				return &row, nil
			}
		}
		return nil, gorm.ErrRecordNotFound
	}
	var row ServiceActionRow
	err := s.db.Table("itsm_service_actions").
		Where("id = ? AND service_id = ? AND deleted_at IS NULL", actionID, serviceID).
		Select("id, name, code, description, action_type, config_json, is_active").
		First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *decisionDataStore) ResolveForTool(resolver *ParticipantResolver, ticketID uint, toolArgs json.RawMessage) ([]uint, error) {
	return resolver.ResolveForTool(s.db, ticketID, toolArgs)
}

func (s *decisionDataStore) loadSnapshotActions(ticketID, serviceID uint) ([]ServiceActionRow, bool, error) {
	if !s.db.Migrator().HasColumn(&ticketModel{}, "service_version_id") ||
		!s.db.Migrator().HasTable("itsm_service_definition_versions") {
		return nil, false, nil
	}
	var row struct {
		ServiceVersionID *uint
		ActionsJSON      string
	}
	err := s.db.Table("itsm_tickets").
		Joins("JOIN itsm_service_definition_versions ON itsm_service_definition_versions.id = itsm_tickets.service_version_id").
		Where("itsm_tickets.id = ? AND itsm_tickets.service_id = ? AND itsm_service_definition_versions.service_id = ?", ticketID, serviceID, serviceID).
		Select("itsm_tickets.service_version_id, itsm_service_definition_versions.actions_json").
		First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	if row.ServiceVersionID == nil {
		return nil, false, nil
	}
	actions, err := ParseServiceActionSnapshotRows(row.ActionsJSON)
	if err != nil {
		return nil, true, err
	}
	return actions, true, nil
}

func ParseServiceActionSnapshotRows(raw string) ([]ServiceActionRow, error) {
	if raw == "" || raw == "null" {
		return []ServiceActionRow{}, nil
	}
	var actions []domain.ServiceActionResponse
	if err := json.Unmarshal([]byte(raw), &actions); err != nil {
		return nil, fmt.Errorf("parse service action snapshot: %w", err)
	}
	rows := make([]ServiceActionRow, len(actions))
	for i, action := range actions {
		rows[i] = ServiceActionRow{
			ID:          action.ID,
			Code:        action.Code,
			Name:        action.Name,
			Description: action.Description,
			ActionType:  action.ActionType,
			ConfigJSON:  string(action.ConfigJSON),
			IsActive:    action.IsActive,
		}
	}
	return rows, nil
}
