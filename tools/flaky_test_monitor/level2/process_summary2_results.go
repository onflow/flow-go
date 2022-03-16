package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/onflow/flow-go/tools/flaky_test_monitor/common"
)

const failuresDir = "./failures/"
const noResultsDir = "./no-results/"

func generateLevel2SummaryFromStructs(level1Summaries []common.Level1Summary) common.Level2Summary {
	// create directory to store failure messages
	err := os.Mkdir(failuresDir, 0755)
	common.AssertNoError(err, "error creating failures directory")

	// create directory to store no-results messages
	err = os.Mkdir(noResultsDir, 0755)
	common.AssertNoError(err, "error creating no-results directory")

	level2Summary := common.Level2Summary{}
	level2Summary.TestResultsMap = make(map[string]*common.Level2TestResult)

	// go through all level 1 test runs create level 2 summary
	for i := 0; i < len(level1Summaries); i++ {
		for _, level1TestResultRow := range level1Summaries[i].Rows {
			// check if already started collecting summary for this test
			level2TestResultsMapKey := level1TestResultRow.TestResult.Package + "/" + level1TestResultRow.TestResult.Test
			level2TestResult, level2TestResultExists := level2Summary.TestResultsMap[level2TestResultsMapKey]

			// this test doesn't have a summary so create one
			// no need to specify other fields explicitly - default values will suffice
			if !level2TestResultExists {
				level2TestResult = &common.Level2TestResult{
					Test:    level1TestResultRow.TestResult.Test,
					Package: level1TestResultRow.TestResult.Package,
				}
			}
			// keep track of each duration so can later calculate average duration
			level2TestResult.Durations = append(level2TestResult.Durations, level1TestResultRow.TestResult.Elapsed)

			// increment # of passes, fails, skips or no-results for this test
			level2TestResult.Runs++
			switch level1TestResultRow.TestResult.Result {
			case "1":
				level2TestResult.Passed++
			case "0":
				level2TestResult.Failed++

				// for tests that don't have a result generated (e.g. using fmt.Printf() with no newline in a test)
				if level1TestResultRow.TestResult.NoResult {
					level2TestResult.NoResult++
					saveNoResultMessage(level1TestResultRow.TestResult)
				} else {
					saveFailureMessage(level1TestResultRow.TestResult)
				}
			case "skip":
				level2TestResult.Skipped++

				// don't count skip as a run
				level2TestResult.Runs--

				// truncate last duration - don't count durations of skips
				level2TestResult.Durations = level2TestResult.Durations[:len(level2TestResult.Durations)-1]
			default:
				panic(fmt.Sprintf("unexpected test result: %s", level1TestResultRow.TestResult.Result))
			}

			level2Summary.TestResultsMap[level2TestResultsMapKey] = level2TestResult
		}
	}
	// calculate averages and other calculations that can only be completed after all test runs have been read
	postProcessLevel2Summary(level2Summary)
	return level2Summary
}

// process level 1 summary files in a single directory and output level 2 summary
func generateLevel2Summary(level1Directory string) common.Level2Summary {
	level1Summaries := buildLevel1SummariesFromJSON(level1Directory)
	level2Summary := generateLevel2SummaryFromStructs(level1Summaries)
	return level2Summary
}

// buildLevel1SummariesFromJSON creates level 1 summaries so the same function can be used to process level 1
// summaries whether they were created from JSON files (used in production) or from pre-constructed level 1 summary structs (used by testing)
func buildLevel1SummariesFromJSON(level1Directory string) []common.Level1Summary {
	var level1Summaries []common.Level1Summary
	dirEntries, err := os.ReadDir(filepath.Join(level1Directory))
	common.AssertNoError(err, "error reading level 1 directory")

	for i := 0; i < len(dirEntries); i++ {
		// read in each level 1 summary
		var level1Summary common.Level1Summary

		level1JsonBytes, err := os.ReadFile(filepath.Join(level1Directory, dirEntries[i].Name()))
		common.AssertNoError(err, "error reading level 1 json")

		err = json.Unmarshal(level1JsonBytes, &level1Summary)
		common.AssertNoError(err, "error unmarshalling level 1 test run: "+dirEntries[i].Name())

		level1Summaries = append(level1Summaries, level1Summary)
	}
	return level1Summaries
}

func saveFailureMessage(testResult common.Level1TestResult) {
	saveMessageHelper(testResult, failuresDir, "failure")
}

func saveNoResultMessage(testResult common.Level1TestResult) {
	saveMessageHelper(testResult, noResultsDir, "no-result")
}

// for each failed / no-result test, we want to save the raw output message as a text file
// there could be multiple failures / no-results of the same test, so we want to save each failed / no-result message in a separate text file
// each test with failures / no-results will have a uniquely named (based on test name and package) subdirectory where failed / no-result messages are saved
// e.g. "failures/TestSanitySha3_256+github.com-onflow-flow-go-crypto-hash" will store failed messages text files
// from test TestSanitySha3_256 from the "github.com/onflow/flow-go/crypto/hash" package
// failure and no-result messages are saved in a similar way so this helper function
// handles saving both types of messages
func saveMessageHelper(testResult common.Level1TestResult, messagesDir string, messageFileStem string) {
	// each subdirectory corresponds to a failed / no-result test name and package name
	messagesDirFullPath := messagesDir + testResult.Test + "+" + strings.ReplaceAll(testResult.Package, "/", "-") + "/"

	// there could already be previous failures / no-results for this test, so it's important
	// to check if failed test / no-result folder exists
	if !common.DirExists(messagesDirFullPath) {
		err := os.Mkdir(messagesDirFullPath, 0755)
		common.AssertNoError(err, "error creating sub-dir under failures / no-results dir")
	}

	// under each sub-directory, there should be 1 or more text files
	// (failure1.txt / no-result1.txt, failure2.txt / no-result2.txt, etc)
	// that holds the raw failure / no-result message for that test
	dirEntries, err := os.ReadDir(messagesDirFullPath)
	common.AssertNoError(err, "error reading sub-dir entries under failures / no-results dir")

	// failure text files will be named "failure1.txt", "failure2.txt", etc
	// no-result text files will be named "no-result1.txt", "no-result2.txt", etc
	// need to know how many failure / no-result text files already exist in the sub-directory before creating the next one
	messageFile, err := os.Create(messagesDirFullPath + fmt.Sprintf(messageFileStem+"%d.txt", len(dirEntries)+1))
	common.AssertNoError(err, "error creating failure / no-result file")
	defer messageFile.Close()

	for _, output := range testResult.Output {
		_, err = messageFile.WriteString(output.Item)
		common.AssertNoError(err, "error writing to failure / no-result file")
	}
}

// postProcessLevel2Summary calculates average duration and failure rate for each test over multiple level 1 summaries
func postProcessLevel2Summary(testSummary2 common.Level2Summary) {
	for _, level2TestResult := range testSummary2.TestResultsMap {
		// calculate average duration for each test summary
		var durationSum float32 = 0
		for _, duration := range level2TestResult.Durations {
			durationSum += duration
		}
		level2TestResult.AverageDuration = common.ConvertToNDecimalPlaces2(2, durationSum, level2TestResult.Runs)

		// calculate failure rate for each test summary
		level2TestResult.FailureRate = common.ConvertToNDecimalPlaces(2, level2TestResult.Failed, level2TestResult.Runs)
	}

	// check if there are no failures so can delete failures sub-directory
	if common.IsDirEmpty(failuresDir) {
		err := os.RemoveAll(failuresDir)
		common.AssertNoError(err, "error removing failures directory")
	}

	// check if there are no no-result tests so can delete no-results sub-directory
	if common.IsDirEmpty(noResultsDir) {
		err := os.RemoveAll(noResultsDir)
		common.AssertNoError(err, "error removing no-results directory")
	}
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

	testSummary2 := generateLevel2Summary(os.Args[1])
	common.SaveToFile("level2-summary.json", testSummary2)
}
