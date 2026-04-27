package command

import (
	"metis/internal/app/node/domain"
	"time"

	"github.com/samber/do/v2"

	"metis/internal/database"
)

type NodeCommandRepo struct {
	db *database.DB
}

func NewNodeCommandRepo(i do.Injector) (*NodeCommandRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &NodeCommandRepo{db: db}, nil
}

func (r *NodeCommandRepo) Create(cmd *domain.NodeCommand) error {
	return r.db.Create(cmd).Error
}

func (r *NodeCommandRepo) FindByID(id uint) (*domain.NodeCommand, error) {
	var cmd domain.NodeCommand
	if err := r.db.First(&cmd, id).Error; err != nil {
		return nil, err
	}
	return &cmd, nil
}

func (r *NodeCommandRepo) FindPendingByNodeID(nodeID uint) ([]domain.NodeCommand, error) {
	var cmds []domain.NodeCommand
	if err := r.db.Where("node_id = ? AND status = ?", nodeID, domain.CommandStatusPending).
		Order("created_at ASC").
		Find(&cmds).Error; err != nil {
		return nil, err
	}
	return cmds, nil
}

func (r *NodeCommandRepo) ListByNodeID(nodeID uint, limit int) ([]domain.NodeCommand, error) {
	var cmds []domain.NodeCommand
	if limit < 1 {
		limit = 50
	}
	if err := r.db.Where("node_id = ?", nodeID).
		Order("created_at DESC").
		Limit(limit).
		Find(&cmds).Error; err != nil {
		return nil, err
	}
	return cmds, nil
}

func (r *NodeCommandRepo) ListByNodeIDPaginated(nodeID uint, page, pageSize int) ([]domain.NodeCommand, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	var total int64
	r.db.Model(&domain.NodeCommand{}).Where("node_id = ?", nodeID).Count(&total)

	var cmds []domain.NodeCommand
	if err := r.db.Where("node_id = ?", nodeID).
		Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&cmds).Error; err != nil {
		return nil, 0, err
	}
	return cmds, total, nil
}

func (r *NodeCommandRepo) Ack(id uint, result string) error {
	now := time.Now()
	return r.db.Model(&domain.NodeCommand{}).Where("id = ?", id).Updates(map[string]any{
		"status":   domain.CommandStatusAcked,
		"result":   result,
		"acked_at": &now,
	}).Error
}

func (r *NodeCommandRepo) Fail(id uint, result string) error {
	now := time.Now()
	return r.db.Model(&domain.NodeCommand{}).Where("id = ?", id).Updates(map[string]any{
		"status":   domain.CommandStatusFailed,
		"result":   result,
		"acked_at": &now,
	}).Error
}

func (r *NodeCommandRepo) CleanupExpired(timeout time.Duration) (int64, error) {
	cutoff := time.Now().Add(-timeout)
	result := r.db.Model(&domain.NodeCommand{}).
		Where("status = ? AND created_at < ?", domain.CommandStatusPending, cutoff).
		Updates(map[string]any{
			"status": domain.CommandStatusFailed,
			"result": "node_offline_timeout",
		})
	return result.RowsAffected, result.Error
}

func (r *NodeCommandRepo) FailPendingByNodeID(nodeID uint, reason string) error {
	now := time.Now()
	return r.db.Model(&domain.NodeCommand{}).
		Where("node_id = ? AND status = ?", nodeID, domain.CommandStatusPending).
		Updates(map[string]any{
			"status":   domain.CommandStatusFailed,
			"result":   reason,
			"acked_at": &now,
		}).Error
}
