package ingestion2

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/utils/slices"
)

var allStatuses = []ResultStatus{
	// valid
	ResultForCertifiedBlock,
	ResultForFinalizedBlock,
	ResultSealed,
	ResultOrphaned,

	// invalid (but permitted by the underlying `uint64` type
	ResultStatus(0),
	ResultStatus(ResultOrphaned + 1),
}

type transitionTestCase struct {
	name  string
	from  ResultStatus
	valid []ResultStatus
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
			name:     "orphaned status is valid",
			status:   ResultOrphaned,
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
		{
			from: ResultForCertifiedBlock,
			valid: []ResultStatus{
				ResultForCertifiedBlock,
				ResultForFinalizedBlock,
				ResultSealed,
				ResultOrphaned,
			},
		},
		{
			from: ResultForFinalizedBlock,
			valid: []ResultStatus{
				ResultForFinalizedBlock,
				ResultSealed,
				ResultOrphaned,
			},
		},
		{
			from: ResultSealed,
			valid: []ResultStatus{
				ResultSealed, // terminal state - sealed results cannot be orphaned
			},
		},
		{
			from: ResultOrphaned,
			valid: []ResultStatus{
				ResultOrphaned, // terminal state - cannot transition out of orphaned
			},
		},
	}
}

// TestResultStatus_IsValidTransition tests the IsValidTransition method
func TestResultStatus_IsValidTransition(t *testing.T) {
	for _, tt := range transitionTests() {
		validTransition := slices.ToMap(tt.valid)

		for _, to := range allStatuses {
			_, isValid := validTransition[to]

			t.Run(fmt.Sprintf("%s -> %s (valid: %t)", tt.from, to, isValid), func(t *testing.T) {
				assert.Equal(t, isValid, tt.from.IsValidTransition(to))
			})
		}
	}
}

// TestResultStatusTracker_Set tests the Set only updates for valid transitions
func TestResultStatusTracker_Set(t *testing.T) {
	for _, tt := range transitionTests() {
		validTransition := slices.ToMap(tt.valid)

		for _, to := range allStatuses {
			_, isValid := validTransition[to]

			tracker := NewResultStatusTracker(tt.from)
			t.Run(fmt.Sprintf("%s -> %s (valid: %t)", tt.from, to, isValid), func(t *testing.T) {
				assert.Equal(t, isValid, tracker.Set(to))
			})
		}
	}
}
