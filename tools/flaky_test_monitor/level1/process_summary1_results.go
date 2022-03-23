package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/onflow/flow-go/tools/flaky_test_monitor/common"
	"github.com/onflow/flow-go/utils/unittest"
)

// ResultReader gives us the flexibility to read test results in multiple ways - from stdin (for production) and from a local file (for unit testing)
type ResultReader interface {
	getReader() *os.File
	close()

	// where to save results - will be different for unit tests vs production
	getResultsFileName() string
}

type StdinResultReader struct {
}

// return reader for reading from stdin - for production
func (stdinResultReader StdinResultReader) getReader() *os.File {
	return os.Stdin
}

// nothing to close when reading from stdin
func (stdinResultReader StdinResultReader) close() {
}

func (stdinResultReader StdinResultReader) getResultsFileName() string {
	return os.Getenv("RESULTS_FILE")
}

func generateLevel1Summary(resultReader ResultReader) (common.Level1Summary, map[string]*common.SkippedTestEntry) {
	reader := resultReader.getReader()
	scanner := bufio.NewScanner(reader)

	defer resultReader.close()

	testResultMap := processTestRunLineByLine(scanner)

	err := scanner.Err()
	common.AssertNoError(err, "error returning EOF for scanner")

	testRun, skippedTests := finalizeLevel1Summary(testResultMap)

	return testRun, skippedTests
}

// Raw JSON result step from `go test -json` execution
// Sequence of result steps (specified by Action value) per test:
// 1. run (once)
// 2. output (one to many)
// 3. pause (zero or once) - for tests using t.Parallel()
// 4. cont (zero or once) - for tests using t.Parallel()
// 5. pass OR fail OR skip (once)
func processTestRunLineByLine(scanner *bufio.Scanner) map[string][]*common.Level1TestResult {
	// test map holds all the tests
	testResultMap := make(map[string][]*common.Level1TestResult)

	for scanner.Scan() {
		var rawTestStep common.RawTestStep
		err := json.Unmarshal(scanner.Bytes(), &rawTestStep)
		common.AssertNoError(err, "error unmarshalling raw test step")

		// each test name needs to be unique, so we add package name in case there are
		// tests with the same name across different packages
		testResultMapKey := rawTestStep.Package + "/" + rawTestStep.Test

		// most raw test steps will have Test value - only package specific steps won't have a Test value
		// we're not storing package specific data
		if rawTestStep.Test != "" {

			// "run" is the very first test step and it needs special treatment - to create all the data structures that will be used by subsequent test steps for the same test
			if rawTestStep.Action == "run" {
				var newTestResult common.Level1TestResult
				newTestResult.Test = rawTestStep.Test
				newTestResult.Package = rawTestStep.Package

				// each test holds specific data, irrespective of what the test result
				newTestResult.CommitDate = common.GetCommitDate()
				newTestResult.JobRunDate = common.GetJobRunDate()
				newTestResult.CommitSha = common.GetCommitSha()
				newTestResult.RunID = common.GetRunID()

				// store outputs as a slice of strings - that's how "go test -json" outputs each output string on a separate line
				// for passing tests, there are usually 2 outputs for a passing test and more outputs for a failing test
				newTestResult.Output = make([]string, 0)

				// append to test result slice, whether it's the first or subsequent test result
				testResultMap[testResultMapKey] = append(testResultMap[testResultMapKey], &newTestResult)
				continue
			}

			lastTestResultIndex := len(testResultMap[testResultMapKey]) - 1
			if lastTestResultIndex < 0 {
				lastTestResultIndex = 0
			}

			testResults, ok := testResultMap[testResultMapKey]
			if !ok {
				panic(fmt.Sprintf("no test result for test %s", rawTestStep.Test))
			}
			lastTestResultPointer := testResults[lastTestResultIndex]

			// subsequent raw json outputs will have different data about the test - whether it passed/failed, what the test output was, etc
			switch rawTestStep.Action {
			case "output":
				// keep appending output to the test
				lastTestResultPointer.Output = append(lastTestResultPointer.Output, rawTestStep.Output)

			// we need to convert pass / fail result into a numerical value so it can averaged and tracked on a graph
			// pass is counted as 1, fail is counted as a 0
			case "pass":
				lastTestResultPointer.Pass = 1
				lastTestResultPointer.Action = "pass"
				lastTestResultPointer.Elapsed = rawTestStep.Elapsed
				lastTestResultPointer.Exception = 0

			case "fail":
				lastTestResultPointer.Pass = 0
				lastTestResultPointer.Action = "fail"
				lastTestResultPointer.Elapsed = rawTestStep.Elapsed
				lastTestResultPointer.Exception = 0

			// skipped tests will be removed after all test results are gathered,
			// since it would be more complicated to remove it here
			case "skip":
				lastTestResultPointer.Action = "skip"
				lastTestResultPointer.Elapsed = rawTestStep.Elapsed

			case "pause", "cont":
				// tests using t.Parallel() will have these values
				// nothing to do - test will continue to run normally and have a pass/fail result at the end

			default:
				panic(fmt.Sprintf("unexpected action: %s", rawTestStep.Action))
			}
		}
	}
	return testResultMap
}

func parseSkipReason(output []string) (unittest.SkipReason, bool) {
	for i := len(output) - 2; i >= 0; i-- {
		skipReason, ok := unittest.ParseSkipReason(output[i])
		if ok {
			return skipReason, true
		}
	}
	return 0, false
}

func finalizeLevel1Summary(testResultMap map[string][]*common.Level1TestResult) (common.Level1Summary, map[string]*common.SkippedTestEntry) {
	var level1Summary common.Level1Summary
	skippedTests := make(map[string]*common.SkippedTestEntry)
	trackSkippedTests := getSkippedTestFile() != ""
	testCategory := getTestCategory()

	for _, testResults := range testResultMap {
		for _, testResult := range testResults {
			if trackSkippedTests {
				skippedTestEntry := &common.SkippedTestEntry{
					Test:       testResult.Test,
					Package:    testResult.Package,
					CommitDate: testResult.CommitDate,
					CommitSHA:  testResult.CommitSha,
					Category:   testCategory,
				}
				skippedTests[testResult.Test] = skippedTestEntry

				if testResult.Action == "skip" {
					skipReason, ok := parseSkipReason(testResult.Output)
					if ok {
						skippedTestEntry.SkipReason = skipReason
					} else {
						panic("could not parse Skip Reason from test output")
					}
				}
			}

			if testResult.Action == "skip" {
				// don't add skipped tests to summary since they can't be used to compute an average pass rate
				continue
			}

			// for tests that don't have a result generated (e.g. using fmt.Printf() with no newline in a test)
			// we want to highlight these tests in Grafana
			// we do this by setting the Exception field to true, so we can filter by that field in Grafana
			if testResult.Action == "" {
				// count exception as a failure
				testResult.Pass = 0
				testResult.Exception = 1
			}

			testResult.Action = ""

			// only save output for failed tests
			if testResult.Pass == 1 {
				testResult.Output = nil
			}

			// only include passed or failed tests - don't include skipped tests
			// this is needed to have accurate Grafana metrics for average pass rate
			level1Summary = append(level1Summary, *testResult)
		}
	}

	return level1Summary, skippedTests
}

func getSkippedTestFile() string {
	return os.Getenv("SKIPPED_TESTS_FILE")
}

func getTestCategory() string {
	return os.Getenv("TEST_CATEGORY")
}

// level 1 flaky test summary processor
// input: json formatted test results from `go test -json` from console (not file)
// output: level 1 summary json file that will be used as input for level 2 summary processor
func main() {
	resultReader := StdinResultReader{}

	testRun, skippedTestMap := generateLevel1Summary(resultReader)

	resultsFile := resultReader.getResultsFileName()
	if resultsFile != "" {
		common.SaveLinesToFile(resultsFile, testRun)
	}

	skippedTestsFile := getSkippedTestFile()
	if skippedTestsFile != "" {
		var skippedTests []*common.SkippedTestEntry
		for _, skippedTestEntry := range skippedTestMap {
			skippedTests = append(skippedTests, skippedTestEntry)
		}
		common.SaveLinesToFile(skippedTestsFile, skippedTests)
	}
}
