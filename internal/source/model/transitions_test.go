package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     LoopState
		to       LoopState
		expected bool
	}{
		// Valid transitions
		{"Unresolved to Running", LoopStateUnresolved, LoopStateRunning, true},
		{"Unresolved to Cancelled", LoopStateUnresolved, LoopStateCancelled, true},
		{"Running to Synced", LoopStateRunning, LoopStateSynced, true},
		{"Running to Flatline", LoopStateRunning, LoopStateFlatline, true},
		{"Running to Unresolved", LoopStateRunning, LoopStateUnresolved, true},
		{"Running to Cancelled", LoopStateRunning, LoopStateCancelled, true},

		// Invalid transitions from Unresolved
		{"Unresolved to Synced", LoopStateUnresolved, LoopStateSynced, false},
		{"Unresolved to Flatline", LoopStateUnresolved, LoopStateFlatline, false},
		{"Unresolved to Unresolved", LoopStateUnresolved, LoopStateUnresolved, false},

		// Invalid transitions from Running
		{"Running to Running", LoopStateRunning, LoopStateRunning, false},

		// Invalid transitions from Synced (terminal or no outgoing transitions defined)
		{"Synced to Unresolved", LoopStateSynced, LoopStateUnresolved, false},
		{"Synced to Running", LoopStateSynced, LoopStateRunning, false},
		{"Synced to Flatline", LoopStateSynced, LoopStateFlatline, false},
		{"Synced to Cancelled", LoopStateSynced, LoopStateCancelled, false},

		// Invalid transitions from Flatline
		{"Flatline to Unresolved", LoopStateFlatline, LoopStateUnresolved, false},
		{"Flatline to Running", LoopStateFlatline, LoopStateRunning, false},
		{"Flatline to Synced", LoopStateFlatline, LoopStateSynced, false},
		{"Flatline to Cancelled", LoopStateFlatline, LoopStateCancelled, false},

		// Invalid transitions from Cancelled
		{"Cancelled to Unresolved", LoopStateCancelled, LoopStateUnresolved, false},
		{"Cancelled to Running", LoopStateCancelled, LoopStateRunning, false},
		{"Cancelled to Synced", LoopStateCancelled, LoopStateSynced, false},
		{"Cancelled to Flatline", LoopStateCancelled, LoopStateFlatline, false},

		// Unknown states
		{"Unknown to Running", LoopState("unknown_state"), LoopStateRunning, false},
		{"Running to Unknown", LoopStateRunning, LoopState("unknown_state"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidTransition(tt.from, tt.to)
			assert.Equal(t, tt.expected, result)
		})
	}
}
