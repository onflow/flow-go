package synchronization

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestShouldReportProbabilistically tests the exact decision boundary of the probabilistic spam
// reporting: the random draw n is sampled uniformly from [0, spamProbabilityMultiplier), and a
// report is created iff n < prob*spamProbabilityMultiplier. These deterministic boundary cases
// complement the statistical load tests in engine_spam_test.go, which retain only
// high-probability wiring smoke groups and cannot exercise small-probability behavior.
func TestShouldReportProbabilistically(t *testing.T) {
	testCases := []struct {
		name     string
		n        uint32
		prob     float32
		expected bool
	}{
		// prob = 0 must never report, for any draw
		{"prob 0, smallest draw", 0, 0.0, false},
		{"prob 0, largest draw", spamProbabilityMultiplier - 1, 0.0, false},

		// prob = 1 must always report, for any draw
		{"prob 1, smallest draw", 0, 1.0, true},
		{"prob 1, largest draw", spamProbabilityMultiplier - 1, 1.0, true},

		// any prob >= 1/spamProbabilityMultiplier must report for the draw n = 0:
		// an implementation that never reports is detected by this case
		{"smallest meaningful prob, smallest draw", 0, 1.0 / spamProbabilityMultiplier, true},
		{"smallest meaningful prob, draw at boundary", 1, 1.0 / spamProbabilityMultiplier, false},

		// exact boundary at prob = 0.5: draws 0..499 report, draws 500.. do not
		{"prob 0.5, draw below boundary", 499, 0.5, true},
		{"prob 0.5, draw at boundary", 500, 0.5, false},

		// exact boundary at prob = 0.1: draws 0..99 report, draws 100.. do not
		{"prob 0.1, draw below boundary", 99, 0.1, true},
		{"prob 0.1, draw at boundary", 100, 0.1, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, shouldReportProbabilistically(tc.n, tc.prob))
		})
	}
}

// TestBatchRequestMisbehaviorProbability pins the scaling formula
// `baseProb * (len(blockIDs) + 1) / MaxSize` (MaxSize = 64) to the documented example values.
func TestBatchRequestMisbehaviorProbability(t *testing.T) {
	testCases := []struct {
		name     string
		baseProb float32
		numIDs   int
		expected float32
	}{
		{"single block ID", 0.1, 1, 0.003125},
		{"small batch", 0.1, 10, 0.0171875},
		{"large batch", 0.1, 99, 0.15625},
		{"small batch, small base prob", 0.01, 10, 0.00171875},
		{"very large batch, small base prob", 0.01, 999, 0.15625},
		{"zero base prob", 0.0, 1000, 0.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &flow.BatchRequest{BlockIDs: unittest.IdentifierListFixture(tc.numIDs)}
			require.Equal(t, tc.expected, batchRequestMisbehaviorProbability(tc.baseProb, req))
		})
	}
}

// TestRangeRequestMisbehaviorProbability pins the scaling formula
// `baseProb * (toHeight - fromHeight + 1) / MaxSize` (MaxSize = 64) to the documented example values.
func TestRangeRequestMisbehaviorProbability(t *testing.T) {
	testCases := []struct {
		name       string
		baseProb   float32
		fromHeight uint64
		toHeight   uint64
		expected   float32
	}{
		{"flat range (from == to)", 0.01, 1, 1, 0.00015625},
		{"single block range", 0.1, 9, 10, 0.003125},
		{"small range", 0.1, 1, 11, 0.0171875},
		{"large range", 0.1, 1, 100, 0.15625},
		{"small range, small base prob", 0.01, 1, 11, 0.00171875},
		{"very large range, small base prob", 0.01, 1, 1000, 0.15625},
		{"zero base prob", 0.0, 1, 1000, 0.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &flow.RangeRequest{FromHeight: tc.fromHeight, ToHeight: tc.toHeight}
			require.Equal(t, tc.expected, rangeRequestMisbehaviorProbability(tc.baseProb, req))
		})
	}
}
