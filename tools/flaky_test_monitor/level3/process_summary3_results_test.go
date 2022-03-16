package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/onflow/flow-go/tools/flaky_test_monitor/common"

	"github.com/stretchr/testify/require"
)

func TestGenerateLevel3Summary(t *testing.T) {
	testDataMap := map[string]string{
		"1 failure the rest pass":                          "test1-1package-1failure",
		"1 no-result test, no other tests":                 "test2-1-no-result-test",
		"many no-result tests":                             "test3-multi-no-result-tests",
		"many failures, many passes":                       "test4-multi-failures",
		"many failures, many passes, many no-result tests": "test5-multi-durations",
		"many failures - cap failures":                     "test6-multi-failures-cap",
		"many durations - cap durations":                   "test7-multi-durations-cap",
	}

	for k, testDir := range testDataMap {
		t.Run(k, func(t *testing.T) {
			runGenerateLevel3Summary(t, testDir)
		})
	}

}

const testDataDir = "../testdata/summary3"

func runGenerateLevel3Summary(t *testing.T, testDir string) {

	testDataBaseDir := filepath.Join(testDataDir, testDir)
	inputTestDataPath := filepath.Join(testDataBaseDir, "input", testDir+".json")
	expectedOutputTestDataPath := filepath.Join(testDataBaseDir, "expected-output", testDir+".json")

	// **************************************************************
	actualTestSummary3 := generateLevel3Summary(inputTestDataPath, filepath.Join(testDataBaseDir, "input"))
	// **************************************************************

	// read in expected summary level 3
	var expectedTestSummary3 common.Level3Summary
	expectedTestSummary3JsonBytes, err := os.ReadFile(expectedOutputTestDataPath)
	require.Nil(t, err)
	require.NotEmpty(t, expectedTestSummary3JsonBytes)
	err = json.Unmarshal(expectedTestSummary3JsonBytes, &expectedTestSummary3)
	require.Nil(t, err)

	// check all details of test summary level 2 between expected and actual

	// check # of no-results, failures and longest durations is the same for expected vs actual
	require.Equal(t, len(expectedTestSummary3.NoResults), len(actualTestSummary3.NoResults))

	require.Equal(t, len(expectedTestSummary3.MostFailures), len(actualTestSummary3.MostFailures))
	require.Equal(t, expectedTestSummary3.MostFailuresTotal, actualTestSummary3.MostFailuresTotal)

	require.Equal(t, len(expectedTestSummary3.LongestRunning), len(actualTestSummary3.LongestRunning))
	require.Equal(t, expectedTestSummary3.LongestRunningTotal, actualTestSummary3.LongestRunningTotal)

	// check no-result, failure and duration lists are the same for expected vs actual
	for noResultsIndex := range expectedTestSummary3.NoResults {
		common.AssertLevel2TestResults(t, expectedTestSummary3.NoResults[noResultsIndex], actualTestSummary3.NoResults[noResultsIndex])
	}

	for failuresIndex := range expectedTestSummary3.MostFailures {
		common.AssertLevel2TestResults(t, expectedTestSummary3.MostFailures[failuresIndex], actualTestSummary3.MostFailures[failuresIndex])
	}

	for durationIndex := range expectedTestSummary3.LongestRunning {
		common.AssertLevel2TestResults(t, expectedTestSummary3.LongestRunning[durationIndex], actualTestSummary3.LongestRunning[durationIndex])
	}

	require.Equal(t, expectedTestSummary3, actualTestSummary3)
}

// test that script panics when supplied file path is invalid (can't find file)
func TestGenerateLevel3Summary_Panic_WrongPath(t *testing.T) {
	require.PanicsWithValue(t, "error reading level 2 json: open foobar: no such file or directory",
		func() {
			generateLevel3Summary("foobar", ".")
		})
}

// test that script panics when supplied file is not valid level 2 format
func TestGenerateLevel3Summary_Panic_WrongFormat(t *testing.T) {
	require.PanicsWithValue(t, "invalid summary 2 file - no test results found",
		func() {
			// supplied file is level 3 file, not level 2 - this should cause a panic
			generateLevel3Summary(filepath.Join(testDataDir, "test1-1package-1failure/expected-output/test1-1package-1failure.json"), ".")
		})
}
