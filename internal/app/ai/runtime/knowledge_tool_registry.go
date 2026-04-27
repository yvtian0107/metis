package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/samber/do/v2"

	"metis/internal/app"
)

const searchKnowledgeToolName = "search_knowledge"

// KnowledgeToolRegistry exposes agent-scoped knowledge search as a runtime tool.
type KnowledgeToolRegistry struct {
	agentRepo         *AgentRepo
	sessionRepo       *SessionRepo
	knowledgeSearcher *KnowledgeSearchService
}

func NewKnowledgeToolRegistry(i do.Injector) (*KnowledgeToolRegistry, error) {
	return &KnowledgeToolRegistry{
		agentRepo:         do.MustInvoke[*AgentRepo](i),
		sessionRepo:       do.MustInvoke[*SessionRepo](i),
		knowledgeSearcher: do.MustInvoke[*KnowledgeSearchService](i),
	}, nil
}

func (r *KnowledgeToolRegistry) HasTool(name string) bool {
	return name == searchKnowledgeToolName
}

func (r *KnowledgeToolRegistry) Execute(ctx context.Context, toolName string, userID uint, args json.RawMessage) (json.RawMessage, error) {
	if toolName != searchKnowledgeToolName {
		return nil, fmt.Errorf("unknown knowledge tool: %s", toolName)
	}
	return r.handleSearchKnowledge(ctx, userID, args)
}

type searchKnowledgeArgs struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

type searchKnowledgeResult struct {
	Query           string                  `json:"query"`
	AssetScopeCount int                     `json:"asset_scope_count"`
	Results         []app.AIKnowledgeResult `json:"results"`
	Message         string                  `json:"message,omitempty"`
}

func (r *KnowledgeToolRegistry) handleSearchKnowledge(ctx context.Context, userID uint, args json.RawMessage) (json.RawMessage, error) {
	var params searchKnowledgeArgs
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}
	params.Query = strings.TrimSpace(params.Query)
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if params.Limit <= 0 || params.Limit > 10 {
		params.Limit = knowledgeRecallLimit
	}

	sessionID, _ := ctx.Value(app.SessionIDKey).(uint)
	if sessionID == 0 {
		return nil, fmt.Errorf("session context is required for knowledge search")
	}
	session, err := r.sessionRepo.FindOwnedByID(sessionID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve current session: %w", err)
	}
	assetIDs, err := r.agentKnowledgeAssetIDs(session.AgentID)
	if err != nil {
		return nil, err
	}
	if len(assetIDs) == 0 {
		return json.Marshal(searchKnowledgeResult{
			Query:           params.Query,
			AssetScopeCount: 0,
			Results:         []app.AIKnowledgeResult{},
			Message:         "当前 Agent 未绑定知识库或知识图谱。",
		})
	}

	results, err := r.knowledgeSearcher.SearchKnowledgeWithContext(ctx, assetIDs, params.Query, params.Limit)
	if err != nil {
		return nil, err
	}
	return json.Marshal(searchKnowledgeResult{
		Query:           params.Query,
		AssetScopeCount: len(assetIDs),
		Results:         results,
	})
}

func (r *KnowledgeToolRegistry) agentKnowledgeAssetIDs(agentID uint) ([]uint, error) {
	kbIDs, err := r.agentRepo.GetKnowledgeBaseIDs(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to load agent knowledge base bindings: %w", err)
	}
	kgIDs, err := r.agentRepo.GetKnowledgeGraphIDs(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to load agent knowledge graph bindings: %w", err)
	}
	return uniqueUintSlice(append(kbIDs, kgIDs...)), nil
}
