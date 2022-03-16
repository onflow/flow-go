package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/tools/flaky_test_monitor/common"
	"github.com/onflow/flow-go/tools/flaky_test_monitor/common/testdata"
)

func TestGenerateLevel2Summary_Struct(t *testing.T) {
	testDataMap := map[string]testdata.Level2TestData{
		"1 level 1 summary, 1 failure the rest pass": {
			Directory:        "test1-1package-1failure",
			HasFailures:      true,
			HasNoResultTests: false,
			Level1Summaries:  testdata.GetTestData_Level2_1FailureRestPass(),
		},

		"1 level 1 summary, 1 no-result test, no other tests": {
			Directory:        "test2-1-no-result-test",
			HasFailures:      false,
			HasNoResultTests: true,
			Level1Summaries:  testdata.GetTestsData_Level2_1NoResultNoOtherTests(),
		},

		"many level 1 summaries, many no-result tests": {
			Directory:        "test3-multi-no-result-tests",
			HasFailures:      false,
			HasNoResultTests: true,
			Level1Summaries:  testdata.GetTestData_Level2_MultipleL1SummariesNoResults(),
		},

		"many level 1 summaries, many failures, many passes": {
			Directory:        "test4-multi-failures",
			HasFailures:      true,
			HasNoResultTests: false,
			Level1Summaries:  testdata.GetTestData_Level2MultipleL1SummariesFailuresPasses(),
		},

		"many level 1 summaries, many failures, many passes, many no-result tests": {
			Directory:        "test5-multi-failures-multi-no-result-tests",
			HasFailures:      true,
			HasNoResultTests: true,
			Level1Summaries:  testdata.GetTestData_Level2MultipleL1SummariesFailuresPassesNoResults(),
		},
	}

	for k, testData := range testDataMap {
		t.Run(k, func(t *testing.T) {
			setUp(t)
			// ************************
			actualLevel2Summary := generateLevel2SummaryFromStructs(testData.Level1Summaries)
			// ************************
			checkLevel2Summary(t, actualLevel2Summary, testData)
			tearDown(t)
		})
	}
}

// TestGenerateLevel2Summary_JSON uses real level 1 JSON files as input
// Don't want to use too many tests since they are more brittle to changes to JSON data structure.
// That's why have very few of these. For new tests, it's best to add level 1 data as structs.
func TestGenerateLevel2Summary_JSON(t *testing.T) {
	testDataMap := map[string]testdata.Level2TestData{
		"1 level 1 summary, 1 failure the rest pass": {
			Directory:        "test1-1package-1failure",
			Level1DataPath:   "../testdata/summary2/test1-1package-1failure/input",
			HasFailures:      true,
			HasNoResultTests: false,
		},
	}
	for k, testData := range testDataMap {
		t.Run(k, func(t *testing.T) {
			setUp(t)
			// ************************
			actualLevel2Summary := generateLevel2Summary(testData.Level1DataPath)
			// ************************
			checkLevel2Summary(t, actualLevel2Summary, testData)
			tearDown(t)
		})
	}
}

func setUp(t *testing.T) {
	deleteMessagesDir(t)
}

func tearDown(t *testing.T) {
	deleteMessagesDir(t)
}

// HELPERS - UTILITIES

const actualFailureMessagesPath = "./failures"
const actualNoResultMessagesPath = "./no-results"

func deleteMessagesDir(t *testing.T) {
	// delete failure test dir that stores failure messages
	err := os.RemoveAll(actualFailureMessagesPath)
	require.Nil(t, err)

	// delete no-result test dir that stores no-result messages
	err = os.RemoveAll(actualNoResultMessagesPath)
	require.Nil(t, err)
}

func checkLevel2Summary(t *testing.T, actualLevel2Summary common.Level2Summary, testData testdata.Level2TestData) {
	expectedOutputTestDataPath := filepath.Join("../testdata/summary2", testData.Directory, "expected-output", testData.Directory+".json")
	expectedFailureMessagesPath := filepath.Join("../testdata/summary2", testData.Directory, "expected-output/failures")
	expectedNoResultMessagesPath := filepath.Join("../testdata/summary2", testData.Directory, "expected-output/no-results")

	// read in expected summary level 2
	var expectedLevel2Summary common.Level2Summary
	expectedLevel2SummaryJsonBytes, err := os.ReadFile(expectedOutputTestDataPath)
	require.Nil(t, err)
	require.NotEmpty(t, expectedLevel2SummaryJsonBytes)
	err = json.Unmarshal(expectedLevel2SummaryJsonBytes, &expectedLevel2Summary)
	require.Nil(t, err)

	require.Equal(t, len(expectedLevel2Summary.TestResultsMap), len(actualLevel2Summary.TestResultsMap))

	// check every expected test runs level 2 summary exists in map of actual test runs level 2 summaries
	for expectedLevel2TestResultKey := range expectedLevel2Summary.TestResultsMap {
		expectedLevel2TestResult := expectedLevel2Summary.TestResultsMap[expectedLevel2TestResultKey]
		actualLevel2TestResults, isFoundActual := actualLevel2Summary.TestResultsMap[expectedLevel2TestResultKey]

		require.True(t, isFoundActual, common.PrintLevel2TestResult(expectedLevel2TestResult, "expected not in actual"))

		common.AssertLevel2TestResults(t, *expectedLevel2TestResult, *actualLevel2TestResults)
	}

	// check every actual test runs level 2 summary exists in map of expected test runs level 2 summaries
	for actualLevel2TestResultKey := range actualLevel2Summary.TestResultsMap {
		actualLevel2TestResult := actualLevel2Summary.TestResultsMap[actualLevel2TestResultKey]
		exptectedLevel2TestResult, isFoundExpected := expectedLevel2Summary.TestResultsMap[actualLevel2TestResultKey]

		require.True(t, isFoundExpected, common.PrintLevel2TestResult(actualLevel2TestResult, "actual not in expected"))

		common.AssertLevel2TestResults(t, *exptectedLevel2TestResult, *actualLevel2TestResult)
	}

	checkFailureMessages(t, testData.HasFailures, expectedFailureMessagesPath)
	checkNoResultMessages(t, testData.HasNoResultTests, expectedNoResultMessagesPath)
}

// check failure messages created
// there are 2 types of scenarios:
// 1. test summaries with no failures - these will not have a `failures` sub-directory and no more checking is needed
// 2. test summaries with failures - these will have a `failures` sub-directory with failure messages saved
//    in text files (1 file/failure under separate sub-directory for each test that has failures)
func checkFailureMessages(t *testing.T, hasFailures bool, expectedFailureMessagesPath string) {
	if !hasFailures {
		return
	}
	checkMessagesHelper(t, expectedFailureMessagesPath, actualFailureMessagesPath)
}

// check no-result messages created - for tests that generated no pass / fail
// there are 2 types of scenarios:
// 1. test summaries with no "no-result" - these will not have a `no-result` sub-directory and no more checking is needed
// 2. test summaries with no-results - these will have a `no-result` sub-directory with output messages saved
//    in text files (1 file/no-result under separate sub-directory for each test that has no-results)
func checkNoResultMessages(t *testing.T, hasNoResultTests bool, expectedNoResultMessagesPath string) {
	if !hasNoResultTests {
		return
	}
	checkMessagesHelper(t, expectedNoResultMessagesPath, actualNoResultMessagesPath)
}

// helps check for both failures and no-result messages since they are very similar, just under
// different directories
func checkMessagesHelper(t *testing.T, expectedMessagesPath string, actualMessagesPath string) {

	// count expected failure / no-result directories (1 directory/test)
	expectedMessageDirs, err := os.ReadDir(expectedMessagesPath)
	require.Nil(t, err)

	// count actual failure / no-result directories
	actualMessageDirs, err := os.ReadDir(actualMessagesPath)
	require.Nil(t, err)

	// expected test summary has at least 1 failure / no-result
	require.Equal(t, len(expectedMessageDirs), len(actualMessageDirs))

	// compare expected vs actual messages
	for expectedMessageDirIndex, expectedMessageDir := range expectedMessageDirs {

		// sub-directory names should be the same - each sub directory corresponds to a failed / no-result test name
		require.Equal(t, expectedMessageDir.Name(), actualMessageDirs[expectedMessageDirIndex].Name())

		// under each sub-directory, there should be 1 or more text files (failure1.txt/no-result1.txt, failure2.txt/no-result2.txt, etc)
		// that holds the raw failure / no-result message for that test
		expectedMessagesDirFiles, err := os.ReadDir(filepath.Join(expectedMessagesPath, expectedMessageDir.Name()))
		require.Nil(t, err)

		actualMessageDirFiles, err := os.ReadDir(filepath.Join(actualMessagesPath, actualMessageDirs[expectedMessageDirIndex].Name()))
		require.Nil(t, err)

		// make sure there are the expected number of failed / no-result text files in the sub-directory
		require.Equal(t, len(expectedMessagesDirFiles), len(actualMessageDirFiles))

		// check contents of each text file for expected failure / no-result message
		// for every test that has failures / no-result, there should be 1 text file per failure / no-result

		// if test has failures / no-results, there should be directory of failure / no-result messages text files
		// a sub-directory of the test name will hold all test failure / no-result messages

		for expectedMessageFileIndex, expectedMessageFileDirEntry := range expectedMessagesDirFiles {
			expectedMessageFilePath := filepath.Join(expectedMessagesPath, expectedMessageDir.Name(), expectedMessageFileDirEntry.Name())
			expectedMessageFileBytes, err := os.ReadFile(expectedMessageFilePath)
			require.Nil(t, err)

			actualMessageFilePath := filepath.Join(actualMessagesPath, actualMessageDirs[expectedMessageDirIndex].Name(), actualMessageDirFiles[expectedMessageFileIndex].Name())
			actualMessageFileBytes, err := os.ReadFile(actualMessageFilePath)
			require.Nil(t, err)

			// read expected and actual text files as bytes and compare them all at once
			require.Equal(t, expectedMessageFileBytes, actualMessageFileBytes)
		}
	}
}
