package runtime

import (
	"log/slog"
	"net/http"
	"sort"
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

const maxSearchConcurrency = 8

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

	ctx := c.Request.Context()

	type result struct {
		res *RecallResult
		err error
	}
	results := make([]result, len(assets))

	sem := make(chan struct{}, maxSearchConcurrency)
	var wg sync.WaitGroup
	for i := range assets {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			a := &assets[idx]
			engine, engineErr := GetEngineForAsset(a)
			if engineErr != nil {
				results[idx] = result{err: engineErr}
				return
			}
			rr, searchErr := engine.Search(ctx, a, &RecallQuery{
				Query: req.Query,
				Mode:  "hybrid",
				TopK:  req.TopK,
			})
			results[idx] = result{res: rr, err: searchErr}
		}(i)
	}
	wg.Wait()

	merged := &RecallResult{Items: []KnowledgeUnit{}}
	seenSources := make(map[uint]struct{})
	for i, r := range results {
		if r.err != nil {
			slog.Warn("knowledge search failed", "assetID", assets[i].ID, "error", r.err)
			continue
		}
		if r.res == nil {
			continue
		}
		merged.Items = append(merged.Items, r.res.Items...)
		merged.Relations = append(merged.Relations, r.res.Relations...)
		for _, s := range r.res.Sources {
			if _, dup := seenSources[s.SourceID]; !dup {
				seenSources[s.SourceID] = struct{}{}
				merged.Sources = append(merged.Sources, s)
			}
		}
	}

	// Enforce TopK on merged results.
	if len(merged.Items) > req.TopK {
		sort.Slice(merged.Items, func(i, j int) bool {
			return merged.Items[i].Score > merged.Items[j].Score
		})
		merged.Items = merged.Items[:req.TopK]
	}

	handler.OK(c, merged)
}
