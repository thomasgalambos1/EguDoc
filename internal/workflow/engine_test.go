// internal/workflow/engine_test.go
package workflow_test

import (
	"testing"

	"github.com/eguilde/egudoc/internal/workflow"
)

func TestValidTransitions(t *testing.T) {
	tests := []struct {
		fromStatus string
		action     workflow.Action
		wantStatus string
		wantErr    bool
	}{
		{"INREGISTRAT", workflow.ActionAssignCompartiment, "ALOCAT_COMPARTIMENT", false},
		{"ALOCAT_COMPARTIMENT", workflow.ActionAssignUser, "IN_LUCRU", false},
		{"IN_LUCRU", workflow.ActionSendForApproval, "FLUX_APROBARE", false},
		{"FLUX_APROBARE", workflow.ActionApprove, "FINALIZAT", false},
		{"FLUX_APROBARE", workflow.ActionReject, "IN_LUCRU", false},
		{"FINALIZAT", workflow.ActionApprove, "", true},
		{"IN_LUCRU", workflow.ActionApprove, "", true},
	}

	for _, tt := range tests {
		transitions, ok := workflow.ValidTransitions[tt.fromStatus]
		if !ok {
			if !tt.wantErr {
				t.Errorf("status %q has no transitions", tt.fromStatus)
			}
			continue
		}
		newStatus, ok := transitions[tt.action]
		if !ok {
			if !tt.wantErr {
				t.Errorf("from=%q action=%q: expected transition but got none", tt.fromStatus, tt.action)
			}
			continue
		}
		if tt.wantErr {
			t.Errorf("from=%q action=%q: expected error but got %q", tt.fromStatus, tt.action, newStatus)
		}
		if newStatus != tt.wantStatus {
			t.Errorf("from=%q action=%q: want %q got %q", tt.fromStatus, tt.action, tt.wantStatus, newStatus)
		}
	}
}
