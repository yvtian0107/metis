package contract

import "testing"

func TestCoreConstantsExposeSingleContractVocabulary(t *testing.T) {
	if TicketStatusDecisioning != "decisioning" {
		t.Fatalf("TicketStatusDecisioning = %q", TicketStatusDecisioning)
	}
	if ActivityStatusPending != "pending" {
		t.Fatalf("ActivityStatusPending = %q", ActivityStatusPending)
	}
	if EngineTypeSmart != "smart" || EngineTypeClassic != "classic" {
		t.Fatalf("unexpected engine types: %q %q", EngineTypeSmart, EngineTypeClassic)
	}
	if NodeTypeTimer.IsExecutable() {
		t.Fatalf("timer node must not be executable until backend support exists")
	}
	if !NodeTypeAction.IsExecutable() {
		t.Fatalf("action node must be executable")
	}
	if ServiceDeskStageAwaitingConfirmation != "awaiting_confirmation" {
		t.Fatalf("ServiceDeskStageAwaitingConfirmation = %q", ServiceDeskStageAwaitingConfirmation)
	}
	if SurfaceTypeITSMDraftForm != "itsm.draft_form" {
		t.Fatalf("SurfaceTypeITSMDraftForm = %q", SurfaceTypeITSMDraftForm)
	}
}
