package main

import (
	"encoding/json"
	"fmt"
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

const failuresDir = "./failures/"

// process level 1 summary files in a single directory and output level 2 summary
func processSummary2TestRun(level1Directory string) TestSummary2 {
	dirEntries, err := os.ReadDir(level1Directory)
	os.Mkdir(failuresDir, 0755)
	assertErrNil(err, "error reading level 1 directory")

	testSummary2 := TestSummary2{}
	testSummary2.TestResults = make(map[string]*TestResultSummary)

	// go through all level 1 summaries in a folder to create level 2 summary
	for i := 0; i < len(dirEntries); i++ {
		// read in each level 1 summary
		var level1TestRun TestRun

		level1JsonBytes, err := os.ReadFile(level1Directory + dirEntries[i].Name())
		assertErrNil(err, "error reading level 1 json")

		err = json.Unmarshal(level1JsonBytes, &level1TestRun)
		assertErrNil(err, "error unmarshalling level 1 test run")

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
					saveFailureMessage(testResult)
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

func saveFailureMessage(testResult TestResult) {
	if testResult.Result != "fail" {
		panic(fmt.Sprintf("unexpected test result: " + testResult.Result))
	}

	// sub-directory names should be the same - each sub directory corresponds to a failed test name
	err := os.Mkdir(failuresDir+testResult.Test, 0755)
	assertErrNil(err, "error creating sub-dir under failuresDir")

	// under each sub-directory, there should be 1 or more text files (failure1.txt, failure2.txt, etc)
	// that holds the raw failure message for that test

	dirEntries, err := os.ReadDir(failuresDir + testResult.Test)
	assertErrNil(err, "error reading sub-dir entries under failuresDir")

	// failure text files will be named "failure1.txt", "failure2.txt", etc so we want to know how
	// many failure files already exist in the sub-directory before creating the next one
	failureFile, err := os.Create(failuresDir + testResult.Test + "/" + fmt.Sprintf("failure%d.txt", len(dirEntries)+1))
	assertErrNil(err, "error creating failure file")

	for _, output := range testResult.Output {
		_, err = failureFile.WriteString(output)
		assertErrNil(err, "error writing to failure file")
	}
}

func postProcessTestSummary2(testSummary2 TestSummary2) {
	for _, testResultSummary := range testSummary2.TestResults {
		// calculate average duration for each test summary
		var durationSum float32 = 0
		for _, duration := range testResultSummary.Durations {
			durationSum += duration
		}
		testResultSummary.AverageDuration = convertTo2DecimalPlaces2(durationSum, testResultSummary.Runs)

		// calculate failure rate for each test summary
		testResultSummary.FailureRate = convertTo2DecimalPlaces(testResultSummary.Failed, testResultSummary.Runs)

	}
}
