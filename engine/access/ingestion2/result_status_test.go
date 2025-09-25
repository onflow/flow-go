package ingestion2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type transitionTestCase struct {
	name     string
	from     ResultStatus
	to       ResultStatus
	expected bool
}

// TestResultStatus_IsValid tests the IsValid method for ResultStatus
func TestResultStatus_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		status   ResultStatus
		expected bool
	}{
		{
			name:     "certified status is valid",
			status:   ResultForCertifiedBlock,
			expected: true,
		},
		{
			name:     "finalized status is valid",
			status:   ResultForFinalizedBlock,
			expected: true,
		},
		{
			name:     "sealed status is valid",
			status:   ResultSealed,
			expected: true,
		},
		{
			name:     "zero status is invalid",
			status:   ResultStatus(0),
			expected: false,
		},
		{
			name:     "unknown status is invalid",
			status:   ResultStatus(999),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.IsValid())
		})
	}
}

func transitionTests() []transitionTestCase {
	return []transitionTestCase{
		// Same status transitions (always valid)
		{
			name:     "certified to certified",
			from:     ResultForCertifiedBlock,
			to:       ResultForCertifiedBlock,
			expected: true,
		},
		{
			name:     "finalized to finalized",
			from:     ResultForFinalizedBlock,
			to:       ResultForFinalizedBlock,
			expected: true,
		},
		{
			name:     "sealed to sealed",
			from:     ResultSealed,
			to:       ResultSealed,
			expected: true,
		},

		// Valid transitions from certified
		{
			name:     "certified to finalized",
			from:     ResultForCertifiedBlock,
			to:       ResultForFinalizedBlock,
			expected: true,
		},
		{
			name:     "certified to sealed",
			from:     ResultForCertifiedBlock,
			to:       ResultSealed,
			expected: true,
		},

		// Valid transitions from finalized
		{
			name:     "finalized to sealed",
			from:     ResultForFinalizedBlock,
			to:       ResultSealed,
			expected: true,
		},

		// Invalid transitions from certified
		{
			name:     "certified to zero",
			from:     ResultForCertifiedBlock,
			to:       ResultStatus(0),
			expected: false,
		},
		{
			name:     "certified to unknown",
			from:     ResultForCertifiedBlock,
			to:       ResultStatus(999),
			expected: false,
		},

		// Invalid transitions from finalized
		{
			name:     "finalized to certified",
			from:     ResultForFinalizedBlock,
			to:       ResultForCertifiedBlock,
			expected: false,
		},
		{
			name:     "finalized to zero",
			from:     ResultForFinalizedBlock,
			to:       ResultStatus(0),
			expected: false,
		},
		{
			name:     "finalized to unknown",
			from:     ResultForFinalizedBlock,
			to:       ResultStatus(999),
			expected: false,
		},

		// Invalid transitions from sealed (sealed is terminal)
		{
			name:     "sealed to certified",
			from:     ResultSealed,
			to:       ResultForCertifiedBlock,
			expected: false,
		},
		{
			name:     "sealed to finalized",
			from:     ResultSealed,
			to:       ResultForFinalizedBlock,
			expected: false,
		},
		{
			name:     "sealed to zero",
			from:     ResultSealed,
			to:       ResultStatus(0),
			expected: false,
		},
		{
			name:     "sealed to unknown",
			from:     ResultSealed,
			to:       ResultStatus(999),
			expected: false,
		},

		// Invalid transitions from zero/unknown status
		{
			name:     "zero to certified",
			from:     ResultStatus(0),
			to:       ResultForCertifiedBlock,
			expected: false,
		},
		{
			name:     "unknown to any status",
			from:     ResultStatus(999),
			to:       ResultForCertifiedBlock,
			expected: false,
		},
	}
}

// TestResultStatus_IsValidTransition tests the IsValidTransition method
func TestResultStatus_IsValidTransition(t *testing.T) {
	for _, tt := range transitionTests() {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.from.IsValidTransition(tt.to))
		})
	}
}

// TestResultStatusTracker_Set tests the Set only updates for valid transitions
func TestResultStatusTracker_Set(t *testing.T) {
	for _, tt := range transitionTests() {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewResultStatusTracker(tt.from)

			result := tracker.Set(tt.to)
			assert.Equal(t, tt.expected, result)
			if tt.expected {
				assert.Equal(t, tt.to, tracker.Value())
			} else {
				assert.Equal(t, tt.from, tracker.Value())
			}
		})
	}
}
