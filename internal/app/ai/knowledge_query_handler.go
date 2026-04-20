package ai

import (
	"context"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"

	"metis/internal/handler"
)

// KnowledgeQueryHandler handles unified knowledge search requests from
// Sidecar / Agent runtime. It accepts multiple asset IDs, routes each to
// its engine, and merges results.
type KnowledgeQueryHandler struct {
	assetRepo *KnowledgeAssetRepo
}

func NewKnowledgeQueryHandler(i do.Injector) (*KnowledgeQueryHandler, error) {
	return &KnowledgeQueryHandler{
		assetRepo: do.MustInvoke[*KnowledgeAssetRepo](i),
	}, nil
}

type knowledgeQueryRequest struct {
	AssetIDs []uint `json:"assetIds" binding:"required,min=1"`
	Query    string `json:"query" binding:"required"`
	TopK     int    `json:"topK"`
}

// Search accepts a list of asset IDs and a query string, routes to the
// correct engine for each asset, and returns merged RecallResult.
func (h *KnowledgeQueryHandler) Search(c *gin.Context) {
	var req knowledgeQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.TopK <= 0 {
		req.TopK = 5
	}

	assets, err := h.assetRepo.FindByIDs(req.AssetIDs)
	if err != nil {
		handler.Fail(c, http.StatusInternalServerError, "failed to load assets")
		return
	}
	if len(assets) == 0 {
		handler.OK(c, &RecallResult{Items: []KnowledgeUnit{}})
		return
	}

	// Search each asset concurrently.
	type result struct {
		res *RecallResult
		err error
	}
	results := make([]result, len(assets))
	var wg sync.WaitGroup
	for i := range assets {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			a := &assets[idx]
			engine, engineErr := GetEngineForAsset(a)
			if engineErr != nil {
				results[idx] = result{err: engineErr}
				return
			}
			rr, searchErr := engine.Search(context.Background(), a, &RecallQuery{
				Query: req.Query,
				Mode:  "hybrid",
				TopK:  req.TopK,
			})
			results[idx] = result{res: rr, err: searchErr}
		}(i)
	}
	wg.Wait()

	// Merge all results.
	merged := &RecallResult{}
	for _, r := range results {
		if r.err != nil || r.res == nil {
			continue
		}
		merged.Items = append(merged.Items, r.res.Items...)
		merged.Relations = append(merged.Relations, r.res.Relations...)
		merged.Sources = append(merged.Sources, r.res.Sources...)
	}

	// Ensure non-nil slices for JSON.
	if merged.Items == nil {
		merged.Items = []KnowledgeUnit{}
	}

	handler.OK(c, merged)
}
