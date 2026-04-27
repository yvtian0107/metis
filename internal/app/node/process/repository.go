package process

import (
	"metis/internal/app/node/domain"
	"time"

	"github.com/samber/do/v2"

	"metis/internal/database"
)

type NodeProcessRepo struct {
	db *database.DB
}

func NewNodeProcessRepo(i do.Injector) (*NodeProcessRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &NodeProcessRepo{db: db}, nil
}

func (r *NodeProcessRepo) Create(np *domain.NodeProcess) error {
	return r.db.Create(np).Error
}

func (r *NodeProcessRepo) FindByID(id uint) (*domain.NodeProcess, error) {
	var np domain.NodeProcess
	if err := r.db.First(&np, id).Error; err != nil {
		return nil, err
	}
	return &np, nil
}

func (r *NodeProcessRepo) FindByNodeAndProcessDef(nodeID, processDefID uint) (*domain.NodeProcess, error) {
	var np domain.NodeProcess
	if err := r.db.Where("node_id = ? AND process_def_id = ?", nodeID, processDefID).First(&np).Error; err != nil {
		return nil, err
	}
	return &np, nil
}

type NodeProcessDetail struct {
	domain.NodeProcess
	ProcessName string `gorm:"column:process_name"`
	DisplayName string `gorm:"column:display_name"`
}

func (r *NodeProcessRepo) ListByNodeID(nodeID uint) ([]NodeProcessDetail, error) {
	var items []NodeProcessDetail
	if err := r.db.Model(&domain.NodeProcess{}).
		Select("node_processes.*, process_defs.name as process_name, process_defs.display_name as display_name").
		Joins("LEFT JOIN process_defs ON process_defs.id = node_processes.process_def_id AND process_defs.deleted_at IS NULL").
		Where("node_processes.node_id = ?", nodeID).
		Order("node_processes.created_at ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *NodeProcessRepo) ListByProcessDefID(processDefID uint) ([]domain.NodeProcess, error) {
	var items []domain.NodeProcess
	if err := r.db.Where("process_def_id = ?", processDefID).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *NodeProcessRepo) UpdateStatus(id uint, status string, pid int) error {
	updates := map[string]any{"status": status, "pid": pid}
	return r.db.Model(&domain.NodeProcess{}).Where("id = ?", id).Updates(updates).Error
}

func (r *NodeProcessRepo) UpdateConfigVersion(id uint, version string) error {
	return r.db.Model(&domain.NodeProcess{}).Where("id = ?", id).Update("config_version", version).Error
}

func (r *NodeProcessRepo) UpdateProbe(id uint, probeResult domain.JSONMap) error {
	return r.db.Model(&domain.NodeProcess{}).Where("id = ?", id).Update("last_probe", probeResult).Error
}

func (r *NodeProcessRepo) Delete(id uint) error {
	return r.db.Delete(&domain.NodeProcess{}, id).Error
}

func (r *NodeProcessRepo) DeleteByNodeID(nodeID uint) error {
	return r.db.Where("node_id = ?", nodeID).Delete(&domain.NodeProcess{}).Error
}

func (r *NodeProcessRepo) BatchUpdateStatusByNodeID(nodeID uint, status string) error {
	return r.db.Model(&domain.NodeProcess{}).Where("node_id = ?", nodeID).Updates(map[string]any{
		"status": status,
		"pid":    0,
	}).Error
}

// ProcessDefNodeDetail holds node info along with its process binding status for a given domain.ProcessDef.
type ProcessDefNodeDetail struct {
	NodeID        uint      `json:"nodeId" gorm:"column:node_id"`
	NodeName      string    `json:"nodeName" gorm:"column:node_name"`
	NodeStatus    string    `json:"nodeStatus" gorm:"column:node_status"`
	ProcessStatus string    `json:"processStatus" gorm:"column:process_status"`
	PID           int       `json:"pid" gorm:"column:pid"`
	ConfigVersion string    `json:"configVersion" gorm:"column:config_version"`
	BoundAt       time.Time `json:"boundAt" gorm:"column:bound_at"`
}

func (r *NodeProcessRepo) ListNodesByProcessDefID(processDefID uint) ([]ProcessDefNodeDetail, error) {
	var items []ProcessDefNodeDetail
	if err := r.db.Model(&domain.NodeProcess{}).
		Select("node_processes.node_id, nodes.name as node_name, nodes.status as node_status, node_processes.status as process_status, node_processes.pid, node_processes.config_version, node_processes.created_at as bound_at").
		Joins("INNER JOIN nodes ON nodes.id = node_processes.node_id AND nodes.deleted_at IS NULL").
		Where("node_processes.process_def_id = ?", processDefID).
		Order("node_processes.created_at ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
