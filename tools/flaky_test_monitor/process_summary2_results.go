package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

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
	NoResult        int       `json:"no_result"`
	FailureRate     float32   `json:"failure_rate"`
	AverageDuration float32   `json:"average_duration"`
	Durations       []float32 `json:"durations"`
}

func processSummary2TestRun(level1Directory string) TestSummary2 {

	dirEntries, err := os.ReadDir(level1Directory)
	assertErrNil(err, "error reading level 1 directory: ")

	testSummary2 := TestSummary2{}
	testSummary2.TestResults = make(map[string]*TestResultSummary)

	// go through all level 1 summaries in a folder to create level 2 summary
	for i := 0; i < len(dirEntries); i++ {
		// read in each level 1 summary
		var level1TestRun TestRun

		level1JsonBytes, err := ioutil.ReadFile(level1Directory + dirEntries[i].Name())
		assertErrNil(err, "error reading level 1 json: ")

		err = json.Unmarshal(level1JsonBytes, &level1TestRun)
		assertErrNil(err, "error unmarshalling level 1 test run: ")

		// go through each level 1 summary and update level 2 summary
		for _, packageResult := range level1TestRun.PackageResults {
			for _, testResult := range packageResult.Tests {
				// check if already started collecting summary for this test
				mapKey := testResult.Package + "/" + testResult.Test
				testResultSummary, testResultSummaryExists := testSummary2.TestResults[mapKey]

				// this test doesn't have a summary so create it
				// no need to specify other fields explicitly - default values will suffice
				if !testResultSummaryExists {
					testResultSummary = &TestResultSummary{
						Test:    testResult.Test,
						Package: testResult.Package,
					}
				}

				// increment # of passes, fails or no results for this test
				testResultSummary.Runs++
				switch testResult.Result {
				case "pass":
					testResultSummary.Passed++
				case "fail":
					testResultSummary.Failed++
				case "":
					testResultSummary.NoResult++
				default:
					panic(fmt.Sprintf("unexpected test result: %s", testResult.Result))
				}

				// keep track of each duration so can later calculate average duration
				testResultSummary.Durations = append(testResultSummary.Durations, testResult.Elapsed)

				testSummary2.TestResults[mapKey] = testResultSummary
			}
		}
	}

	// calculate averages and other calculations that can only be completed after all test runs have been read
	postProcessTestSummary2(testSummary2)
	return testSummary2
}

func postProcessTestSummary2(testSummary2 TestSummary2) {
	for _, testResultSummary := range testSummary2.TestResults {
		// calculate average duration for each test summary
		var durationSum float32 = 0
		for _, duration := range testResultSummary.Durations {
			durationSum += duration
		}
		testResultSummary.AverageDuration = durationSum / float32(testResultSummary.Runs)

		// calculate failure rate for each test summary
		testResultSummary.FailureRate = float32(testResultSummary.Failed/testResultSummary.Runs) * 100
	}
}
