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

func TestGenerateLevel2Summary_Struct(t *testing.T) {
	testDataMap := map[string]testdata.Level3TestData{
		"1 failure the rest pass": {
			Directory:             "test1-1package-1failure",
			PropertyFileDirectory: filepath.Join(testDataDir, "test1-1package-1failure", "input"),
			InputLevel2Summary:    testdata.GetTestData_Level3_Input_1Package1Failure(),
			ExpectedLevel3Summary: testdata.GetTestData_Level3_Expected_1Package1Failure(),
		},
		"1 no-result test, no other tests": {
			Directory:             "test2-1-no-result-test",
			PropertyFileDirectory: filepath.Join(testDataDir, "test2-1-no-result-test", "input"),
			InputLevel2Summary:    testdata.GetTestData_Level3_Input_1NoResultTest(),
			ExpectedLevel3Summary: testdata.GetTestData_Level3_Expected_1NoResultTest(),
		},
	}

	for k, testData := range testDataMap {
		t.Run(k, func(t *testing.T) {
			actualLevel3Summary := generateLevel3SummaryFromStructs(testData.InputLevel2Summary, testData.PropertyFileDirectory)
			checkLevel3Summary(t, actualLevel3Summary, testData)
		})
	}
}

// TestGenerateLevel3Summary_JSON uses real level 2 JSON file as input
// Don't want to use too many tests since they are more brittle to changes to JSON data structure.
// That's why have very few of these. For new tests, it's best to add level 2 data as structs.
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
			// **************************************************************
			actualTestSummary3 := generateLevel3Summary(testData.InputLevel2SummaryPath, testData.PropertyFileDirectory)
			// **************************************************************
			// read JSON file to determine expected level 3 summary
			testData.ExpectedLevel3Summary = readExpectedLevel3SummaryFromJSON(t, testData)
			checkLevel3Summary(t, actualTestSummary3, testData)
		})
	}

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
