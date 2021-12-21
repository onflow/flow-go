package common

// models full level 3 summary of a test run from 1 (single) level 2 test run
type TestSummary3 struct {
	NoResults      []TestResultSummary `json:"no_results"`
	MostFailures   []TestResultSummary `json:"most_failures"`
	LongestRunning []TestResultSummary `json:"longest_running"`
}
