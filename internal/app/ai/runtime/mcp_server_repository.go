package runtime

import (
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
)

type MCPServerRepo struct {
	db *database.DB
}

func NewMCPServerRepo(i do.Injector) (*MCPServerRepo, error) {
	return &MCPServerRepo{db: do.MustInvoke[*database.DB](i)}, nil
}

type MCPServerListParams struct {
	Keyword  string
	Page     int
	PageSize int
}

func (r *MCPServerRepo) List(params MCPServerListParams) ([]MCPServer, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}

	query := r.db.Model(&MCPServer{})
	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		query = query.Where("name LIKE ?", like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var servers []MCPServer
	offset := (params.Page - 1) * params.PageSize
	if err := query.Offset(offset).Limit(params.PageSize).Order("created_at DESC").Find(&servers).Error; err != nil {
		return nil, 0, err
	}
	return servers, total, nil
}

func (r *MCPServerRepo) FindByID(id uint) (*MCPServer, error) {
	var s MCPServer
	if err := r.db.First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *MCPServerRepo) Create(s *MCPServer) error {
	return r.db.Create(s).Error
}

func (r *MCPServerRepo) Update(s *MCPServer) error {
	return r.db.Save(s).Error
}

func (r *MCPServerRepo) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("mcp_server_id = ?", id).Delete(&AgentMCPServer{}).Error; err != nil {
			return err
		}
		return tx.Delete(&MCPServer{}, id).Error
	})
}
