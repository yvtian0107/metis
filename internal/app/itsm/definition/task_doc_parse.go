package definition

import (
	"context"
	"encoding/json"
	"fmt"
)

// HandleDocParse returns a scheduler task handler that parses knowledge documents.
func HandleDocParse(svc *KnowledgeDocService) func(ctx context.Context, payload json.RawMessage) error {
	return func(ctx context.Context, payload json.RawMessage) error {
		var p struct {
			DocumentID uint `json:"document_id"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("unmarshal payload: %w", err)
		}
		if p.DocumentID == 0 {
			return fmt.Errorf("document_id is required")
		}
		return svc.ParseDocument(p.DocumentID)
	}
}
