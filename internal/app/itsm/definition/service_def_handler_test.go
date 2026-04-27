package definition

import (
	. "metis/internal/app/itsm/domain"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"metis/internal/handler"
)

func TestServiceDefHandlerCreate_Returns400ForMissingCatalog(t *testing.T) {
	db := newTestDB(t)
	svc := newServiceDefServiceForTest(t, db)
	h := &ServiceDefHandler{svc: svc}

	rec := performJSONRequest(t, func(r *gin.Engine) {
		r.POST("/services", h.Create)
	}, http.MethodPost, "/services", []byte(`{"name":"VPN","code":"vpn","catalogId":999,"engineType":"classic"}`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestServiceDefHandlerList_ParsesEngineTypeFilter(t *testing.T) {
	db := newTestDB(t)
	catSvc := newCatalogServiceForTest(t, db)
	svc := newServiceDefServiceForTest(t, db)
	h := &ServiceDefHandler{svc: svc}

	root, err := catSvc.Create("Root", "root", "", "", nil, 10)
	if err != nil {
		t.Fatalf("create root: %v", err)
	}
	if _, err := svc.Create(&ServiceDefinition{Name: "Classic", Code: "classic", CatalogID: root.ID, EngineType: "classic"}); err != nil {
		t.Fatalf("create classic: %v", err)
	}
	if _, err := svc.Create(&ServiceDefinition{Name: "Smart", Code: "smart", CatalogID: root.ID, EngineType: "smart"}); err != nil {
		t.Fatalf("create smart: %v", err)
	}

	rec := performJSONRequest(t, func(r *gin.Engine) {
		r.GET("/services", h.List)
	}, http.MethodGet, "/services?engineType=smart", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeResponseBody[handler.R](t, rec)
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected object data, got %#v", resp.Data)
	}
	items, ok := data["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected 1 filtered item, got %#v", data["items"])
	}
	first, ok := items[0].(map[string]any)
	if !ok || first["engineType"] != "smart" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
}
