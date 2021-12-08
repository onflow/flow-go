package main

import (
	"encoding/json"
	"flaky-test-monitor/common"
	"fmt"
	"os"
	"strings"
)

const failuresDir = "./failures/"

// process level 1 summary files in a single directory and output level 2 summary
func processSummary2TestRun(level1Directory string) common.TestSummary2 {
	dirEntries, err := os.ReadDir(level1Directory)
	common.AssertNoError(err, "error reading level 1 directory")
	err = os.Mkdir(failuresDir, 0755)
	common.AssertNoError(err, "error creating failures directory")

	testSummary2 := common.TestSummary2{}
	testSummary2.TestResults = make(map[string]*common.TestResultSummary)

	// go through all level 1 summaries in a folder to create level 2 summary
	for i := 0; i < len(dirEntries); i++ {
		// read in each level 1 summary
		var level1TestRun common.TestRun

		level1JsonBytes, err := os.ReadFile(level1Directory + dirEntries[i].Name())
		common.AssertNoError(err, "error reading level 1 json")

		err = json.Unmarshal(level1JsonBytes, &level1TestRun)
		common.AssertNoError(err, "error unmarshalling level 1 test run")

		// go through each level 1 summary and update level 2 summary
		for _, packageResult := range level1TestRun.PackageResults {
			for _, testResult := range packageResult.Tests {
				// check if already started collecting summary for this test
				mapKey := testResult.Package + "/" + testResult.Test
				testResultSummary, testResultSummaryExists := testSummary2.TestResults[mapKey]

				// this test doesn't have a summary so create it
				// no need to specify other fields explicitly - default values will suffice
				if !testResultSummaryExists {
					testResultSummary = &common.TestResultSummary{
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

// for each failed test, we want to save the raw failed message as a text file
// there could be multiple failures of the same test so we want to save each failed message in a separate text file
// each test with failures will have a uniquely named (based on test name and package) sub-directory where failed messages are saved
// e.g. "failures/TestSanitySha3_256+github.com-onflow-flow-go-crypto-hash" will store failed messages text files
// from test TestSanitySha3_256 from the "github.com/onflow/flow-go/crypto/hash" package
func saveFailureMessage(testResult common.TestResult) {
	if testResult.Result != "fail" {
		panic(fmt.Sprintf("unexpected test result: " + testResult.Result))
	}

	// each sub directory corresponds to a failed test name and package name
	failuresDirFullPath := failuresDir + testResult.Test + "+" + strings.ReplaceAll(testResult.Package, "/", "-") + "/"

	// there could already be previous failures for this test, so it's important to check if failed
	// test folder exists
	if !common.DirExists(failuresDirFullPath) {
		err := os.Mkdir(failuresDirFullPath, 0755)
		common.AssertNoError(err, "error creating sub-dir under failuresDir")
	}

	// under each sub-directory, there should be 1 or more text files (failure1.txt, failure2.txt, etc)
	// that holds the raw failure message for that test
	dirEntries, err := os.ReadDir(failuresDirFullPath)
	common.AssertNoError(err, "error reading sub-dir entries under failuresDir")

	// failure text files will be named "failure1.txt", "failure2.txt", etc so we want to know how
	// many failure files already exist in the sub-directory before creating the next one
	failureFile, err := os.Create(failuresDirFullPath + fmt.Sprintf("failure%d.txt", len(dirEntries)+1))
	common.AssertNoError(err, "error creating failure file")

	for _, output := range testResult.Output {
		_, err = failureFile.WriteString(output)
		common.AssertNoError(err, "error writing to failure file")
	}
}

func postProcessTestSummary2(testSummary2 common.TestSummary2) {
	for _, testResultSummary := range testSummary2.TestResults {
		// calculate average duration for each test summary
		var durationSum float32 = 0
		for _, duration := range testResultSummary.Durations {
			durationSum += duration
		}
		testResultSummary.AverageDuration = common.ConvertToNDecimalPlaces2(2, durationSum, testResultSummary.Runs)

		// calculate failure rate for each test summary
		testResultSummary.FailureRate = common.ConvertToNDecimalPlaces(2, testResultSummary.Failed, testResultSummary.Runs)
	}

	// check if there are no failures so can delete failures sub-directory
	if common.IsDirEmpty(failuresDir) {
		err := os.RemoveAll(failuresDir)
		common.AssertNoError(err, "error removing failures directory")
	}

	// check if there are no nil tests (no results) so delete no-results sub-directory
}

// level 2 flaky test summary processor
// input: command line argument of directory where level 1 summary files exist
// output: level 2 summary json file that will be used as input for level 3 summary processor
func main() {
	// need to pass in single argument of where level 1 summary files exist
	if len(os.Args[1:]) != 1 {
		panic("wrong number of arguments - expected single argument with directory of level 1 files")
	}

	if !common.DirExists(os.Args[1]) {
		panic("directory doesn't exist")
	}

	testSummary2 := processSummary2TestRun(common.AddTrailingSlash(os.Args[1]))
	common.SaveToFile("level2-summary.json", testSummary2)
}
