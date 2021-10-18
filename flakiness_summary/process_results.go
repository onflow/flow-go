package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

// models single line from "go test -json" output
type RawTestStep struct {
	Time    time.Time `json:"Time"`
	Action  string    `json:"Action"`
	Package string    `json:"Package"`
	Test    string    `json:"Test"`
	Output  string    `json:"Output"`
	Elapsed float64   `json:"Elapsed"`
}

// models full summary of a test run from "go test -json"
type TestRun struct {
	CommitSha      string          `json:"commit_sha"`
	CommitDate     time.Time       `json:"commit_date"`
	JobRunDate     time.Time       `json:"job_run_date"`
	PackageResults []PackageResult `json:"results"`
}

// save TestRun to local JSON file
func (testRun *TestRun) save() {
	testRunBytes, err := json.MarshalIndent(testRun, "", "  ")

	if err != nil {
		panic("error marshalling json" + err.Error())
	}

	fileName := os.Args[1]

	file, err := os.Create(fileName)
	if err != nil {
		panic("error creating filename: " + err.Error())
	}
	defer file.Close()

	_, err = file.Write(testRunBytes)
	if err != nil {
		panic("error saving test run to file: " + err.Error())
	}
}

// models test result of an entire package which can have multiple tests
type PackageResult struct {
	Package string       `json:"package"`
	Result  string       `json:"result"`
	Elapsed float64      `json:"elapsed"`
	Output  []string     `json:"output"`
	Tests   []TestResult `json:"tests"`
	TestMap map[string][]TestResult
}

// models result of a single test that's part of a larger package result
type TestResult struct {
	Test    string   `json:"test"`
	Package string   `json:"package"`
	Output  []string `json:"output"`
	Result  string   `json:"result"`
	Elapsed float64  `json:"elapsed"`
}

// this interface gives us the flexibility to read test results in multiple ways - from stdin (for production) and from a local file (for testing)
type ResultReader interface {
	getReader() *os.File
	close()
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

func processTestRun(resultReader ResultReader) TestRun {
	reader := resultReader.getReader()
	scanner := bufio.NewScanner(reader)

	defer resultReader.close()

	packageResultMap := processTestRunLineByLine(scanner)

	err := scanner.Err()
	if err != nil {
		panic("error returning EOF for scanner: " + err.Error())
	}

	postProcessTestRun(packageResultMap)

	testRun := finalizeTestRun(packageResultMap)
	testRun.save()

	return testRun
}

func processTestRunLineByLine(scanner *bufio.Scanner) map[string]*PackageResult {
	packageResultMap := make(map[string]*PackageResult)
	// reuse the same package result over and over
	for scanner.Scan() {
		var rawTestStep RawTestStep
		err := json.Unmarshal(scanner.Bytes(), &rawTestStep)
		if err != nil {
			panic("error unmarshalling raw test step: " + err.Error())
		}

		// check if package result exists to hold test results
		packageResult, packageResultExists := packageResultMap[rawTestStep.Package]
		if !packageResultExists {
			packageResult = &PackageResult{
				Package: rawTestStep.Package,

				// package result will hold map of test results
				TestMap: make(map[string][]TestResult),

				// store outputs as a slice of strings - that's how "go test -json" outputs each output string on a separate line
				// there are usually 2 or more outputs for a package
				Output: make([]string, 0),
			}
			packageResultMap[rawTestStep.Package] = packageResult
		}

		// most raw test steps will have Test value - only package specific steps won't
		if rawTestStep.Test != "" {

			lastTestResultIndex := len(packageResult.TestMap[rawTestStep.Test]) - 1
			if lastTestResultIndex < 0 {
				lastTestResultIndex = 0
			}

			// subsequent raw json outputs will have different data about the test - whether it passed/failed, what the test output was, etc
			switch rawTestStep.Action {

			// Raw JSON result step from `go test -json` execution
			// Sequence of result steps (specified by Action value) per test:
			// 1. run (once)
			// 2. output (one to many)
			// 3. pause (zero or once) - for tests using t.Parallel()
			// 4. cont (zero or once) - for tests using t.Parallel()
			// 5. pass OR fail OR skip (once)

			case "run":
				var newTestResult TestResult
				newTestResult.Test = rawTestStep.Test

				// store outputs as a slice of strings - that's how "go test -json" outputs each output string on a separate line
				// for passing tests, there are usually 2 outputs for a passing test and more outputs for a failing test
				newTestResult.Output = make([]string, 0)

				// append to test result slice, whether it's the first or subsequent test result
				packageResult.TestMap[rawTestStep.Test] = append(packageResult.TestMap[rawTestStep.Test], newTestResult)
				lastTestResultIndex = len(packageResult.TestMap[rawTestStep.Test]) - 1
				packageResult.TestMap[rawTestStep.Test][lastTestResultIndex].Package = rawTestStep.Package

			case "output":
				testResults, ok := packageResult.TestMap[rawTestStep.Test]
				if !ok {
					panic(fmt.Sprintf("no test result for test %s", rawTestStep.Test))
				}
				packageResult.TestMap[rawTestStep.Test][lastTestResultIndex].Output = append(testResults[lastTestResultIndex].Output, rawTestStep.Output)

			case "pass", "fail", "skip":
				packageResult.TestMap[rawTestStep.Test][lastTestResultIndex].Result = rawTestStep.Action
				packageResult.TestMap[rawTestStep.Test][lastTestResultIndex].Elapsed = rawTestStep.Elapsed

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

func postProcessTestRun(packageResultMap map[string]*PackageResult) {
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

func finalizeTestRun(packageResultMap map[string]*PackageResult) TestRun {
	commitSha := os.Getenv("COMMIT_SHA")
	if commitSha == "" {
		panic("COMMIT_SHA can't be empty")
	}

	commitDate, err := time.Parse(time.RFC3339, os.Getenv("COMMIT_DATE"))
	if err != nil {
		panic("error parsing COMMIT_DATE: " + err.Error())
	}

	jobStarted, err := time.Parse(time.RFC3339, os.Getenv("JOB_STARTED"))
	if err != nil {
		panic("error parsing JOB_STARTED: " + err.Error())
	}

	var testRun TestRun
	testRun.CommitDate = commitDate
	testRun.CommitSha = commitSha
	testRun.JobRunDate = jobStarted

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
	processTestRun(StdinResultReader{})
}
