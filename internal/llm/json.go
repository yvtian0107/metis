package llm

import (
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/kaptinlin/jsonrepair"
)

// ExtractJSON strips markdown code fences and repairs malformed JSON from LLM output.
// Uses jsonrepair to handle common LLM issues: trailing commas, single quotes,
// comments, truncated JSON, missing closing brackets, etc.
func ExtractJSON(content string) string {
	jsonStr := content

	// Strip markdown code fences
	if idx := strings.Index(content, "```json"); idx != -1 {
		start := idx + 7
		end := strings.Index(content[start:], "```")
		if end != -1 {
			jsonStr = content[start : start+end]
		}
	} else if idx := strings.Index(content, "```"); idx != -1 {
		start := idx + 3
		if nl := strings.Index(content[start:], "\n"); nl != -1 {
			start = start + nl + 1
		}
		end := strings.Index(content[start:], "```")
		if end != -1 {
			jsonStr = content[start : start+end]
		}
	}

	jsonStr = strings.TrimSpace(jsonStr)

	if jsonStr == "" {
		return ""
	}

	// Try standard parse first — skip repair if JSON is already valid
	if json.Valid([]byte(jsonStr)) {
		return jsonStr
	}

	// Repair malformed JSON
	repaired, err := jsonrepair.Repair(jsonStr)
	if err != nil {
		slog.Debug("json repair failed, returning original", "error", err, "preview", truncateStr(jsonStr, 200))
		return jsonStr
	}
	return repaired
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
