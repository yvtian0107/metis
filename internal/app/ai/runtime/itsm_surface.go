package runtime

import (
	"encoding/json"
	"fmt"
)

const itsmDraftFormSurfaceType = "itsm.draft_form"

type itsmServiceLoadOutput struct {
	ServiceID uint   `json:"service_id"`
	Name      string `json:"name"`
	Engine    string `json:"engine_type"`
}

type itsmDraftPrepareOutput struct {
	OK                   bool           `json:"ok"`
	ReadyForConfirmation bool           `json:"ready_for_confirmation"`
	ServiceID            uint           `json:"service_id"`
	ServiceName          string         `json:"service_name"`
	ServiceEngineType    string         `json:"service_engine_type"`
	DraftVersion         int            `json:"draft_version"`
	Summary              string         `json:"summary"`
	FormData             map[string]any `json:"form_data"`
	FormSchema           any            `json:"form_schema"`
}

func itsmDraftSurfaceID(toolCallID string) string {
	if toolCallID == "" {
		return "itsm-draft-form"
	}
	return fmt.Sprintf("itsm-draft-form-%s", toolCallID)
}

func makeITSMDraftLoadingSurface(toolCallID string) Event {
	data, _ := json.Marshal(map[string]any{
		"status": "loading",
		"title":  "正在整理草稿",
	})
	return Event{
		Type:        EventTypeUISurface,
		SurfaceID:   itsmDraftSurfaceID(toolCallID),
		SurfaceType: itsmDraftFormSurfaceType,
		SurfaceData: data,
	}
}

func makeITSMDraftReadySurface(toolCallID string, output string) (Event, bool) {
	var resp itsmDraftPrepareOutput
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return Event{}, false
	}
	if !resp.OK || !resp.ReadyForConfirmation || resp.ServiceEngineType != "smart" {
		return Event{}, false
	}
	data, _ := json.Marshal(map[string]any{
		"status":       "ready",
		"serviceId":    resp.ServiceID,
		"title":        resp.ServiceName,
		"summary":      resp.Summary,
		"schema":       resp.FormSchema,
		"values":       resp.FormData,
		"draftVersion": resp.DraftVersion,
		"submitAction": map[string]any{
			"method": "POST",
			"kind":   "itsm.draft.submit",
		},
	})
	return Event{
		Type:        EventTypeUISurface,
		SurfaceID:   itsmDraftSurfaceID(toolCallID),
		SurfaceType: itsmDraftFormSurfaceType,
		SurfaceData: data,
	}, true
}

func parseITSMServiceEngine(output string) string {
	var resp itsmServiceLoadOutput
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return ""
	}
	return resp.Engine
}
