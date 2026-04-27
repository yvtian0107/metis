package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/samber/do/v2"

	"metis/internal/app"
)

type KnowledgeSearchService struct {
	assetRepo *KnowledgeAssetRepo
}

func NewKnowledgeSearchService(i do.Injector) (*KnowledgeSearchService, error) {
	return &KnowledgeSearchService{assetRepo: do.MustInvoke[*KnowledgeAssetRepo](i)}, nil
}

func (s *KnowledgeSearchService) SearchKnowledge(kbIDs []uint, query string, limit int) ([]app.AIKnowledgeResult, error) {
	return s.SearchKnowledgeWithContext(context.Background(), kbIDs, query, limit)
}

// SearchKnowledgeWithContext searches selected knowledge assets and respects upstream cancellation.
func (s *KnowledgeSearchService) SearchKnowledgeWithContext(ctx context.Context, kbIDs []uint, query string, limit int) ([]app.AIKnowledgeResult, error) {
	if limit <= 0 {
		limit = 5
	}
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	assets, err := s.assetRepo.FindByIDs(uniqueUintSlice(kbIDs))
	if err != nil {
		return nil, err
	}
	results := make([]app.AIKnowledgeResult, 0)
	failures := 0
	searched := 0
	for i := range assets {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		asset := &assets[i]
		if asset.Status == AssetStatusError {
			failures++
			slog.Warn("knowledge search skipped asset in error status", "assetID", asset.ID, "name", asset.Name)
			continue
		}
		engine, err := GetEngineForAsset(asset)
		if err != nil {
			failures++
			slog.Warn("knowledge search engine unavailable", "assetID", asset.ID, "name", asset.Name, "error", err)
			continue
		}
		recall, err := engine.Search(ctx, asset, &RecallQuery{
			Query: query,
			Mode:  "hybrid",
			TopK:  limit,
		})
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			failures++
			slog.Warn("knowledge search failed for asset", "assetID", asset.ID, "name", asset.Name, "error", err)
			continue
		}
		searched++
		if recall == nil {
			continue
		}
		for _, item := range recall.Items {
			title := item.Title
			if title == "" {
				title = asset.Name
			}
			content := item.Content
			if content == "" {
				content = item.Summary
			}
			if content == "" {
				continue
			}
			results = append(results, app.AIKnowledgeResult{
				Title:   title,
				Content: content,
				Score:   item.Score,
			})
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > limit {
		results = results[:limit]
	}
	if len(results) == 0 && searched == 0 && failures > 0 {
		return nil, fmt.Errorf("knowledge search failed for all selected assets")
	}
	return results, nil
}

var _ app.AIKnowledgeSearcher = (*KnowledgeSearchService)(nil)
