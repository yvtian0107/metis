package catalog

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"

	"metis/internal/handler"
)

func TestCatalogHandlerCreate_Returns409ForDuplicateCode(t *testing.T) {
	db := newTestDB(t)
	svc := newCatalogServiceForTest(t, db)
	h := &CatalogHandler{svc: svc}

	if _, err := svc.Create("Root", "root", "", "", nil, 10); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}

	rec := performJSONRequest(t, func(r *gin.Engine) {
		r.POST("/catalogs", h.Create)
	}, http.MethodPost, "/catalogs", []byte(`{"name":"Other","code":"root"}`))

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}

	resp := decodeResponseBody[handler.R](t, rec)
	if resp.Message == "" {
		t.Fatalf("expected error message")
	}
}

func TestCatalogHandlerUpdate_Returns400ForSelfParent(t *testing.T) {
	db := newTestDB(t)
	svc := newCatalogServiceForTest(t, db)
	h := &CatalogHandler{svc: svc}

	root, err := svc.Create("Root", "root", "", "", nil, 10)
	if err != nil {
		t.Fatalf("seed catalog: %v", err)
	}

	body := []byte(`{"parentId":` + itoa(root.ID) + `}`)
	rec := performJSONRequest(t, func(r *gin.Engine) {
		r.PUT("/catalogs/:id", h.Update)
	}, http.MethodPut, "/catalogs/"+itoa(root.ID), body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func performJSONRequest(t *testing.T, routes func(*gin.Engine), method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	r := gin.New()
	routes(r)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	return rec
}

func itoa(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}
