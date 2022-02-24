package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/onflow/flow-go/tools/flaky_test_monitor/common"
)

// this interface gives us the flexibility to read test results in multiple ways - from stdin (for production) and from a local file (for unit testing)
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
	return os.Args[1]
}

func processSummary1TestRun(resultReader ResultReader) common.TestRun {
	reader := resultReader.getReader()
	scanner := bufio.NewScanner(reader)

	defer resultReader.close()

	packageResultMap, testResultMap := processTestRunLineByLine(scanner)

	err := scanner.Err()
	common.AssertNoError(err, "error returning EOF for scanner")

	postProcessTestRun(packageResultMap)

	testRun := finalizeTestRun(testResultMap)

	return testRun
}

// Raw JSON result step from `go test -json` execution
// Sequence of result steps (specified by Action value) per test:
// 1. run (once)
// 2. output (one to many)
// 3. pause (zero or once) - for tests using t.Parallel()
// 4. cont (zero or once) - for tests using t.Parallel()
// 5. pass OR fail OR skip (once)
func processTestRunLineByLine(scanner *bufio.Scanner) (map[string]*common.PackageResult, map[string][]*common.TestResult) {
	// test map holds all the tests
	testResultMap := make(map[string][]*common.TestResult)

	// testRun := &common.TestRun{}

	packageResultMap := make(map[string]*common.PackageResult)
	// reuse the same package result over and over
	for scanner.Scan() {
		var rawTestStep common.RawTestStep
		err := json.Unmarshal(scanner.Bytes(), &rawTestStep)
		common.AssertNoError(err, "error unmarshalling raw test step")

		// check if package result exists to hold test results
		packageResult, packageResultExists := packageResultMap[rawTestStep.Package]
		if !packageResultExists {
			packageResult = &common.PackageResult{
				Package: rawTestStep.Package,

				// package result will hold map of test results
				TestMap: make(map[string][]common.TestResult),

				// store outputs as a slice of strings - that's how "go test -json" outputs each output string on a separate line
				// there are usually 2 or more outputs for a package
				Output: make([]string, 0),
			}
			packageResultMap[rawTestStep.Package] = packageResult
		}

		// each test name needs to be unique so we add package name in case there are
		// tests with the same name across different packages
		testResultMapKey := rawTestStep.Package + "/" + rawTestStep.Test

		// most raw test steps will have Test value - only package specific steps won't
		if rawTestStep.Test != "" {

			// "run" is the very first test step and it needs special treatment - to create all the data structures that will be used by subsequent test steps for the same test
			if rawTestStep.Action == "run" {
				var newTestResult common.TestResult
				newTestResult.Test = rawTestStep.Test
				newTestResult.Package = rawTestStep.Package

				// each test holds specific data, irrespective of what the test result
				newTestResult.CommitDate = common.GetCommitDate()
				newTestResult.JobRunDate = common.GetJobRunDate()
				newTestResult.CommitSha = common.GetCommitSha()

				// store outputs as a slice of strings - that's how "go test -json" outputs each output string on a separate line
				// for passing tests, there are usually 2 outputs for a passing test and more outputs for a failing test
				newTestResult.Output = make([]struct {
					Item string "json:\"item\""
				}, 0)

				// append to test result slice, whether it's the first or subsequent test result
				packageResult.TestMap[rawTestStep.Test] = append(packageResult.TestMap[rawTestStep.Test], newTestResult)

				testResultMap[testResultMapKey] = append(testResultMap[testResultMapKey], &newTestResult)
				continue
			}

			lastTestResultIndex := len(packageResult.TestMap[rawTestStep.Test]) - 1
			if lastTestResultIndex < 0 {
				lastTestResultIndex = 0
			}

			lastTestResultIndex2 := len(testResultMap[testResultMapKey]) - 1
			if lastTestResultIndex2 < 0 {
				lastTestResultIndex2 = 0
			}

			testResults, ok := packageResult.TestMap[rawTestStep.Test]
			if !ok {
				panic(fmt.Sprintf("no test result for test %s", rawTestStep.Test))
			}
			lastTestResultPointer := &testResults[lastTestResultIndex]

			testResults2, ok := testResultMap[testResultMapKey]
			if !ok {
				panic(fmt.Sprintf("no test result for test %s", rawTestStep.Test))
			}
			lastTestResultPointer2 := testResults2[lastTestResultIndex2]

			// subsequent raw json outputs will have different data about the test - whether it passed/failed, what the test output was, etc
			switch rawTestStep.Action {
			case "output":
				output := struct {
					Item string `json:"item"`
				}{rawTestStep.Output}
				lastTestResultPointer.Output = append(lastTestResultPointer.Output, output)

				// keep appending output to the test
				lastTestResultPointer2.Output = append(lastTestResultPointer2.Output, output)

			// we need to convert pass / fail result into a numerical value so it can averaged and tracked on a graph
			// pass is counted as 1, fail is counted as a 0
			case "pass":
				lastTestResultPointer.Result = "1"
				lastTestResultPointer.Elapsed = rawTestStep.Elapsed

				lastTestResultPointer2.Result = "1"
				lastTestResultPointer2.Elapsed = rawTestStep.Elapsed

			case "fail":
				lastTestResultPointer.Result = "0"
				lastTestResultPointer.Elapsed = rawTestStep.Elapsed

				lastTestResultPointer2.Result = "0"
				lastTestResultPointer2.Elapsed = rawTestStep.Elapsed

			// skipped tests will be removed after all test results are gathered,
			// since it would be more complicated to remove it here
			case "skip":
				lastTestResultPointer.Result = rawTestStep.Action
				lastTestResultPointer.Elapsed = rawTestStep.Elapsed

				lastTestResultPointer2.Result = rawTestStep.Action
				lastTestResultPointer2.Elapsed = rawTestStep.Elapsed

			case "pause", "cont":
				// tests using t.Parallel() will have these values
				// nothing to do - test will continue to run normally and have a pass/fail result at the end

			default:
				panic(fmt.Sprintf("unexpected action: %s", rawTestStep.Action))
			}
		} else {
			// package level raw messages won't have a Test value
			switch rawTestStep.Action {
			case "output":
				packageResult.Output = append(packageResult.Output, rawTestStep.Output)
			case "pass", "fail", "skip":
				packageResult.Result = rawTestStep.Action
				packageResult.Elapsed = rawTestStep.Elapsed
			default:
				panic(fmt.Sprintf("unexpected action (package): %s", rawTestStep.Action))
			}
		}
	}
	return packageResultMap, testResultMap
}

func postProcessTestRun(packageResultMap map[string]*common.PackageResult) {

	// transfer each test result map in each package result to a test result slice
	for packageName, packageResult := range packageResultMap {

		// delete skipped packages since they don't have any tests - won't be adding it to result map
		if packageResult.Result == "skip" {
			delete(packageResultMap, packageName)
			continue
		}

		for _, testResults := range packageResult.TestMap {
			for _, testResult := range testResults {
				if testResult.Result != "1" && testResult.Result != "0" {
					// for tests that don't have a result generated (e.g. using fmt.Printf() with no newline in a test)
					// we want to highlight these tests so they show up at the top in Granfa
					// we do this by simulating a really low fail rate so that the average success rate
					// will be very low compared to other tests so it will show up on the "flakiest test" panel
					if testResult.Result == "" {
						testResult.Result = "-100"
					} else {
						// only include passed or failed tests - don't include skipped tests
						// this is needed to have accurate Grafana metrics for average pass rate
						continue
					}
				}
				packageResult.Tests = append(packageResult.Tests, testResult)
			}
		}
	}
}

func finalizeTestRun(testResultMap map[string][]*common.TestResult) common.TestRun {
	var testRun common.TestRun

	for _, testResults := range testResultMap {
		for _, testResult := range testResults {
			// don't add skipped tests since they can't be used to compute an average pass rate
			if testResult.Result == "skip" {
				continue
			}

			// for tests that don't have a result generated (e.g. using fmt.Printf() with no newline in a test)
			// we want to highlight these tests so they show up at the top in Granfa
			// we do this by simulating a really low fail rate so that the average success rate
			// will be very low compared to other tests so it will show up on the "flakiest test" panel
			if testResult.Result == "" {
				testResult.Result = "-100"
			}

			// only include passed or failed tests - don't include skipped tests
			// this is needed to have accurate Grafana metrics for average pass rate
			testRun.Rows = append(testRun.Rows, common.TestResultRow{TestResult: *testResult})
		}
	}

	return testRun
}

// level 1 flaky test summary processor
// input: json formatted test results from `go test -json` from console (not file)
// output: level 1 summary json file that will be used as input for level 2 summary processor
func main() {
	resultReader := StdinResultReader{}

	testRun := processSummary1TestRun(resultReader)

	common.SaveToFile(resultReader.getResultsFileName(), testRun)
}
