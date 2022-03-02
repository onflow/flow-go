package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Level2Summary models full level 2 summary of multiple tests from 1 or more level 1 test runs
type Level2Summary struct {
	TestResultsMap map[string]*Level2TestResult `json:"tests"`
}

// Level2TestResult models all results from a specific test over many (level 1) test runs
type Level2TestResult struct {
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

// AssertLevel2TestResults checks that 2 Level2TestResult structs are equal, doing a deep comparison.
// This is needed for multiple test files (level 2 and 3) so it's extracted as a helper function here.
func AssertLevel2TestResults(t *testing.T, expectedLevel2TestResult, actualLevel2TestResult Level2TestResult) {
	require.Equal(t, expectedLevel2TestResult.Package, actualLevel2TestResult.Package)
	require.Equal(t, expectedLevel2TestResult.Test, actualLevel2TestResult.Test)

	require.Equal(t, expectedLevel2TestResult.Runs, actualLevel2TestResult.Runs, "test: "+expectedLevel2TestResult.Test)
	require.Equal(t, expectedLevel2TestResult.Passed, actualLevel2TestResult.Passed)
	require.Equal(t, expectedLevel2TestResult.Failed, actualLevel2TestResult.Failed)
	require.Equal(t, expectedLevel2TestResult.Skipped, actualLevel2TestResult.Skipped)
	require.Equal(t, expectedLevel2TestResult.NoResult, actualLevel2TestResult.NoResult)

	require.Equal(t, expectedLevel2TestResult.FailureRate, actualLevel2TestResult.FailureRate)

	// check all durations
	require.Equal(t, len(expectedLevel2TestResult.Durations), len(actualLevel2TestResult.Durations))
	for i := range expectedLevel2TestResult.Durations {
		require.Equal(t, expectedLevel2TestResult.Durations[i], actualLevel2TestResult.Durations[i])
	}
	require.Equal(t, expectedLevel2TestResult.AverageDuration, actualLevel2TestResult.AverageDuration)
}
