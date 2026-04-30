package definition

import (
	. "metis/internal/app/itsm/domain"
	"net/http"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func newKnowledgeDocServiceForTest(t *testing.T, db *gorm.DB, serviceDefs *ServiceDefService) *KnowledgeDocService {
	t.Helper()
	return &KnowledgeDocService{
		repo:        &KnowledgeDocRepo{db: db},
		db:          db,
		serviceDefs: serviceDefs,
	}
}

func TestKnowledgeDocHandlerDelete_RequiresParentServiceMatch(t *testing.T) {
	db := newTestDB(t)
	catSvc := newCatalogServiceForTest(t, db)
	serviceDefs := newServiceDefServiceForTest(t, db)
	docSvc := newKnowledgeDocServiceForTest(t, db, serviceDefs)
	h := &KnowledgeDocHandler{svc: docSvc}

	root, err := catSvc.Create("Root", "root", "", "", nil, 10)
	if err != nil {
		t.Fatalf("create root: %v", err)
	}
	serviceA, err := serviceDefs.Create(&ServiceDefinition{Name: "A", Code: "doc-a", CatalogID: root.ID, EngineType: "smart", CollaborationSpec: "spec"})
	if err != nil {
		t.Fatalf("create service A: %v", err)
	}
	serviceB, err := serviceDefs.Create(&ServiceDefinition{Name: "B", Code: "doc-b", CatalogID: root.ID, EngineType: "smart", CollaborationSpec: "spec"})
	if err != nil {
		t.Fatalf("create service B: %v", err)
	}
	doc := &ServiceKnowledgeDocument{
		ServiceID:   serviceB.ID,
		FileName:    "b.md",
		FilePath:    "uploads/itsm/knowledge/b.md",
		FileSize:    8,
		FileType:    "text/markdown",
		ParseStatus: "completed",
	}
	if err := db.Create(doc).Error; err != nil {
		t.Fatalf("create document: %v", err)
	}

	path := "/services/" + strconv.FormatUint(uint64(serviceA.ID), 10) + "/knowledge-docs/" + strconv.FormatUint(uint64(doc.ID), 10)
	rec := performJSONRequest(t, func(r *gin.Engine) {
		r.DELETE("/services/:id/knowledge-docs/:docId", h.Delete)
	}, http.MethodDelete, path, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected cross-service delete to return 404, got %d body=%s", rec.Code, rec.Body.String())
	}

	var remaining ServiceKnowledgeDocument
	if err := db.First(&remaining, doc.ID).Error; err != nil {
		t.Fatalf("document should not be deleted: %v", err)
	}
}
