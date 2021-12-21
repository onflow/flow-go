package main

import (
	"encoding/json"
	"flaky-test-monitor/common"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProcessSummary3TestRun(t *testing.T) {
	testDataMap := map[string]string{
		"1 failure the rest pass":                                                  "test1-1package-1failure",
		"1 no-result test, no other tests":                                         "test2-1-no-result-test",
		"many level 1 summaries, many no-result tests":                             "test3-multi-no-result-tests",
		"many level 1 summaries, many failures, many passes":                       "test4-multi-failures",
		"many level 1 summaries, many failures, many passes, many no-result tests": "test5-multi-durations",
	}

	for k, testDir := range testDataMap {
		t.Run(k, func(t *testing.T) {
			runProcessSummary3TestRun(t, testDir)
		})
	}

}

const testDataDir = "../testdata/summary3/"

func runProcessSummary3TestRun(t *testing.T, testDir string) {
	testDataBaseDir := testDataDir + testDir
	inputTestDataPath := testDataBaseDir + "/input/" + testDir + ".json"
	expectedOutputTestDataPath := testDataBaseDir + "/expected-output/" + testDir + ".json"

	// **************************************************************
	actualTestSummary3 := processSummary3TestRun(inputTestDataPath)
	// **************************************************************

	// read in expected summary level 3
	var expectedTestSummary3 common.TestSummary3
	expectedTestSummary3JsonBytes, err := os.ReadFile(expectedOutputTestDataPath)
	require.Nil(t, err)
	require.NotEmpty(t, expectedTestSummary3JsonBytes)
	err = json.Unmarshal(expectedTestSummary3JsonBytes, &expectedTestSummary3)
	require.Nil(t, err)

	// check all details of test summary level 2 between expected and actual

	// check # of no-results, failures and longest durations is the same for expected vs actual
	require.Equal(t, len(expectedTestSummary3.NoResults), len(actualTestSummary3.NoResults))
	require.Equal(t, len(expectedTestSummary3.MostFailures), len(actualTestSummary3.MostFailures))
	require.Equal(t, len(expectedTestSummary3.LongestRunning), len(actualTestSummary3.LongestRunning))

	// check no-result, failure and duration lists are the same for expected vs actual
	for noResultsIndex := range expectedTestSummary3.NoResults {
		common.AssertTestSummariesEqual(t, expectedTestSummary3.NoResults[noResultsIndex], actualTestSummary3.NoResults[noResultsIndex])
	}

	for failuresIndex := range expectedTestSummary3.MostFailures {
		common.AssertTestSummariesEqual(t, expectedTestSummary3.MostFailures[failuresIndex], actualTestSummary3.MostFailures[failuresIndex])
	}

	for durationIndex := range expectedTestSummary3.LongestRunning {
		common.AssertTestSummariesEqual(t, expectedTestSummary3.LongestRunning[durationIndex], actualTestSummary3.LongestRunning[durationIndex])
	}

	require.Equal(t, expectedTestSummary3, actualTestSummary3)
}

var expectedPanicFunc_WrongPath = func() {
	processSummary3TestRun("foobar")
}

// test that script panics when supplied file path is invalid (can't find file)
func TestProcessSummary3TestRun_Panic_WrongPath(t *testing.T) {
	require.PanicsWithValue(t, "error reading level 2 json: open foobar: no such file or directory",
		expectedPanicFunc_WrongPath)
}

var expectedPanicFun_WrongFormat = func() {
	// supplied file is level 3 file, not level 2 - this should cause a panic
	processSummary3TestRun(testDataDir + "test1-1package-1failure/expected-output/test1-1package-1failure.json")
}

// test that script panics when supplied file is not valid level 2 format
func TestProcessSummary3TestRun_Panic_WrongFormat(t *testing.T) {
	require.PanicsWithValue(t, "invalid summary 2 file - no test results found", expectedPanicFun_WrongFormat)
}
