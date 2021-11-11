package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// data driven table test
func TestProcessTestRun(t *testing.T) {
	testDataMap := map[string]string{
		"1 count all pass":                "test-result-crypto-hash-1-count-pass.json",
		"1 count 1 fail the rest pass":    "test-result-crypto-hash-1-count-fail.json",
		"1 count 2 skipped the rest pass": "test-result-crypto-hash-1-count-skip-pass.json",

		// raw results generated with: go test -json -count 1 --tags relic ./utils/unittest/...
		"1 count skip all packages": "test-result-crypto-hash-1-count-skip-all-packages.json",
		"2 count all pass":          "test-result-crypto-hash-2-count-pass.json",
		"10 count all pass":         "test-result-crypto-hash-10-count-pass.json",
		"10 count some failures":    "test-result-crypto-hash-10-count-fail.json",

		// raw results generated with: go test -v -tags relic -count=1 -json ./model/encodable/. -test.run TestEncodableRandomBeaconPrivKeyMsgPack
		// this is a single unit test that produces a nil test result
		// "1 count single nil test": "test-result-nil-test-single-1-count-pass.json",

		//raw results generated with: go test -v -tags relic -count=5 -json ./model/encodable/. -test.run TestEncodableRandomBeaconPrivKeyMsgPack
		//for testing that re-slicing logic works when there are multiple nil tests in a row
		// "5 nil tests in a row": "test-result-nil-test-single-5-count-pass.json",

		//for testing that re-slicing logic works when there is a normal test at the of a test run with multiple nil tests in front of it
		// "4 nil tests in a row, 1 normal test": "test-result-nil-test-single-5-count-4-nil-1-normal-pass.json",

		// raw results generated with: go test -v -tags relic -count=3 -json ./model/encodable/.
		// this is a group of unit tests with a single nil test result
		// "3 count nil test with normal tests": "test-result-nil-test-others-normal-3-count-pass.json",
	}

	for k, testJsonData := range testDataMap {
		t.Run(k, func(t *testing.T) {
			runProcessTestRun(t, testJsonData)
		})
	}
}

// HELPERS - UTILITIES

func runProcessTestRun(t *testing.T, jsonExpectedActualFile string) {
	const expectedJsonFilePath = "./testdata/expected/"
	const rawJsonFilePath = "./testdata/raw/"

	var expectedTestRun TestRun
	// read in expected JSON from file
	expectedJsonBytes, err := ioutil.ReadFile(expectedJsonFilePath + jsonExpectedActualFile)
	require.Nil(t, err)
	require.NotEmpty(t, expectedJsonBytes)

	err = json.Unmarshal(expectedJsonBytes, &expectedTestRun)
	require.Nil(t, err)

	// convert to UTC to remove any local time zone settings -
	// even though the time stamp in the test json can be in UTC (or not), there will still be a local time zone set that will fail equality check - this removes the timezone setting
	expectedTestRun.CommitDate = expectedTestRun.CommitDate.UTC()
	expectedTestRun.JobRunDate = expectedTestRun.JobRunDate.UTC()

	// sort all package results alphabetically
	sort.SliceStable(expectedTestRun.PackageResults, func(i, j int) bool {
		return expectedTestRun.PackageResults[i].Package < expectedTestRun.PackageResults[j].Package
	})

	// sort all tests alphabetically within each package - otherwise, equality check will fail
	for k := range expectedTestRun.PackageResults {
		sort.Slice(expectedTestRun.PackageResults[k].Tests, func(i, j int) bool {
			return expectedTestRun.PackageResults[k].Tests[i].Test < expectedTestRun.PackageResults[k].Tests[j].Test
		})

		// init TestMap to empty - otherwise get comparison failure because would be nil
		expectedTestRun.PackageResults[k].TestMap = make(map[string][]TestResult)
	}

	// these hard coded values simulate a real test run that would obtain these environment variables dynamically
	// we are simulating this scenario by setting the environment variables explicitly in the test before calling the main processing script which will look for them
	// these values are uses in testdata/expected/*.json files
	require.NoError(t, os.Setenv("COMMIT_DATE", "2021-09-21T18:06:25-07:00"))
	require.NoError(t, os.Setenv("COMMIT_SHA", "46baf6c6be29af9c040bc14195e195848598bbae"))
	require.NoError(t, os.Setenv("JOB_STARTED", "2021-09-21T21:06:25-07:00"))

	// simulate generating raw "go test -json" output by loading output from saved file
	resultReader := FileResultReader{
		rawJsonFile: rawJsonFilePath + jsonExpectedActualFile,
	}
	actualTestRun := processTestRun(&resultReader)

	checkTestRuns(t, expectedTestRun, actualTestRun)
}

func checkTestRuns(t *testing.T, expectedTestRun TestRun, actualTestRun TestRun) {
	// it's difficult to determine why 2 test runs aren't equal, so we will check the different sub components of them to see where a potential discrepancy exists
	require.Equal(t, expectedTestRun.CommitDate, actualTestRun.CommitDate)
	require.Equal(t, expectedTestRun.CommitSha, actualTestRun.CommitSha)
	require.Equal(t, expectedTestRun.JobRunDate, actualTestRun.JobRunDate)
	require.Equal(t, len(expectedTestRun.PackageResults), len(actualTestRun.PackageResults))

	// check each package
	for packageIndex := range expectedTestRun.PackageResults {
		expectedPackageResults := expectedTestRun.PackageResults[packageIndex]
		actualPackageResults := actualTestRun.PackageResults[packageIndex]

		require.Equal(t, expectedPackageResults.Elapsed, actualPackageResults.Elapsed)
		require.Equal(t, expectedPackageResults.Package, actualPackageResults.Package)
		require.Equal(t, expectedPackageResults.Result, actualPackageResults.Result)
		require.Empty(t, expectedPackageResults.TestMap, actualPackageResults.TestMap)

		// check outputs of each package result
		require.Equal(t, len(expectedPackageResults.Output), len(actualPackageResults.Output))
		for packageOutputIndex := range expectedPackageResults.Output {
			require.Equal(t, expectedPackageResults.Output[packageOutputIndex], actualPackageResults.Output[packageOutputIndex])
		}

		// check all regular and nil tests results of each package
		checkTestResults(t, expectedPackageResults.Tests, actualPackageResults.Tests)
		checkTestResults(t, expectedPackageResults.NilTests, actualPackageResults.NilTests)
	}
	// finally, compare the entire actual test run against what's expected - if there were any discrepancies they should have been caught by now
	require.Equal(t, expectedTestRun, actualTestRun)
}

// checks regular and nil test results
func checkTestResults(t *testing.T, expectedTestResults []TestResult, actualTestResults []TestResult) {
	require.Equal(t, len(expectedTestResults), len(actualTestResults))
	for testResultIndex := range expectedTestResults {

		expectedTestResult := expectedTestResults[testResultIndex]
		actualTestResult := actualTestResults[testResultIndex]

		// check all outputs of each test result
		require.Equal(t, len(expectedTestResult.Output), len(actualTestResult.Output), fmt.Sprintf("TestResult[%d].Test: %s", testResultIndex, actualTestResult.Test))
		for testResultOutputIndex := range expectedTestResult.Output {
			require.Equal(t, expectedTestResult.Output[testResultOutputIndex], actualTestResult.Output[testResultOutputIndex], fmt.Sprintf("TestResult[%d] Output[%d]", testResultIndex, testResultOutputIndex))
		}

		require.Equal(t, expectedTestResult.Package, actualTestResult.Package)
		require.Equal(t, expectedTestResult.Test, actualTestResult.Test)
		require.Equal(t, expectedTestResult.Elapsed, actualTestResult.Elapsed, fmt.Sprintf("TestResult[%d].Test: %s", testResultIndex, actualTestResult.Test))
		require.Equal(t, expectedTestResult.Result, actualTestResult.Result)
	}
}

// read raw results from local json file - for testing
type FileResultReader struct {
	rawJsonFile string
	file        *os.File
}

// return reader for reading from local json file - for testing
func (fileResultReader *FileResultReader) getReader() *os.File {
	f, err := os.Open(fileResultReader.rawJsonFile)
	if err != nil {
		log.Fatal("error opening file: " + err.Error())
	}
	fileResultReader.file = f
	return f
}

func (fileResultReader *FileResultReader) close() {
	err := fileResultReader.file.Close()
	if err != nil {
		log.Fatal("error closing file: " + err.Error())
	}
}

// tests will create their own local result file based on time stamp vs production which uses a supplied result file name
func (fileResultReader FileResultReader) getResultsFileName() string {
	t := time.Now()
	return "test-run-" + strings.ReplaceAll(t.Format("2006-01-02-15-04-05.0000"), ".", "-") + ".json"
}
