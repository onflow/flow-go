package common

import (
	"fmt"
	"strings"
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

	require.Equal(t, expectedLevel2TestResult.Runs, actualLevel2TestResult.Runs, "runs not equal; test: "+expectedLevel2TestResult.Test)
	require.Equal(t, expectedLevel2TestResult.Passed, actualLevel2TestResult.Passed, "passed not equal; test: "+expectedLevel2TestResult.Test)
	require.Equal(t, expectedLevel2TestResult.Failed, actualLevel2TestResult.Failed, "failed not equal; test: "+expectedLevel2TestResult.Test)
	require.Equal(t, expectedLevel2TestResult.Skipped, actualLevel2TestResult.Skipped, "skipped not equal; test: "+expectedLevel2TestResult.Test)
	require.Equal(t, expectedLevel2TestResult.NoResult, actualLevel2TestResult.NoResult, "no result not equal; test: "+expectedLevel2TestResult.Test)

	require.Equal(t, expectedLevel2TestResult.FailureRate, actualLevel2TestResult.FailureRate, "failure rate not equal; test: "+expectedLevel2TestResult.Test)
	require.Equal(t, expectedLevel2TestResult.AverageDuration, actualLevel2TestResult.AverageDuration, "avg duration not equal; test: "+expectedLevel2TestResult.Test)
	require.Equal(t, len(expectedLevel2TestResult.Durations), len(actualLevel2TestResult.Durations), PrintLevel2TestResult(&actualLevel2TestResult, "duration list sizes don't match"))
	// skip checking all individual durations because
	// a. require.Contains() seems to have a bug for checking float values in a list
	// b. durations might be removed from level 2 in the future since they're not necessary
	// c. the main metrics involving duration is average duration and test runs which are both checked
}

func PrintLevel2TestResult(level2TestResult *Level2TestResult, message string) string {
	builder := strings.Builder{}
	builder.WriteString("*** Level 2 Test Result (not found) " + message + "***")
	builder.WriteString("\nTest: " + level2TestResult.Test)
	builder.WriteString("\nPackage: " + level2TestResult.Package)
	builder.WriteString("\nRuns: " + fmt.Sprintf("%d", level2TestResult.Runs))
	builder.WriteString("\nPassed: " + fmt.Sprintf("%d", level2TestResult.Passed))
	builder.WriteString("\nFailed: " + fmt.Sprintf("%d", level2TestResult.Failed))
	builder.WriteString("\nNo Result: " + fmt.Sprintf("%d", level2TestResult.NoResult))
	builder.WriteString("\nSkipped: " + fmt.Sprintf("%d", level2TestResult.Skipped))
	builder.WriteString("\nAvg Duration: " + fmt.Sprintf("%f", level2TestResult.AverageDuration))

	for i, duration := range level2TestResult.Durations {
		builder.WriteString("\nDurations[" + fmt.Sprintf("%d", i) + "]" + fmt.Sprintf("%f", duration))
	}

	return builder.String()
}
