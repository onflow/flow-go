package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/tools/test_monitor/common/testdata"
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
			ExpectedLevel1Summary: testdata.GetTestData_Level1_1CountAllPass(),
			RawJSONTestRunFile:    "test-result-crypto-hash-1-count-skip-pass.json",
		},

		// raw results generated with: go test -json -count 1 ./utils/unittest/...
		"2 count all pass": {
			ExpectedLevel1Summary: testdata.GetTestData_Level1_2CountPass(),
			RawJSONTestRunFile:    "test-result-crypto-hash-2-count-pass.json",
		},

		// raw results generated with: go test -json -count 1 ./utils/unittest/...
		"10 count all pass": {
			ExpectedLevel1Summary: testdata.GetTestData_Level1_10CountPass(),
			RawJSONTestRunFile:    "test-result-crypto-hash-10-count-pass.json",
		},

		// raw results generated with: go test -json -count 1 ./utils/unittest/...
		"10 count some failures": {
			ExpectedLevel1Summary: testdata.GetTestData_Level1_10CountSomeFailures(),
			RawJSONTestRunFile:    "test-result-crypto-hash-10-count-fail.json",
		},

		// no result tests - tests below don't generate pass/fail result due to `go test` bug
		// with using `fmt.printf("log message")` without newline `\n`

		// raw results generated with: go test -v -count=1 -json ./model/encodable/. -test.run TestEncodableRandomBeaconPrivKeyMsgPack
		// this is a single unit test that produces a no result
		"1 count single no result test": {
			ExpectedLevel1Summary: testdata.GetTestData_Level1_1CountSingleExceptionTest(),
			RawJSONTestRunFile:    "test-result-exception-single-1-count-pass.json",
		},

		//raw results generated with: go test -v -count=5 -json ./model/encodable/. -test.run TestEncodableRandomBeaconPrivKeyMsgPack
		//multiple no result tests in a row
		"5 no result tests in a row": {
			ExpectedLevel1Summary: testdata.GetTestData_Level1_5CountSingleExceptionTest(),
			RawJSONTestRunFile:    "test-result-exception-single-5-count-pass.json",
		},

		//normal test at the end of a test run with multiple no result tests in front of it
		"4 no result tests in a row, 1 normal test": {
			ExpectedLevel1Summary: testdata.GetTestData_Level1_5CountMultipleExceptionTests(),
			RawJSONTestRunFile:    "test-result-exception-single-5-count-4-nil-1-normal-pass.json",
		},

		// raw results generated with: go test -v -count=3 -json ./model/encodable/.
		// group of unit tests with a single no result test
		"3 count no result test with normal tests": {
			ExpectedLevel1Summary: testdata.GetTestData_Leve1_3CountExceptionWithNormalTests(),
			RawJSONTestRunFile:    "test-result-exception-others-normal-3-count-pass.json",
		},
	}

	require.NoError(t, os.Setenv("COMMIT_DATE", testdata.COMMIT_DATE))
	require.NoError(t, os.Setenv("COMMIT_SHA", testdata.COMMIT_SHA))
	require.NoError(t, os.Setenv("JOB_STARTED", testdata.JOB_STARTED))
	require.NoError(t, os.Setenv("RUN_ID", testdata.RUN_ID))

	for k, testData := range testDataMap {
		t.Run(k, func(t *testing.T) {
			// simulate generating raw "go test -json" output by loading output from saved file
			resultReader := FileResultReader{
				rawJsonFile: filepath.Join(rawJsonFilePath, testData.RawJSONTestRunFile),
			}
			// *****************************************************
			actualLevel1Summary, _ := generateLevel1Summary(&resultReader)
			// *****************************************************

			require.ElementsMatch(t, testData.ExpectedLevel1Summary, actualLevel1Summary, "actual and expected level 1 summary do not match")
		})
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
