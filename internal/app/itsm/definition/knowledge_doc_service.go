package definition

import (
	"encoding/json"
	"fmt"
	"io"
	. "metis/internal/app/itsm/domain"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/pkg/docparse"
	"metis/internal/scheduler"
)

type KnowledgeDocService struct {
	repo        *KnowledgeDocRepo
	db          *gorm.DB
	serviceDefs *ServiceDefService
}

func NewKnowledgeDocService(i do.Injector) (*KnowledgeDocService, error) {
	repo := do.MustInvoke[*KnowledgeDocRepo](i)
	db := do.MustInvoke[*database.DB](i)
	serviceDefs := do.MustInvoke[*ServiceDefService](i)
	return &KnowledgeDocService{repo: repo, db: db.DB, serviceDefs: serviceDefs}, nil
}

// MaxFileSize is the upload limit (10 MB).
const MaxFileSize = 10 * 1024 * 1024

// AllowedExtensions lists file extensions supported for upload.
var AllowedExtensions = map[string]string{
	".txt":  "text/plain",
	".md":   "text/markdown",
	".pdf":  "application/pdf",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
}

func IsAllowedExtension(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	_, ok := AllowedExtensions[ext]
	return ok
}

func (s *KnowledgeDocService) Upload(serviceID uint, fileName string, fileSize int64, reader io.Reader) (*ServiceKnowledgeDocument, error) {
	ext := strings.ToLower(filepath.Ext(fileName))
	mime, ok := AllowedExtensions[ext]
	if !ok {
		return nil, fmt.Errorf("不支持的文件类型: %s", ext)
	}
	if fileSize > MaxFileSize {
		return nil, fmt.Errorf("文件大小超过限制 (%d MB)", MaxFileSize/1024/1024)
	}

	// Save file to disk
	uploadDir := filepath.Join("uploads", "itsm", "knowledge", fmt.Sprintf("%d", serviceID))
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("create upload dir: %w", err)
	}

	storedName := fmt.Sprintf("%d_%s", time.Now().UnixMilli(), fileName)
	filePath := filepath.Join(uploadDir, storedName)

	f, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	written, err := io.Copy(f, reader)
	if err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("save file: %w", err)
	}

	doc := &ServiceKnowledgeDocument{
		ServiceID:   serviceID,
		FileName:    fileName,
		FilePath:    filePath,
		FileSize:    written,
		FileType:    mime,
		ParseStatus: "pending",
	}

	if err := s.repo.Create(doc); err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("create record: %w", err)
	}

	// Submit async parse task
	payload, _ := json.Marshal(map[string]uint{"document_id": doc.ID})
	exec := &model.TaskExecution{
		TaskName: "itsm-doc-parse",
		Trigger:  scheduler.TriggerAPI,
		Status:   scheduler.ExecPending,
		Payload:  string(payload),
	}
	s.db.Create(exec)
	if s.serviceDefs != nil {
		if err := s.serviceDefs.RefreshPublishHealthCheckIfPresent(serviceID); err != nil {
			return nil, err
		}
	}

	return doc, nil
}

func (s *KnowledgeDocService) List(serviceID uint) ([]ServiceKnowledgeDocument, error) {
	return s.repo.ListByServiceID(serviceID)
}

func (s *KnowledgeDocService) Delete(id uint) error {
	doc, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(id); err != nil {
		return err
	}
	if s.serviceDefs != nil {
		return s.serviceDefs.RefreshPublishHealthCheckIfPresent(doc.ServiceID)
	}
	return nil
}

// ParseDocument synchronously parses a document (called by scheduler task handler).
func (s *KnowledgeDocService) ParseDocument(documentID uint) error {
	doc, err := s.repo.GetByID(documentID)
	if err != nil {
		return fmt.Errorf("get document: %w", err)
	}

	// Mark as processing
	s.repo.UpdateParseResult(doc.ID, "processing", "", "")

	text, err := docparse.Parse(doc.FilePath)
	if err != nil {
		s.repo.UpdateParseResult(doc.ID, "failed", "", err.Error())
		if s.serviceDefs != nil {
			_ = s.serviceDefs.RefreshPublishHealthCheckIfPresent(doc.ServiceID)
		}
		return fmt.Errorf("parse document: %w", err)
	}

	if err := s.repo.UpdateParseResult(doc.ID, "completed", text, ""); err != nil {
		return err
	}
	if s.serviceDefs != nil {
		return s.serviceDefs.RefreshPublishHealthCheckIfPresent(doc.ServiceID)
	}
	return nil
}
