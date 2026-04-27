package log

import (
	"metis/internal/app/node/domain"
	"time"

	"github.com/samber/do/v2"

	"metis/internal/database"
)

type NodeProcessLogRepo struct {
	db *database.DB
}

func NewNodeProcessLogRepo(i do.Injector) (*NodeProcessLogRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &NodeProcessLogRepo{db: db}, nil
}

func (r *NodeProcessLogRepo) CreateBatch(logs []domain.NodeProcessLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.CreateInBatches(&logs, 100).Error
}

type LogListParams struct {
	NodeID       uint
	ProcessDefID uint
	Stream       string
	Page         int
	PageSize     int
}

type LogListResult struct {
	Items []domain.NodeProcessLog `json:"items"`
	Total int64                   `json:"total"`
}

func (r *NodeProcessLogRepo) List(params LogListParams) (*LogListResult, error) {
	query := r.db.Model(&domain.NodeProcessLog{})

	if params.NodeID > 0 {
		query = query.Where("node_id = ?", params.NodeID)
	}
	if params.ProcessDefID > 0 {
		query = query.Where("process_def_id = ?", params.ProcessDefID)
	}
	if params.Stream != "" {
		query = query.Where("stream = ?", params.Stream)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 50
	}
	if params.PageSize > 200 {
		params.PageSize = 200
	}

	var logs []domain.NodeProcessLog
	if err := query.Order("timestamp DESC").
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Find(&logs).Error; err != nil {
		return nil, err
	}

	return &LogListResult{Items: logs, Total: total}, nil
}

func (r *NodeProcessLogRepo) DeleteBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("timestamp < ?", cutoff).Delete(&domain.NodeProcessLog{})
	return result.RowsAffected, result.Error
}
