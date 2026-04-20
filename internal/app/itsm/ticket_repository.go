package itsm

import (
	"fmt"
	"sync"
	"time"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
)

type TicketRepo struct {
	db    *database.DB
	seqMu sync.Mutex
}

func NewTicketRepo(i do.Injector) (*TicketRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &TicketRepo{db: db}, nil
}

// NextCode generates the next ticket code (TICK-XXXXXX) in a concurrency-safe manner.
func (r *TicketRepo) NextCode() (string, error) {
	r.seqMu.Lock()
	defer r.seqMu.Unlock()

	var maxID uint
	if err := r.db.Model(&Ticket{}).Select("COALESCE(MAX(id), 0)").Scan(&maxID).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("TICK-%06d", maxID+1), nil
}

func (r *TicketRepo) Create(t *Ticket) error {
	return r.db.Create(t).Error
}

func (r *TicketRepo) FindByID(id uint) (*Ticket, error) {
	var t Ticket
	if err := r.db.First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TicketRepo) FindByCode(code string) (*Ticket, error) {
	var t Ticket
	if err := r.db.Where("code = ?", code).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TicketRepo) Update(id uint, updates map[string]any) error {
	return r.db.Model(&Ticket{}).Where("id = ?", id).Updates(updates).Error
}

type TicketListParams struct {
	Keyword     string
	Status      string
	PriorityID  *uint
	ServiceID   *uint
	AssigneeID  *uint
	RequesterID *uint
	Page        int
	PageSize    int
	DeptScope   *[]uint
}

func (r *TicketRepo) List(params TicketListParams) ([]Ticket, int64, error) {
	query := r.db.Model(&Ticket{})

	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		query = query.Where("code LIKE ? OR title LIKE ? OR description LIKE ?", like, like, like)
	}
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.PriorityID != nil {
		query = query.Where("priority_id = ?", *params.PriorityID)
	}
	if params.ServiceID != nil {
		query = query.Where("service_id = ?", *params.ServiceID)
	}
	if params.AssigneeID != nil {
		query = query.Where("assignee_id = ?", *params.AssigneeID)
	}
	if params.RequesterID != nil {
		query = query.Where("requester_id = ?", *params.RequesterID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}

	var items []Ticket
	offset := (params.Page - 1) * params.PageSize
	if err := query.Offset(offset).Limit(params.PageSize).Order("id DESC").Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// TodoListParams holds query parameters for the todo list.
type TodoListParams struct {
	UserID      uint
	PositionIDs []uint
	DeptIDs     []uint
	Keyword     string
	Status      string
	Page        int
	PageSize    int
}

// ListTodo returns active tickets where the user has an assignment on an active activity.
// Matches by userID, positionIDs, or deptIDs on TicketAssignment.
func (r *TicketRepo) ListTodo(params TodoListParams) ([]Ticket, int64, error) {
	activeStatuses := []string{TicketStatusPending, TicketStatusInProgress, TicketStatusWaitingApproval}

	buildQuery := func(q *gorm.DB) *gorm.DB {
		q = q.
			Joins("JOIN itsm_ticket_assignments AS a ON a.ticket_id = itsm_tickets.id").
			Joins("JOIN itsm_ticket_activities AS act ON act.id = a.activity_id").
			Where("itsm_tickets.deleted_at IS NULL AND act.deleted_at IS NULL").
			Where("act.status IN ?", []string{"pending", "in_progress"})

		if params.Status != "" {
			q = q.Where("itsm_tickets.status = ?", params.Status)
		} else {
			q = q.Where("itsm_tickets.status IN ?", activeStatuses)
		}

		// Multi-dimensional participant matching
		conditions := r.db.Where("a.user_id = ?", params.UserID)
		if len(params.PositionIDs) > 0 {
			conditions = conditions.Or("a.position_id IN ?", params.PositionIDs)
		}
		if len(params.DeptIDs) > 0 {
			conditions = conditions.Or("a.department_id IN ?", params.DeptIDs)
		}
		q = q.Where(conditions)

		if params.Keyword != "" {
			like := "%" + params.Keyword + "%"
			q = q.Where("(itsm_tickets.code LIKE ? OR itsm_tickets.title LIKE ?)", like, like)
		}
		return q
	}

	// Count distinct tickets
	countQuery := buildQuery(r.db.Model(&Ticket{})).Select("DISTINCT itsm_tickets.id")
	var total int64
	if err := r.db.Table("(?) AS sub", countQuery).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}

	var items []Ticket
	offset := (params.Page - 1) * params.PageSize
	dataQuery := buildQuery(r.db.Model(&Ticket{})).
		Joins("LEFT JOIN itsm_priorities ON itsm_priorities.id = itsm_tickets.priority_id").
		Select("DISTINCT itsm_tickets.*").
		Order("itsm_priorities.value ASC, itsm_tickets.created_at ASC").
		Offset(offset).Limit(params.PageSize)
	if err := dataQuery.Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

type HistoryListParams struct {
	UserID     *uint
	AssigneeID *uint
	StartDate  *time.Time
	EndDate    *time.Time
	Page       int
	PageSize   int
}

// ListHistory returns terminal-state tickets with optional filters.
// When UserID is set, restricts to tickets where the user is requester or assignee.
func (r *TicketRepo) ListHistory(params HistoryListParams) ([]Ticket, int64, error) {
	terminalStatuses := []string{TicketStatusCompleted, TicketStatusFailed, TicketStatusCancelled}
	query := r.db.Model(&Ticket{}).Where("status IN ?", terminalStatuses)

	if params.UserID != nil {
		query = query.Where("(requester_id = ? OR assignee_id = ?)", *params.UserID, *params.UserID)
	}
	if params.AssigneeID != nil {
		query = query.Where("assignee_id = ?", *params.AssigneeID)
	}
	if params.StartDate != nil {
		query = query.Where("finished_at >= ?", *params.StartDate)
	}
	if params.EndDate != nil {
		query = query.Where("finished_at <= ?", *params.EndDate)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}

	var items []Ticket
	offset := (params.Page - 1) * params.PageSize
	if err := query.Offset(offset).Limit(params.PageSize).Order("finished_at DESC").Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ApprovalItem represents a pending approval activity with its ticket context.
type ApprovalItem struct {
	// Ticket fields
	TicketID              uint       `json:"ticketId"`
	TicketCode            string     `json:"ticketCode"`
	TicketTitle           string     `json:"ticketTitle"`
	TicketStatus          string     `json:"ticketStatus"`
	ServiceID             uint       `json:"serviceId"`
	PriorityID            uint       `json:"priorityId"`
	SLAStatus             string     `json:"slaStatus"`
	SLAResponseDeadline   *time.Time `json:"slaResponseDeadline"`
	SLAResolutionDeadline *time.Time `json:"slaResolutionDeadline"`
	// Activity fields
	ActivityID     uint       `json:"activityId"`
	ActivityName   string     `json:"activityName"`
	ActivityType   string     `json:"activityType"`
	ActivityStatus string     `json:"activityStatus"`
	FormSchema     JSONField  `json:"formSchema"`
	AIConfidence   float64    `json:"aiConfidence"`
	AIReasoning    string     `json:"aiReasoning"`
	StartedAt      *time.Time `json:"startedAt"`
	CreatedAt      time.Time  `json:"createdAt"`
	// Assignment fields (zero for ai_confirm items)
	AssignmentID    uint   `json:"assignmentId"`
	ParticipantType string `json:"participantType"`
	CanAct          bool   `json:"canAct"`
	// Discriminator
	ApprovalKind string `json:"approvalKind"`
}

// ListApprovals returns pending approval items: workflow approvals (via assignment) and AI decision confirmations (pending_approval status).
func (r *TicketRepo) ListApprovals(userID uint, positionIDs []uint, deptIDs []uint, page, pageSize int) ([]ApprovalItem, int64, error) {
	// Build user matching condition for workflow approvals
	userCond := r.db.Where("a.user_id = ? OR a.assignee_id = ?", userID, userID)
	if len(positionIDs) > 0 {
		userCond = userCond.Or("a.position_id IN ?", positionIDs)
	}
	if len(deptIDs) > 0 {
		userCond = userCond.Or("a.department_id IN ?", deptIDs)
	}

	// Single query with OR: workflow approvals via assignment OR AI confirmations via status
	baseQuery := r.db.Table("itsm_ticket_activities AS act").
		Joins("JOIN itsm_tickets AS t ON t.id = act.ticket_id").
		Joins("LEFT JOIN itsm_ticket_assignments AS a ON a.activity_id = act.id").
		Joins("LEFT JOIN itsm_priorities AS p ON p.id = t.priority_id").
		Where("t.deleted_at IS NULL AND act.deleted_at IS NULL").
		Where(
			r.db.Where(
				// Workflow approvals: approve activity + active status + user match
				r.db.Where("act.activity_type = ? AND act.status IN ?", "approve", []string{"pending", "in_progress"}).
					Where("a.status = ?", AssignmentPending).
					Where(userCond),
			).Or(
				// AI confirmations: pending_approval status owned by actionable assignments only
				r.db.Where("act.status = ?", "pending_approval").
					Where("a.status = ?", AssignmentPending).
					Where(userCond),
			),
		)

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	var items []ApprovalItem
	offset := (page - 1) * pageSize
	if err := baseQuery.
		Select(`t.id AS ticket_id, t.code AS ticket_code, t.title AS ticket_title, t.status AS ticket_status,
			t.service_id, t.priority_id, t.sla_status, t.sla_response_deadline, t.sla_resolution_deadline,
			act.id AS activity_id, act.name AS activity_name, act.activity_type, act.status AS activity_status,
			act.form_schema, act.ai_confidence, act.ai_reasoning, act.started_at, act.created_at,
			COALESCE(a.id, 0) AS assignment_id, COALESCE(a.participant_type, '') AS participant_type,
			CASE WHEN COALESCE(a.id, 0) > 0 THEN true ELSE false END AS can_act,
			CASE WHEN act.status = 'pending_approval' THEN 'ai_confirm' ELSE 'workflow' END AS approval_kind`).
		Order("p.value ASC, act.created_at ASC").
		Offset(offset).Limit(pageSize).
		Scan(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// CountApprovals returns the combined count of pending workflow approvals and AI decision confirmations.
func (r *TicketRepo) CountApprovals(userID uint, positionIDs []uint, deptIDs []uint) (int64, error) {
	userCond := r.db.Where("a.user_id = ? OR a.assignee_id = ?", userID, userID)
	if len(positionIDs) > 0 {
		userCond = userCond.Or("a.position_id IN ?", positionIDs)
	}
	if len(deptIDs) > 0 {
		userCond = userCond.Or("a.department_id IN ?", deptIDs)
	}

	query := r.db.Table("itsm_ticket_activities AS act").
		Joins("JOIN itsm_tickets AS t ON t.id = act.ticket_id").
		Joins("LEFT JOIN itsm_ticket_assignments AS a ON a.activity_id = act.id").
		Where("t.deleted_at IS NULL AND act.deleted_at IS NULL").
		Where(
			r.db.Where(
				r.db.Where("act.activity_type = ? AND act.status IN ?", "approve", []string{"pending", "in_progress"}).
					Where("a.status = ?", AssignmentPending).
					Where(userCond),
			).Or(
				r.db.Where("act.status = ?", "pending_approval").
					Where("a.status = ?", AssignmentPending).
					Where(userCond),
			),
		)

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// UpdateInTx performs an update within a provided transaction.
func (r *TicketRepo) UpdateInTx(tx *gorm.DB, id uint, updates map[string]any) error {
	return tx.Model(&Ticket{}).Where("id = ?", id).Updates(updates).Error
}

func (r *TicketRepo) DB() *gorm.DB {
	return r.db.DB
}
