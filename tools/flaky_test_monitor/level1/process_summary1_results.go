package main

import (
	"bufio"
	"encoding/json"
	"flaky-test-monitor/helpers"
	"fmt"
	"os"
	"sort"
	"time"
)

// this interface gives us the flexibility to read test results in multiple ways - from stdin (for production) and from a local file (for testing)
type ResultReader interface {
	getReader() *os.File
	close()

	// where to save results - will be different for tests vs production
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

func processSummary1TestRun(resultReader ResultReader) helpers.TestRun {
	reader := resultReader.getReader()
	scanner := bufio.NewScanner(reader)

	defer resultReader.close()

	packageResultMap := processTestRunLineByLine(scanner)

	err := scanner.Err()
	helpers.AssertErrNil(err, "error returning EOF for scanner")

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
func processTestRunLineByLine(scanner *bufio.Scanner) map[string]*helpers.PackageResult {
	packageResultMap := make(map[string]*helpers.PackageResult)
	// reuse the same package result over and over
	for scanner.Scan() {
		var rawTestStep helpers.RawTestStep
		err := json.Unmarshal(scanner.Bytes(), &rawTestStep)
		helpers.AssertErrNil(err, "error unmarshalling raw test step")

		// check if package result exists to hold test results
		packageResult, packageResultExists := packageResultMap[rawTestStep.Package]
		if !packageResultExists {
			packageResult = &helpers.PackageResult{
				Package: rawTestStep.Package,

				// package result will hold map of test results
				TestMap: make(map[string][]helpers.TestResult),

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
				var newTestResult helpers.TestResult
				newTestResult.Test = rawTestStep.Test
				newTestResult.Package = rawTestStep.Package

				// store outputs as a slice of strings - that's how "go test -json" outputs each output string on a separate line
				// for passing tests, there are usually 2 outputs for a passing test and more outputs for a failing test
				newTestResult.Output = make([]string, 0)

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
				lastTestResultPointer.Output = append(lastTestResultPointer.Output, rawTestStep.Output)

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

func postProcessTestRun(packageResultMap map[string]*helpers.PackageResult) {
	// transfer each test result map in each package result to a test result slice
	for packageName, packageResult := range packageResultMap {

		// delete skipped packages since they don't have any tests - won't be adding it to result map
		if packageResult.Result == "skip" {
			delete(packageResultMap, packageName)
			continue
		}

		for _, testResults := range packageResult.TestMap {
			packageResult.Tests = append(packageResult.Tests, testResults...)
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

func finalizeTestRun(packageResultMap map[string]*helpers.PackageResult) helpers.TestRun {
	commitSha := os.Getenv("COMMIT_SHA")
	if commitSha == "" {
		panic("COMMIT_SHA can't be empty")
	}

	commitDate, err := time.Parse(time.RFC3339, os.Getenv("COMMIT_DATE"))
	helpers.AssertErrNil(err, "error parsing COMMIT_DATE")

	jobStarted, err := time.Parse(time.RFC3339, os.Getenv("JOB_STARTED"))
	helpers.AssertErrNil(err, "error parsing JOB_STARTED")

	var testRun helpers.TestRun
	testRun.CommitDate = commitDate.UTC()
	testRun.CommitSha = commitSha
	testRun.JobRunDate = jobStarted.UTC()

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

func main() {
	resultReader := StdinResultReader{}

	testRun := processSummary1TestRun(resultReader)

	testRun.Save(resultReader.getResultsFileName())
}
