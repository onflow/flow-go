package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"

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

	packageResultMap := processTestRunLineByLine(scanner)

	err := scanner.Err()
	common.AssertNoError(err, "error returning EOF for scanner")

	postProcessTestRun(packageResultMap)

	testRun := finalizeTestRun(packageResultMap)

	return testRun
}

// Raw JSON result step from `go test -json` execution
// Sequence of result steps (specified by Action value) per test:
// 1. run (once)
// 2. output (one to many)
// 3. pause (zero or once) - for tests using t.Parallel()
// 4. cont (zero or once) - for tests using t.Parallel()
// 5. pass OR fail OR skip (once)
func processTestRunLineByLine(scanner *bufio.Scanner) map[string]*common.PackageResult {
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
				//newTestResult.Output = make([]string, 0)
				newTestResult.Output = make([]struct {
					Item string "json:\"item\""
				}, 0)

				// append to test result slice, whether it's the first or subsequent test result
				packageResult.TestMap[rawTestStep.Test] = append(packageResult.TestMap[rawTestStep.Test], newTestResult)
				continue
			}

			lastTestResultIndex := len(packageResult.TestMap[rawTestStep.Test]) - 1
			if lastTestResultIndex < 0 {
				lastTestResultIndex = 0
			}

			testResults, ok := packageResult.TestMap[rawTestStep.Test]
			if !ok {
				panic(fmt.Sprintf("no test result for test %s", rawTestStep.Test))
			}
			lastTestResultPointer := &testResults[lastTestResultIndex]

			// subsequent raw json outputs will have different data about the test - whether it passed/failed, what the test output was, etc
			switch rawTestStep.Action {
			case "output":
				output := struct {
					Item string `json:"item"`
				}{rawTestStep.Output}
				lastTestResultPointer.Output = append(lastTestResultPointer.Output, output)

			case "pass", "fail", "skip":
				lastTestResultPointer.Result = rawTestStep.Action
				lastTestResultPointer.Elapsed = rawTestStep.Elapsed

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
	return packageResultMap
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
				if testResult.Result != "pass" && testResult.Result != "fail" {
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

		// clear test result map once all values transfered to slice - needed for testing so will check against an empty map
		for k := range packageResultMap[packageName].TestMap {
			delete(packageResultMap[packageName].TestMap, k)
		}
	}

	// sort all the test results in each package result slice - needed for testing so it's easy to compare ordered tests
	for _, pr := range packageResultMap {
		sort.SliceStable(pr.Tests, func(i, j int) bool {
			return pr.Tests[i].Test < pr.Tests[j].Test
		})
	}
}

func finalizeTestRun(packageResultMap map[string]*common.PackageResult) common.TestRun {
	var testRun common.TestRun

	testRun.CommitDate = common.GetCommitDate()
	testRun.CommitSha = common.GetCommitSha()
	testRun.JobRunDate = common.GetJobRunDate()

	// add all the package results to the test run
	for _, pr := range packageResultMap {
		testRun.PackageResults = append(testRun.PackageResults, *pr)
	}

	// sort all package results in the test run
	sort.SliceStable(testRun.PackageResults, func(i, j int) bool {
		return testRun.PackageResults[i].Package < testRun.PackageResults[j].Package
	})

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
