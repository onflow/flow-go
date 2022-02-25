package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/tools/flaky_test_monitor/common"
)

func TestProcessSummary1TestRun_Struct(t *testing.T) {
	const rawJsonFilePath = "../testdata/summary1/raw"

	// data driven table test
	testDataMap := map[string]TestData{
		"1 count single nil test": {
			ExpectedTestRun:    GetTestData_Level1_1CountSingleNilTest(),
			RawJSONTestRunFile: "test-result-nil-test-single-1-count-pass.json",
		},

		"1 count all pass": {
			ExpectedTestRun:    GetTestData_Level1_1CountPass(),
			RawJSONTestRunFile: "test-result-crypto-hash-1-count-pass.json",
		},
	}

	require.NoError(t, os.Setenv("COMMIT_DATE", "2021-09-21T18:06:25-07:00"))
	require.NoError(t, os.Setenv("COMMIT_SHA", "46baf6c6be29af9c040bc14195e195848598bbae"))
	require.NoError(t, os.Setenv("JOB_STARTED", "2021-09-21T21:06:25-07:00"))

	for k, testData := range testDataMap {
		t.Run(k, func(t *testing.T) {
			// simulate generating raw "go test -json" output by loading output from saved file
			resultReader := FileResultReader{
				rawJsonFile: filepath.Join(rawJsonFilePath, testData.RawJSONTestRunFile),
			}
			actualTestRun := processSummary1TestRun(&resultReader)
			checkTestRuns(t, testData.ExpectedTestRun, actualTestRun)
		})
	}
}

func TestProcessSummary1TestRun_JSON(t *testing.T) {
	testDataMap := map[string]string{
		"1 count all pass":                "test-result-crypto-hash-1-count-pass.json",
		"1 count 1 fail the rest pass":    "test-result-crypto-hash-1-count-fail.json",
		"1 count 2 skipped the rest pass": "test-result-crypto-hash-1-count-skip-pass.json",

		// raw results generated with: go test -json -count 1 --tags relic ./utils/unittest/...
		"2 count all pass":       "test-result-crypto-hash-2-count-pass.json",
		"10 count all pass":      "test-result-crypto-hash-10-count-pass.json",
		"10 count some failures": "test-result-crypto-hash-10-count-fail.json",

		// nil tests - tests below don't generate pass/fail result due to `go test` bug
		// with using `fmt.printf("log message")` without newline `\n`

		// raw results generated with: go test -v -tags relic -count=1 -json ./model/encodable/. -test.run TestEncodableRandomBeaconPrivKeyMsgPack
		// this is a single unit test that produces a nil test result
		"1 count single nil test": "test-result-nil-test-single-1-count-pass.json",

		//raw results generated with: go test -v -tags relic -count=5 -json ./model/encodable/. -test.run TestEncodableRandomBeaconPrivKeyMsgPack
		//multiple nil tests in a row
		"5 nil tests in a row": "test-result-nil-test-single-5-count-pass.json",

		//normal test at the end of a test run with multiple nil tests in front of it
		"4 nil tests in a row, 1 normal test": "test-result-nil-test-single-5-count-4-nil-1-normal-pass.json",

		// raw results generated with: go test -v -tags relic -count=3 -json ./model/encodable/.
		// group of unit tests with a single nil test result
		"3 count nil test with normal tests": "test-result-nil-test-others-normal-3-count-pass.json",
	}

	for k, testJsonData := range testDataMap {
		t.Run(k, func(t *testing.T) {
			runProcessSummary1TestRun(t, testJsonData)
		})
	}
}

// HELPERS - UTILITIES

func runProcessSummary1TestRun(t *testing.T, jsonExpectedActualFile string) {
	const expectedJsonFilePath = "../testdata/summary1/expected"
	const rawJsonFilePath = "../testdata/summary1/raw"

	var expectedTestRun common.TestRun
	// read in expected JSON from file
	expectedJsonBytes, err := os.ReadFile(filepath.Join(expectedJsonFilePath, jsonExpectedActualFile))
	require.Nil(t, err)
	require.NotEmpty(t, expectedJsonBytes)

	err = json.Unmarshal(expectedJsonBytes, &expectedTestRun)
	require.Nil(t, err)
	require.NotEmpty(t, expectedTestRun.Rows)

	// these hard coded values simulate a real test run that would obtain these environment variables dynamically
	// we are simulating this scenario by setting the environment variables explicitly in the test before calling the main processing script which will look for them
	// these values are uses in testdata/expected/*.json files
	require.NoError(t, os.Setenv("COMMIT_DATE", "2021-09-21T18:06:25-07:00"))
	require.NoError(t, os.Setenv("COMMIT_SHA", "46baf6c6be29af9c040bc14195e195848598bbae"))
	require.NoError(t, os.Setenv("JOB_STARTED", "2021-09-21T21:06:25-07:00"))

	// simulate generating raw "go test -json" output by loading output from saved file
	resultReader := FileResultReader{
		rawJsonFile: filepath.Join(rawJsonFilePath, jsonExpectedActualFile),
	}
	actualTestRun := processSummary1TestRun(&resultReader)

	checkTestRuns(t, expectedTestRun, actualTestRun)
}

func checkTestRuns(t *testing.T, expectedTestRun common.TestRun, actualTestRun common.TestRun) {
	// number of TestResults should be the same between expected and actual
	require.Equal(t, len(expectedTestRun.Rows), len(actualTestRun.Rows))

	// check that all expected TestResults are in actual TestResults
	for actualRowIndex := range actualTestRun.Rows {
		// require.Contains(t, actualTestRun.Rows, expectedTestRun.Rows[actualRowIndex], "expected TestResult doesn't exist in actual: ", expectedTestRun.Rows[actualRowIndex].TestResult)
		require.Contains(t, actualTestRun.Rows, expectedTestRun.Rows[actualRowIndex], printTestResult(expectedTestRun.Rows[actualRowIndex].TestResult))
	}

	// check that all actual TestResults are in expected TestResults
	for exptedRowIndex := range expectedTestRun.Rows {
		require.Contains(t, expectedTestRun.Rows, actualTestRun.Rows[exptedRowIndex], printTestResult(actualTestRun.Rows[exptedRowIndex].TestResult))
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

func printTestResult(testResult common.TestResult) string {
	builder := strings.Builder{}
	builder.WriteString("*** Test Result (not found) ***")
	builder.WriteString("\nTest: " + testResult.Test)
	builder.WriteString("\nCommit SHA: " + testResult.CommitSha)
	builder.WriteString("\nPackage: " + testResult.Package)
	builder.WriteString("\nCommit Date: " + testResult.CommitDate.String())
	builder.WriteString("\nJob Run Date: " + testResult.JobRunDate.String())
	builder.WriteString("\nElapsed: " + fmt.Sprintf("%f", testResult.Elapsed))
	builder.WriteString("\nResult: " + testResult.Result)
	for i, bar := range testResult.Output {
		builder.WriteString("\nOutput[" + fmt.Sprintf("%d", i) + "]" + bar.Item)
	}
	return builder.String()
}
