package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

// models full level 2 summary of a test run from 1 or more level 1 test runs
type TestSummary2 struct {
	// ResultFiles []struct {
	// 	File string `json:"file"`
	// } `json:"result_files"`
	//TestResults []TestResultSummary `json:"tests"`
	TestResults map[string]*TestResultSummary `json:"tests"`
	// Tests []struct {
	// 	Test            string        `json:"test"`
	// 	Package         string        `json:"package"`
	// 	Passed          int           `json:"passed"`
	// 	Failed          int           `json:"failed"`
	// 	NoResult        int           `json:"no result"`
	// 	NoResultFiles   []interface{} `json:"no_result_files"`
	// 	FlakyRate       float32       `json:"flaky_rate"`
	// 	AverageDuration float32       `json:"average_duration"`
	// } `json:"tests"`
}

// models all results from a specific test over many (level 1) test runs
type TestResultSummary struct {
	Test     string `json:"test"`
	Package  string `json:"package"`
	Passed   int    `json:"passed"`
	Failed   int    `json:"failed"`
	NoResult int    `json:"no_result"`
	//NoResultFiles   []interface{} `json:"no_result_files"`
	FlakyRate       float32 `json:"flaky_rate"`
	AverageDuration float32 `json:"average_duration"`
}

func processSummary2TestRun(level1Directory string) TestSummary2 {

	dirEntries, err := os.ReadDir(level1Directory)
	assertErrNil(err, "error reading level 1 directory: ")

	testSummary2 := TestSummary2{}
	//testSummary2.TestResults = make([]TestResultSummary, 0)
	testSummary2.TestResults = make(map[string]*TestResultSummary)

	//var level1TestRuns []TestRun
	//testResultSummaryMap := make(map[string]*TestResultSummary)

	// go through all level 1 summaries in a folder to create level 2 summary
	for i := 0; i < len(dirEntries); i++ {
		fmt.Println("dirEntry:" + dirEntries[i].Name())

		// read in each level 1 summary
		var level1TestRun TestRun

		level1JsonBytes, err := ioutil.ReadFile(level1Directory + dirEntries[i].Name())
		assertErrNil(err, "error reading level 1 json: ")

		err = json.Unmarshal(level1JsonBytes, &level1TestRun)
		assertErrNil(err, "error unmarshalling level 1 test run: ")

		// go through each level 1 summary and update level 2 summary
		for _, packageResult := range level1TestRun.PackageResults {
			for _, testResult := range packageResult.Tests {
				//check if already started collecting summary for this test
				mapKey := testResult.Package + "/" + testResult.Test
				testResultSummary, testResultSummaryExists := testSummary2.TestResults[mapKey]

				//this test doesn't have a summary so create it
				if !testResultSummaryExists {
					testResultSummary = &TestResultSummary{
						Test:            testResult.Test,
						Package:         testResult.Package,
						Passed:          0,
						Failed:          0,
						NoResult:        0,
						FlakyRate:       0.0,
						AverageDuration: 0.0,
					}
				}

				// increment # of passes, fails or no results for this test
				switch testResult.Result {
				case "pass":
					testResultSummary.Passed++
				case "fail":
					testResultSummary.Failed++
				case "":
					testResultSummary.NoResult++
				}

				testSummary2.TestResults[mapKey] = testResultSummary
			}
		}

		//keep building list of level 1 test runs that can go through and create level 2 summary
		//level1TestRuns = append(level1TestRuns, level1TestRun)
	}

	//testSummary2.Tests = make([]Test, 0)
	//newTestResult.Output = make([]string, 0)
	return testSummary2
}
