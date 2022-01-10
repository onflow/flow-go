// Package common has helper / utility functions used by all levels of flaky test monitor.
package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"
)

// AssertNoError checks that the passed in error is nil and panics
// with the supplied message if that's not the case.
// Useful helper to eliminate the need to keep checking for errors.
func AssertNoError(err error, panicMessage string) {
	if err != nil {
		panic(panicMessage + ": " + err.Error())
	}
}

// AddTrailingSlash adds a trailing slash to the supplied string, if required.
// If path doesn't have trailing slash, appends one.
// If path has trailing slash, doesn't change it.
func AddTrailingSlash(path string) string {
	if strings.HasSuffix(path, "/") {
		return path
	}
	return path + "/"
}

// ConvertToNDecimalPlaces2 converts the supplied numerator and denominator fraction into
// a decimal with n decimla places. Works the same way as ConvertToNDecimalPlaces()
// but has a float for the numerator.
func ConvertToNDecimalPlaces2(n int, numerator float32, denominator int) float32 {
	return convertToNDecimalPlacesInternal(n, numerator, float32(denominator))
}

// ConvertToNDecimalPlaces converts the supplied numerator and denominator fraction into
// a decimal with n decimal places.
func ConvertToNDecimalPlaces(n int, numerator, denominator int) float32 {
	return convertToNDecimalPlacesInternal(n, float32(numerator), float32(denominator))
}

func convertToNDecimalPlacesInternal(n int, numerator, denominator float32) float32 {
	if numerator == 0 || denominator == 0 {
		return 0
	}
	formatSpecifier := "%." + fmt.Sprint(n) + "f"
	ratioString := fmt.Sprintf(formatSpecifier, numerator/denominator)
	ratioFloat, err := strconv.ParseFloat(ratioString, 32)
	AssertNoError(err, "failure parsing string to float")
	return float32(ratioFloat)
}

// IsDirEmpty checks if directory is empty (has no files) and return true if it's empty, false otherwise.
// Useful for determining whether to delete the failures / no-results directories
// for cases when there were no failures / no-results.
// From https://stackoverflow.com/a/30708914/5719544.
func IsDirEmpty(name string) bool {
	f, err := os.Open(name)
	AssertNoError(err, "error reading directory")

	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true
	}
	AssertNoError(err, "error reading dir contents")
	return false
}

// DirExists checks if directory exists and return true if it does, false otherwise.
func DirExists(path string) bool {
	_, err := os.Stat(path)

	// directory exists if there is no error
	if err == nil {
		return true
	}

	// directory doesn't exist if error is of specific type
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}

	// should never get to here
	panic("error checking if directory exists:" + err.Error())
}

// SaveToFile save test run/summary to local JSON file.
func SaveToFile(fileName string, testSummary interface{}) {
	testSummaryBytes, err := json.MarshalIndent(testSummary, "", "  ")
	AssertNoError(err, "error marshalling json")

	file, err := os.Create(fileName)
	AssertNoError(err, "error creating filename")
	defer file.Close()

	_, err = file.Write(testSummaryBytes)
	AssertNoError(err, "error saving test summary to file")
}

func ProcessRawTestStep(rawTestStep *RawTestStep) {
	// check if package result exists to hold test results
	packageResult, packageResultExists := packageResultMap[rawTestStep.Package]
	if !packageResultExists {
		packageResult = &PackageResult{
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
