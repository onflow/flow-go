package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestsLevel2Summary models full level 2 summary of multiple tests from 1 or more level 1 test runs
type TestsLevel2Summary struct {
	TestResults map[string]*TestRunsLevel2Summary `json:"tests"`
}

// TestRunsLevel2Summary models all results from a specific test over many (level 1) test runs
type TestRunsLevel2Summary struct {
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
func AssertTestSummariesEqual(t *testing.T, expectedTestRunsLevel2Summary, actualTestRunsLevel2Summary TestRunsLevel2Summary) {
	require.Equal(t, expectedTestRunsLevel2Summary.Package, actualTestRunsLevel2Summary.Package)
	require.Equal(t, expectedTestRunsLevel2Summary.Test, actualTestRunsLevel2Summary.Test)

	require.Equal(t, expectedTestRunsLevel2Summary.Runs, actualTestRunsLevel2Summary.Runs)
	require.Equal(t, expectedTestRunsLevel2Summary.Passed, actualTestRunsLevel2Summary.Passed)
	require.Equal(t, expectedTestRunsLevel2Summary.Failed, actualTestRunsLevel2Summary.Failed)
	require.Equal(t, expectedTestRunsLevel2Summary.Skipped, actualTestRunsLevel2Summary.Skipped)
	require.Equal(t, expectedTestRunsLevel2Summary.NoResult, actualTestRunsLevel2Summary.NoResult)

	require.Equal(t, expectedTestRunsLevel2Summary.FailureRate, actualTestRunsLevel2Summary.FailureRate)

	// check all durations
	require.Equal(t, len(expectedTestRunsLevel2Summary.Durations), len(actualTestRunsLevel2Summary.Durations))
	for i := range expectedTestRunsLevel2Summary.Durations {
		require.Equal(t, expectedTestRunsLevel2Summary.Durations[i], actualTestRunsLevel2Summary.Durations[i])
	}
	require.Equal(t, expectedTestRunsLevel2Summary.AverageDuration, actualTestRunsLevel2Summary.AverageDuration)
}
