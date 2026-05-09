package tools

import (
	"fmt"

	"metis/internal/app/itsm/contract"
)

// ServiceDeskState represents the multi-turn conversation state for the service desk flow.
type ServiceDeskState struct {
	Stage                   string         `json:"stage"` // idle|candidates_ready|service_selected|service_loaded|awaiting_confirmation|confirmed|submitted
	CandidateServiceIDs     []uint         `json:"candidate_service_ids,omitempty"`
	TopMatchServiceID       uint           `json:"top_match_service_id,omitempty"`
	ConfirmedServiceID      uint           `json:"confirmed_service_id,omitempty"`
	ConfirmationRequired    bool           `json:"confirmation_required"`
	LoadedServiceID         uint           `json:"loaded_service_id,omitempty"`
	ServiceVersionID        uint           `json:"service_version_id,omitempty"`
	ServiceVersionHash      string         `json:"service_version_hash,omitempty"`
	DraftSummary            string         `json:"draft_summary,omitempty"`
	DraftFormData           map[string]any `json:"draft_form_data,omitempty"`
	RequestText             string         `json:"request_text,omitempty"`
	PrefillFormData         map[string]any `json:"prefill_form_data,omitempty"`
	DraftVersion            int            `json:"draft_version"`
	ConfirmedDraftVersion   int            `json:"confirmed_draft_version"`
	FieldsHash              string         `json:"fields_hash,omitempty"`
	MissingFields           []string       `json:"missing_fields,omitempty"`
	AskedFields             []string       `json:"asked_fields,omitempty"`
	MinDecisionReady        bool           `json:"min_decision_ready"`
	PendingNextRequiredTool string         `json:"pending_next_required_tool,omitempty"`
}

var validTransitions = map[string][]string{
	string(contract.ServiceDeskStageIdle):                 {string(contract.ServiceDeskStageCandidatesReady)},
	string(contract.ServiceDeskStageCandidatesReady):      {string(contract.ServiceDeskStageCandidatesReady), string(contract.ServiceDeskStageServiceSelected), string(contract.ServiceDeskStageServiceLoaded)},
	string(contract.ServiceDeskStageServiceSelected):      {string(contract.ServiceDeskStageCandidatesReady), string(contract.ServiceDeskStageServiceLoaded)},
	string(contract.ServiceDeskStageServiceLoaded):        {string(contract.ServiceDeskStageCandidatesReady), string(contract.ServiceDeskStageAwaitingConfirmation)},
	string(contract.ServiceDeskStageAwaitingConfirmation): {string(contract.ServiceDeskStageCandidatesReady), string(contract.ServiceDeskStageConfirmed), string(contract.ServiceDeskStageSubmitted), string(contract.ServiceDeskStageAwaitingConfirmation)},
	string(contract.ServiceDeskStageConfirmed):            {string(contract.ServiceDeskStageCandidatesReady), string(contract.ServiceDeskStageSubmitted)},
	string(contract.ServiceDeskStageSubmitted):            {string(contract.ServiceDeskStageCandidatesReady)},
}

// TransitionTo validates and performs a stage transition.
// Transition to idle is always allowed because it resets a conversation.
func (s *ServiceDeskState) TransitionTo(next string) error {
	if next == string(contract.ServiceDeskStageIdle) {
		s.Stage = next
		return nil
	}
	allowed, ok := validTransitions[s.Stage]
	if !ok {
		return fmt.Errorf("invalid transition from %q to %q: current stage unknown", s.Stage, next)
	}
	for _, a := range allowed {
		if a == next {
			s.Stage = next
			return nil
		}
	}
	return fmt.Errorf("invalid transition from %q to %q", s.Stage, next)
}

func defaultState() *ServiceDeskState {
	return &ServiceDeskState{Stage: string(contract.ServiceDeskStageIdle)}
}
