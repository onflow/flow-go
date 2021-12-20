package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSummary2 models full level 2 summary of a test run from 1 or more level 1 test runs
type TestSummary2 struct {
	TestResults map[string]*TestResultSummary `json:"tests"`
}

// TestResultSummary models all results from a specific test over many (level 1) test runs
type TestResultSummary struct {
	Test            string    `json:"test"`
	Package         string    `json:"package"`
	Runs            int       `json:"runs"`
	Passed          int       `json:"passed"`
	Failed          int       `json:"failed"`
	Skipped         int       `json:"skipped"`
	NoResult        int       `json:"no_result"`
	FailureRate     float32   `json:"failure_rate"`
	AverageDuration float32   `json:"average_duration"`
	Durations       []float32 `json:"durations"`
}

// AssertTestSummariesEqual checks that 2 TestResultSummary structs are equal, doing a deep comparison.
// This is needed for multiple test files (level 2 and 3) so it's extracted as a helper function here.
func AssertTestSummariesEqual(t *testing.T, expectedTestSummary, actualTestSummary *TestResultSummary) {
	require.Equal(t, expectedTestSummary.Package, actualTestSummary.Package)
	require.Equal(t, expectedTestSummary.Test, actualTestSummary.Test)

	require.Equal(t, expectedTestSummary.Runs, actualTestSummary.Runs)
	require.Equal(t, expectedTestSummary.Passed, actualTestSummary.Passed)
	require.Equal(t, expectedTestSummary.Failed, actualTestSummary.Failed)
	require.Equal(t, expectedTestSummary.Skipped, actualTestSummary.Skipped)
	require.Equal(t, expectedTestSummary.NoResult, actualTestSummary.NoResult)

	require.Equal(t, expectedTestSummary.FailureRate, actualTestSummary.FailureRate)

	// check all durations
	require.Equal(t, len(expectedTestSummary.Durations), len(actualTestSummary.Durations))
	for i := range expectedTestSummary.Durations {
		require.Equal(t, expectedTestSummary.Durations[i], actualTestSummary.Durations[i])
	}
	require.Equal(t, expectedTestSummary.AverageDuration, actualTestSummary.AverageDuration)
}
