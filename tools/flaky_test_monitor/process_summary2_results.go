package main

// models full level 2 summary of a test run from 1 or more level 1 test runs
type TestSummary2 struct {
	ResultFiles []struct {
		File string `json:"file"`
	} `json:"result_files"`
	Tests []struct {
		Test            string        `json:"test"`
		Package         string        `json:"package"`
		Passed          int           `json:"passed"`
		Failed          int           `json:"failed"`
		NoResult        int           `json:"no result"`
		NoResultFiles   []interface{} `json:"no_result_files"`
		FlakyRate       float32       `json:"flaky_rate"`
		AverageDuration float32       `json:"average_duration"`
	} `json:"tests"`
}

func processSummary2TestRun(level1Directory string) TestSummary2 {
	testSummary2 := TestSummary2{}
	//testSummary2.Tests = make([]Test, 0)
	//newTestResult.Output = make([]string, 0)
	return testSummary2
}
