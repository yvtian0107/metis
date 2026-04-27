package log

import (
	"context"
	"encoding/json"
	"log/slog"
	"metis/internal/app/node/domain"
	"time"

	"github.com/samber/do/v2"
)

const logRetentionDays = 7

type ProcessDefFinder interface {
	FindByName(name string) (*domain.ProcessDef, error)
}

type NodeProcessLogService struct {
	logRepo        *NodeProcessLogRepo
	processDefRepo ProcessDefFinder
}

func NewNodeProcessLogService(i do.Injector) (*NodeProcessLogService, error) {
	return &NodeProcessLogService{
		logRepo:        do.MustInvoke[*NodeProcessLogRepo](i),
		processDefRepo: do.MustInvoke[ProcessDefFinder](i),
	}, nil
}

type UploadLogEntry struct {
	ProcessName string `json:"processName"`
	Stream      string `json:"stream"`
	Content     string `json:"content"`
}

func (s *NodeProcessLogService) Ingest(nodeID uint, entries []UploadLogEntry) error {
	now := time.Now()
	// Cache ProcessName → ProcessDefID within this batch
	defIDCache := make(map[string]uint)
	logs := make([]domain.NodeProcessLog, 0, len(entries))
	for _, e := range entries {
		defID, ok := defIDCache[e.ProcessName]
		if !ok {
			if pd, err := s.processDefRepo.FindByName(e.ProcessName); err == nil {
				defID = pd.ID
			}
			defIDCache[e.ProcessName] = defID
		}
		logs = append(logs, domain.NodeProcessLog{
			NodeID:       nodeID,
			ProcessDefID: defID,
			ProcessName:  e.ProcessName,
			Stream:       e.Stream,
			Content:      e.Content,
			Timestamp:    now,
		})
	}
	return s.logRepo.CreateBatch(logs)
}

func (s *NodeProcessLogService) List(params LogListParams) (*LogListResult, error) {
	return s.logRepo.List(params)
}

func (s *NodeProcessLogService) CleanupOldLogs(_ context.Context, _ json.RawMessage) error {
	cutoff := time.Now().AddDate(0, 0, -logRetentionDays)
	affected, err := s.logRepo.DeleteBefore(cutoff)
	if err != nil {
		return err
	}
	if affected > 0 {
		slog.Info("node process log cleanup", "deleted", affected)
	}
	return nil
}
