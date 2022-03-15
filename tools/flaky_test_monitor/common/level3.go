package common

// models full level 3 summary of a test run from 1 (single) level 2 test run
type Level3Summary struct {
	NoResults []Level2TestResult `json:"no_results"`

	// ordered list of tests (from level 2 summary) that:
	// a) met minimum failure threshold as specified in property file property `failures_threshold_percent`
	// b) is up to maximum slice size as specified in `failures_slice_max` in property file
	MostFailures []Level2TestResult `json:"most_failures"`

	// total # of tests (from level 2 summary) that:
	// a) met minimum failure threshold as specified in property file property `failures_threshold_percent`
	MostFailuresTotal int `json:"most_failures_total"`

	// ordered list of tests (from level 2 summary) that:
	// a) met minimum duration threshold as specified in property file property `duration_threshold_seconds`
	// b) is up to maximum slice size as specified in `duration_slice_max` in property file
	LongestRunning []Level2TestResult `json:"longest_running"`

	// total # of tests (from level 2 summary) that:
	// a) met minimum duration threshold as specified in property file property `duration_threshold_seconds`
	LongestRunningTotal int `json:"longest_running_total"`
}
