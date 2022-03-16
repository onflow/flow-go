package main

import (
	"encoding/json"
	"github.com/onflow/flow-go/tools/flaky_test_monitor/common/testdata"
	"os"
	"path/filepath"
	"testing"

	"github.com/onflow/flow-go/tools/flaky_test_monitor/common"

	"github.com/stretchr/testify/require"
)

//func TestGenerateLevel2Summary_Struct(t *testing.T) {
//	testDataMap := map[string]testdata.Level3TestData{
//		"1 failure the rest pass": {
//			Directory: "test1-1package-1failure",
//		},
//	}
//
//	for k, testData := range testDataMap {
//		t.Run(k, func(t *testing.T) {
//			actualLevel3Summary := generateLevel3SummaryFromStructs(testData.InputLevel2Summary, testData.PropertyFileDirectory)
//		})
//	}
//}

func TestGenerateLevel3Summary_JSON(t *testing.T) {
	testDataMap := map[string]testdata.Level3TestData{
		//expectedOutputTestDataPath := filepath.Join(testDataBaseDir, "expected-output", testDir+".json")

		"1 failure the rest pass": {
			Directory:                 "test1-1package-1failure",
			InputLevel2SummaryPath:    filepath.Join(testDataDir, "test1-1package-1failure", "input", "test1-1package-1failure.json"),
			PropertyFileDirectory:     filepath.Join(testDataDir, "test1-1package-1failure", "input"),
			ExpectedLevel3SummaryPath: filepath.Join(testDataDir, "test1-1package-1failure", "expected-output", "test1-1package-1failure.json"),
		},
		"1 no-result test, no other tests": {
			Directory:                 "test2-1-no-result-test",
			InputLevel2SummaryPath:    filepath.Join(testDataDir, "test2-1-no-result-test", "input", "test2-1-no-result-test.json"),
			PropertyFileDirectory:     filepath.Join(testDataDir, "test2-1-no-result-test", "input"),
			ExpectedLevel3SummaryPath: filepath.Join(testDataDir, "test2-1-no-result-test", "expected-output", "test2-1-no-result-test.json"),
		},
		"many no-result tests": {
			Directory:                 "test3-multi-no-result-tests",
			InputLevel2SummaryPath:    filepath.Join(testDataDir, "test3-multi-no-result-tests", "input", "test3-multi-no-result-tests.json"),
			PropertyFileDirectory:     filepath.Join(testDataDir, "test3-multi-no-result-tests", "input"),
			ExpectedLevel3SummaryPath: filepath.Join(testDataDir, "test3-multi-no-result-tests", "expected-output", "test3-multi-no-result-tests.json"),
		},
		"many failures, many passes": {
			Directory:                 "test4-multi-failures",
			InputLevel2SummaryPath:    filepath.Join(testDataDir, "test4-multi-failures", "input", "test4-multi-failures.json"),
			PropertyFileDirectory:     filepath.Join(testDataDir, "test4-multi-failures", "input"),
			ExpectedLevel3SummaryPath: filepath.Join(testDataDir, "test4-multi-failures", "expected-output", "test4-multi-failures.json"),
		},
		"many failures, many passes, many no-result tests": {
			Directory:                 "test5-multi-durations",
			InputLevel2SummaryPath:    filepath.Join(testDataDir, "test5-multi-durations", "input", "test5-multi-durations.json"),
			PropertyFileDirectory:     filepath.Join(testDataDir, "test5-multi-durations", "input"),
			ExpectedLevel3SummaryPath: filepath.Join(testDataDir, "test5-multi-durations", "expected-output", "test5-multi-durations.json"),
		},
		"many failures - cap failures": {
			Directory:                 "test6-multi-failures-cap",
			InputLevel2SummaryPath:    filepath.Join(testDataDir, "test6-multi-failures-cap", "input", "test6-multi-failures-cap.json"),
			PropertyFileDirectory:     filepath.Join(testDataDir, "test6-multi-failures-cap", "input"),
			ExpectedLevel3SummaryPath: filepath.Join(testDataDir, "test6-multi-failures-cap", "expected-output", "test6-multi-failures-cap.json"),
		},
		"many durations - cap durations": {
			Directory:                 "test7-multi-durations-cap",
			InputLevel2SummaryPath:    filepath.Join(testDataDir, "test7-multi-durations-cap", "input", "test7-multi-durations-cap.json"),
			PropertyFileDirectory:     filepath.Join(testDataDir, "test7-multi-durations-cap", "input"),
			ExpectedLevel3SummaryPath: filepath.Join(testDataDir, "test7-multi-durations-cap", "expected-output", "test7-multi-durations-cap.json"),
		},
	}

	for k, testData := range testDataMap {
		t.Run(k, func(t *testing.T) {
			// ************************
			//actualLevel3Summary := runGenerateLevel3Summary(t, testData.Directory)
			runGenerateLevel3Summary(t, testData.Directory, testData)
			// ************************
			// read JSON file to determine expected level 3 summary
			testData.ExpectedLevel3Summary = readExpectedLevel3SummaryFromJSON(t, testData)
			//checkLevel3Summary(t, actualLevel3Summary, testData)
		})
	}

}

// HELPERS - UTILITIES

const testDataDir = "../testdata/summary3"

func readExpectedLevel3SummaryFromJSON(t *testing.T, testData testdata.Level3TestData) common.Level3Summary {
	var expectedLevel3Summary common.Level3Summary
	expectedTestSummary3JsonBytes, err := os.ReadFile(testData.ExpectedLevel3SummaryPath)
	require.Nil(t, err)
	require.NotEmpty(t, expectedTestSummary3JsonBytes)
	err = json.Unmarshal(expectedTestSummary3JsonBytes, &expectedLevel3Summary)
	require.Nil(t, err)
	return expectedLevel3Summary
}

func runGenerateLevel3Summary(t *testing.T, testDir string, testData testdata.Level3TestData) common.Level3Summary {

	// **************************************************************
	actualTestSummary3 := generateLevel3Summary(testData.InputLevel2SummaryPath, testData.PropertyFileDirectory)
	// **************************************************************

	expectedTestSummary3 := readExpectedLevel3SummaryFromJSON(t, testData)

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
	return actualTestSummary3
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

func checkLevel3Summary(t *testing.T, actualLevel3Summary common.Level3Summary, testData testdata.Level3TestData) {
	// check # of no-results, failures and longest durations is the same for expected vs actual
	require.Equal(t, len(testData.ExpectedLevel3Summary.NoResults), len(actualLevel3Summary.NoResults))

	require.Equal(t, len(testData.ExpectedLevel3Summary.MostFailures), len(actualLevel3Summary.MostFailures))
	require.Equal(t, testData.ExpectedLevel3Summary.MostFailuresTotal, actualLevel3Summary.MostFailuresTotal)

	require.Equal(t, len(testData.ExpectedLevel3Summary.LongestRunning), len(actualLevel3Summary.LongestRunning))
	require.Equal(t, testData.ExpectedLevel3Summary.LongestRunningTotal, actualLevel3Summary.LongestRunningTotal)

	// check no-result, failure and duration lists are the same for expected vs actual
	for noResultsIndex := range testData.ExpectedLevel3Summary.NoResults {
		common.AssertLevel2TestResults(t, testData.ExpectedLevel3Summary.NoResults[noResultsIndex], actualLevel3Summary.NoResults[noResultsIndex])
	}

	for failuresIndex := range testData.ExpectedLevel3Summary.MostFailures {
		common.AssertLevel2TestResults(t, testData.ExpectedLevel3Summary.MostFailures[failuresIndex], actualLevel3Summary.MostFailures[failuresIndex])
	}

	for durationIndex := range testData.ExpectedLevel3Summary.LongestRunning {
		common.AssertLevel2TestResults(t, testData.ExpectedLevel3Summary.LongestRunning[durationIndex], actualLevel3Summary.LongestRunning[durationIndex])
	}

	require.Equal(t, testData.ExpectedLevel3Summary, actualLevel3Summary)
}
