package main

import (
	"encoding/json"
	"flaky-test-monitor/helpers"
	"fmt"
	"os"
)

const failuresDir = "./failures/"

// process level 1 summary files in a single directory and output level 2 summary
func processSummary2TestRun(level1Directory string) helpers.TestSummary2 {
	dirEntries, err := os.ReadDir(level1Directory)
	helpers.AssertErrNil(err, "error reading level 1 directory")
	err = os.Mkdir(failuresDir, 0755)
	helpers.AssertErrNil(err, "error creating failures directory")

	testSummary2 := helpers.TestSummary2{}
	testSummary2.TestResults = make(map[string]*helpers.TestResultSummary)

	// go through all level 1 summaries in a folder to create level 2 summary
	for i := 0; i < len(dirEntries); i++ {
		// read in each level 1 summary
		var level1TestRun helpers.TestRun

		level1JsonBytes, err := os.ReadFile(level1Directory + dirEntries[i].Name())
		helpers.AssertErrNil(err, "error reading level 1 json")

		err = json.Unmarshal(level1JsonBytes, &level1TestRun)
		helpers.AssertErrNil(err, "error unmarshalling level 1 test run")

		// go through each level 1 summary and update level 2 summary
		for _, packageResult := range level1TestRun.PackageResults {
			for _, testResult := range packageResult.Tests {
				// check if already started collecting summary for this test
				mapKey := testResult.Package + "/" + testResult.Test
				testResultSummary, testResultSummaryExists := testSummary2.TestResults[mapKey]

				// this test doesn't have a summary so create it
				// no need to specify other fields explicitly - default values will suffice
				if !testResultSummaryExists {
					testResultSummary = &helpers.TestResultSummary{
						Test:    testResult.Test,
						Package: testResult.Package,
					}
				}

				// keep track of each duration so can later calculate average duration
				testResultSummary.Durations = append(testResultSummary.Durations, testResult.Elapsed)

				// increment # of passes, fails, skips or no results for this test
				testResultSummary.Runs++
				switch testResult.Result {
				case "pass":
					testResultSummary.Passed++
				case "fail":
					testResultSummary.Failed++
					saveFailureMessage(testResult)
				case "skip":
					testResultSummary.Skipped++

					// don't count skip as a run
					testResultSummary.Runs--

					// truncate last duration - don't count durations of skips
					testResultSummary.Durations = testResultSummary.Durations[:len(testResultSummary.Durations)-1]
				case "":
					testResultSummary.NoResult++
				default:
					panic(fmt.Sprintf("unexpected test result: %s", testResult.Result))
				}

				testSummary2.TestResults[mapKey] = testResultSummary
			}
		}
	}

	// calculate averages and other calculations that can only be completed after all test runs have been read
	postProcessTestSummary2(testSummary2)
	return testSummary2
}

func saveFailureMessage(testResult helpers.TestResult) {
	if testResult.Result != "fail" {
		panic(fmt.Sprintf("unexpected test result: " + testResult.Result))
	}

	// sub-directory names should be the same - each sub directory corresponds to a failed test name
	// there could already be previous failures for this test, so it's important to check if failed
	// test folder exists
	if !helpers.FolderExists(failuresDir + testResult.Test) {
		err := os.Mkdir(failuresDir+testResult.Test, 0755)
		helpers.AssertErrNil(err, "error creating sub-dir under failuresDir")
	}

	// under each sub-directory, there should be 1 or more text files (failure1.txt, failure2.txt, etc)
	// that holds the raw failure message for that test

	dirEntries, err := os.ReadDir(failuresDir + testResult.Test)
	helpers.AssertErrNil(err, "error reading sub-dir entries under failuresDir")

	// failure text files will be named "failure1.txt", "failure2.txt", etc so we want to know how
	// many failure files already exist in the sub-directory before creating the next one
	failureFile, err := os.Create(failuresDir + testResult.Test + "/" + fmt.Sprintf("failure%d.txt", len(dirEntries)+1))
	helpers.AssertErrNil(err, "error creating failure file")

	for _, output := range testResult.Output {
		_, err = failureFile.WriteString(output)
		helpers.AssertErrNil(err, "error writing to failure file")
	}
}

func postProcessTestSummary2(testSummary2 helpers.TestSummary2) {
	for _, testResultSummary := range testSummary2.TestResults {
		// calculate average duration for each test summary
		var durationSum float32 = 0
		for _, duration := range testResultSummary.Durations {
			durationSum += duration
		}
		testResultSummary.AverageDuration = helpers.ConvertTo2DecimalPlaces2(durationSum, testResultSummary.Runs)

		// calculate failure rate for each test summary
		testResultSummary.FailureRate = helpers.ConvertTo2DecimalPlaces(testResultSummary.Failed, testResultSummary.Runs)
	}
}

func main() {
	// need to pass in single argument of where level 1 summary files exist
	if len(os.Args[1:]) != 1 {
		panic("wrong number of arguments - expected single argument with directory of level 1 files")
	}

	if !helpers.FolderExists(os.Args[1]) {
		panic("directory doesn't exist")
	}

	testSummary2 := processSummary2TestRun(helpers.AddTrailingSlash(os.Args[1]))
	helpers.SaveToFile("level2-summary.json", testSummary2)
}
