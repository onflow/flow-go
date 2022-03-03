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
	"github.com/onflow/flow-go/tools/flaky_test_monitor/common/testdata"
)

func TestGenerateLevel1Summary_Struct(t *testing.T) {
	const rawJsonFilePath = "../testdata/summary1/raw"

	// data driven table test
	testDataMap := map[string]testdata.Level1TestData{
		"1 count all pass": {
			ExpectedLevel1Summary: testdata.GetTestData_Level1_1CountPass(),
			RawJSONTestRunFile:    "test-result-crypto-hash-1-count-pass.json",
		},

		"1 count 1 fail the rest pass": {
			ExpectedLevel1Summary: testdata.GetTestData_Level1_1Count1FailRestPass(),
			RawJSONTestRunFile:    "test-result-crypto-hash-1-count-fail.json",
		},

		"1 count 2 skipped the rest pass": {
			ExpectedLevel1Summary: testdata.GetTestData_Level1_1Count2SkippedRestPass(),
			RawJSONTestRunFile:    "test-result-crypto-hash-1-count-skip-pass.json",
		},

		// raw results generated with: go test -json -count 1 --tags relic ./utils/unittest/...
		"2 count all pass": {
			ExpectedLevel1Summary: testdata.GetTestData_Level1_2CountPass(),
			RawJSONTestRunFile:    "test-result-crypto-hash-2-count-pass.json",
		},

		// raw results generated with: go test -json -count 1 --tags relic ./utils/unittest/...
		"10 count all pass": {
			ExpectedLevel1Summary: testdata.GetTestData_Level1_10CountPass(),
			RawJSONTestRunFile:    "test-result-crypto-hash-10-count-pass.json",
		},

		// raw results generated with: go test -json -count 1 --tags relic ./utils/unittest/...
		"10 count some failures": {
			ExpectedLevel1Summary: testdata.GetTestData_Level1_10CountSomeFailures(),
			RawJSONTestRunFile:    "test-result-crypto-hash-10-count-fail.json",
		},

		// nil tests - tests below don't generate pass/fail result due to `go test` bug
		// with using `fmt.printf("log message")` without newline `\n`

		// raw results generated with: go test -v -tags relic -count=1 -json ./model/encodable/. -test.run TestEncodableRandomBeaconPrivKeyMsgPack
		// this is a single unit test that produces a nil test result
		"1 count single nil test": {
			ExpectedLevel1Summary: testdata.GetTestData_Level1_1CountSingleNilTest(),
			RawJSONTestRunFile:    "test-result-nil-test-single-1-count-pass.json",
		},

		//raw results generated with: go test -v -tags relic -count=5 -json ./model/encodable/. -test.run TestEncodableRandomBeaconPrivKeyMsgPack
		//multiple nil tests in a row
		"5 nil tests in a row": {
			ExpectedLevel1Summary: testdata.GetTestData_Level1_5CountSingleNilTest(),
			RawJSONTestRunFile:    "test-result-nil-test-single-5-count-pass.json",
		},

		//normal test at the end of a test run with multiple nil tests in front of it
		"4 nil tests in a row, 1 normal test": {
			ExpectedLevel1Summary: testdata.GetTestData_Level1_5CountMultipleNilTests(),
			RawJSONTestRunFile:    "test-result-nil-test-single-5-count-4-nil-1-normal-pass.json",
		},

		// raw results generated with: go test -v -tags relic -count=3 -json ./model/encodable/.
		// group of unit tests with a single nil test result
		"3 count nil test with normal tests": {
			ExpectedLevel1Summary: testdata.GetTestData_Leve1_3CountNilWithNormalTests(),
			RawJSONTestRunFile:    "test-result-nil-test-others-normal-3-count-pass.json",
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
			// *****************************************************
			actualLevel1Summary := generateLevel1Summary(&resultReader)
			// *****************************************************
			checkLevel1Summaries(t, testData.ExpectedLevel1Summary, actualLevel1Summary)
		})
	}
}

// TestGenerateLevel1Summary_JSON uses real level 1 JSON files as expected output.
// Don't want to use too many tests since they are more brittle to changes to JSON data structure.
// That's why have very few of these. For new tests, it's best to add level 1 data as structs.
func TestGenerateLevel1Summary_JSON(t *testing.T) {
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
			// *****************************************************
			actualLevel1Summary := runGenerateLevel1Summary(t, testJsonData)
			// *****************************************************
			expectedLevel1Summary := getExpectedLevel1SummaryFromJSON(t, testJsonData)
			checkLevel1Summaries(t, expectedLevel1Summary, actualLevel1Summary)
		})
	}
}

// HELPERS - UTILITIES

func getExpectedLevel1SummaryFromJSON(t *testing.T, file string) common.Level1Summary {
	const expectedJsonFilePath = "../testdata/summary1/expected"

	var expectedLevel1Summary common.Level1Summary
	// read in expected JSON from file
	expectedJsonBytes, err := os.ReadFile(filepath.Join(expectedJsonFilePath, file))
	require.Nil(t, err)
	require.NotEmpty(t, expectedJsonBytes)

	err = json.Unmarshal(expectedJsonBytes, &expectedLevel1Summary)
	require.Nil(t, err)
	require.NotEmpty(t, expectedLevel1Summary.Rows)

	return expectedLevel1Summary
}

func runGenerateLevel1Summary(t *testing.T, jsonExpectedActualFile string) common.Level1Summary {
	const rawJsonFilePath = "../testdata/summary1/raw"

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
	// *****************************************************
	actualLevel1Summary := generateLevel1Summary(&resultReader)
	// *****************************************************
	return actualLevel1Summary
}

func checkLevel1Summaries(t *testing.T, expectedLevel1Summary common.Level1Summary, actualLevel1Summary common.Level1Summary) {
	// number of results should be the same between expected and actual
	require.Equal(t, len(expectedLevel1Summary.Rows), len(actualLevel1Summary.Rows))

	// check that all expected results are in actual results
	for actualRowIndex := range actualLevel1Summary.Rows {
		require.Contains(t, actualLevel1Summary.Rows, expectedLevel1Summary.Rows[actualRowIndex], printLevel1TestResult(expectedLevel1Summary.Rows[actualRowIndex].TestResult, "expected not in actual"))
	}

	// check that all actual results are in expected results
	for exptedRowIndex := range expectedLevel1Summary.Rows {
		require.Contains(t, expectedLevel1Summary.Rows, actualLevel1Summary.Rows[exptedRowIndex], printLevel1TestResult(actualLevel1Summary.Rows[exptedRowIndex].TestResult, "actual not in expected"))
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

func printLevel1TestResult(level1TestResult common.Level1TestResult, message string) string {
	builder := strings.Builder{}
	builder.WriteString("*** Test Result (not found) " + message + "***")
	builder.WriteString("\nTest: " + level1TestResult.Test)
	builder.WriteString("\nCommit SHA: " + level1TestResult.CommitSha)
	builder.WriteString("\nPackage: " + level1TestResult.Package)
	builder.WriteString("\nCommit Date: " + level1TestResult.CommitDate.String())
	builder.WriteString("\nJob Run Date: " + level1TestResult.JobRunDate.String())
	builder.WriteString("\nElapsed: " + fmt.Sprintf("%f", level1TestResult.Elapsed))
	builder.WriteString("\nResult: " + level1TestResult.Result)
	for i, output := range level1TestResult.Output {
		builder.WriteString("\nOutput[" + fmt.Sprintf("%d", i) + "]" + output.Item)
	}
	return builder.String()
}
