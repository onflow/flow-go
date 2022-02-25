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
	"time"
)

// AssertNoError checks that the passed in error is nil and panics
// with the supplied message if that's not the case.
// Useful helper to eliminate the need to keep checking for errors.
func AssertNoError(err error, panicMessage string) {
	if err != nil {
		panic(panicMessage + ": " + err.Error())
	}
}

// ConvertToNDecimalPlaces2 converts the supplied numerator and denominator fraction into
// a decimal with n decimal places. Works the same way as ConvertToNDecimalPlaces()
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

func GetCommitSha() string {
	commitSha := os.Getenv("COMMIT_SHA")
	if commitSha == "" {
		panic("COMMIT_SHA can't be empty")
	}
	return commitSha
}

func GetCommitDate() time.Time {
	commitDate, err := time.Parse(time.RFC3339, os.Getenv("COMMIT_DATE"))
	AssertNoError(err, "error parsing COMMIT_DATE")
	return commitDate.UTC()
}

func GetJobRunDate() time.Time {
	jobStarted, err := time.Parse(time.RFC3339, os.Getenv("JOB_STARTED"))
	AssertNoError(err, "error parsing JOB_STARTED")
	return jobStarted.UTC()
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
