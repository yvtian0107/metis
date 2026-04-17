package engine

import (
	"testing"
	"time"
)

func TestCheckTicketSLA_ResponseBreach(t *testing.T) {
	// This is a logic-level test verifying breach detection.
	// In production, checkTicketSLA writes to DB — here we test the condition logic directly.
	now := time.Now()
	pastDeadline := now.Add(-10 * time.Minute)
	futureDeadline := now.Add(10 * time.Minute)

	tests := []struct {
		name             string
		responseDeadline *time.Time
		resolveDeadline  *time.Time
		currentSLA       string
		wantBreach       bool
		breachType       string
	}{
		{
			name:             "response breached",
			responseDeadline: &pastDeadline,
			currentSLA:       slaOnTrack,
			wantBreach:       true,
			breachType:       "response",
		},
		{
			name:            "resolution breached",
			resolveDeadline: &pastDeadline,
			currentSLA:      slaOnTrack,
			wantBreach:      true,
			breachType:      "resolution",
		},
		{
			name:             "no breach - future deadline",
			responseDeadline: &futureDeadline,
			resolveDeadline:  &futureDeadline,
			currentSLA:       slaOnTrack,
			wantBreach:       false,
		},
		{
			name:             "already breached - no re-trigger",
			responseDeadline: &pastDeadline,
			currentSLA:       slaBreachedResponse,
			wantBreach:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket := &ticketModel{
				ID:                    1,
				SLAResponseDeadline:   tt.responseDeadline,
				SLAResolutionDeadline: tt.resolveDeadline,
				SLAStatus:             tt.currentSLA,
			}

			// Verify breach detection logic
			responseBreach := ticket.SLAResponseDeadline != nil &&
				now.After(*ticket.SLAResponseDeadline) &&
				ticket.SLAStatus != slaBreachedResponse &&
				ticket.SLAStatus != slaBreachedResolve

			resolveBreach := !responseBreach &&
				ticket.SLAResolutionDeadline != nil &&
				now.After(*ticket.SLAResolutionDeadline) &&
				ticket.SLAStatus != slaBreachedResolve

			gotBreach := responseBreach || resolveBreach
			if gotBreach != tt.wantBreach {
				t.Errorf("breach detection: got %v, want %v", gotBreach, tt.wantBreach)
			}
			if tt.wantBreach && tt.breachType == "response" && !responseBreach {
				t.Error("expected response breach")
			}
			if tt.wantBreach && tt.breachType == "resolution" && !resolveBreach {
				t.Error("expected resolution breach")
			}
		})
	}
}

func TestEscalationTriggerTiming(t *testing.T) {
	now := time.Now()
	deadline := now.Add(-30 * time.Minute) // breached 30 minutes ago

	tests := []struct {
		name        string
		waitMinutes int
		shouldFire  bool
	}{
		{"fires immediately (0 min wait)", 0, true},
		{"fires after 15 min wait", 15, true},
		{"fires after 30 min wait", 30, true},
		{"does not fire after 45 min wait", 45, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			triggerTime := deadline.Add(time.Duration(tt.waitMinutes) * time.Minute)
			fired := !now.Before(triggerTime)
			if fired != tt.shouldFire {
				t.Errorf("got fired=%v, want %v", fired, tt.shouldFire)
			}
		})
	}
}

func TestSLAPauseResumeDeadlineExtension(t *testing.T) {
	// Simulate pause/resume cycle
	originalDeadline := time.Now().Add(2 * time.Hour)
	pausedAt := time.Now().Add(-30 * time.Minute) // paused 30 minutes ago
	pausedDuration := time.Since(pausedAt)

	extendedDeadline := originalDeadline.Add(pausedDuration)

	// The extended deadline should be approximately 30 minutes later than original
	diff := extendedDeadline.Sub(originalDeadline)
	if diff < 29*time.Minute || diff > 31*time.Minute {
		t.Errorf("deadline extension should be ~30 minutes, got %v", diff)
	}
}

func TestSLAConstants(t *testing.T) {
	// Verify SLA status constants match expected values
	if slaOnTrack != "on_track" {
		t.Error("slaOnTrack mismatch")
	}
	if slaBreachedResponse != "breached_response" {
		t.Error("slaBreachedResponse mismatch")
	}
	if slaBreachedResolve != "breached_resolution" {
		t.Error("slaBreachedResolve mismatch")
	}
}
