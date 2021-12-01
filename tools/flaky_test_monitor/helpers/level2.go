package helpers

// models full level 2 summary of a test run from 1 or more level 1 test runs
type TestSummary2 struct {
	TestResults map[string]*TestResultSummary `json:"tests"`
}

// models all results from a specific test over many (level 1) test runs
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
