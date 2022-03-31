package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/tools/flaky_test_monitor/common"
	"github.com/onflow/flow-go/tools/flaky_test_monitor/common/testdata"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestGenerateLevel2Summary_Struct(t *testing.T) {
	testDataMap := map[string]testdata.Level2TestData{
		"1 level 1 summary, 1 failure the rest pass": {
			Directory:       "test1-1package-1failure",
			HasFailures:     true,
			HasExceptions:   false,
			Level1Summaries: testdata.GetTestData_Level2_1FailureRestPass(),
		},

		"1 level 1 summary, 1 exception, no other tests": {
			Directory:       "test2-1-exception",
			HasFailures:     false,
			HasExceptions:   true,
			Level1Summaries: testdata.GetTestsData_Level2_1ExceptionNoOtherTests(),
		},

		"many level 1 summaries, many exceptions": {
			Directory:       "test3-multi-exceptions",
			HasFailures:     false,
			HasExceptions:   true,
			Level1Summaries: testdata.GetTestData_Level2_MultipleL1SummariesExceptions(),
		},

		"many level 1 summaries, many failures, many passes": {
			Directory:       "test4-multi-failures",
			HasFailures:     true,
			HasExceptions:   false,
			Level1Summaries: testdata.GetTestData_Level2MultipleL1SummariesFailuresPasses(),
		},

		"many level 1 summaries, many failures, many passes, many exceptions": {
			Directory:       "test5-multi-failures-multi-exceptions",
			HasFailures:     true,
			HasExceptions:   true,
			Level1Summaries: testdata.GetTestData_Level2MultipleL1SummariesFailuresPassesExceptions(),
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
	unittest.SkipUnless(t, unittest.TEST_TODO, "broken")
	testDataMap := map[string]testdata.Level2TestData{
		"1 level 1 summary, 1 failure the rest pass": {
			Directory:      "test1-1package-1failure",
			Level1DataPath: "../testdata/summary2/test1-1package-1failure/input",
			HasFailures:    true,
			HasExceptions:  false,
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
const actualExceptionMessagesPath = "./exceptions"

func deleteMessagesDir(t *testing.T) {
	// delete failure test dir that stores failure messages
	err := os.RemoveAll(actualFailureMessagesPath)
	require.Nil(t, err)

	// delete exceptions test dir that stores exception messages
	err = os.RemoveAll(actualExceptionMessagesPath)
	require.Nil(t, err)
}

func checkLevel2Summary(t *testing.T, actualLevel2Summary common.Level2Summary, testData testdata.Level2TestData) {
	expectedOutputTestDataPath := filepath.Join("../testdata/summary2", testData.Directory, "expected-output", testData.Directory+".json")
	expectedFailureMessagesPath := filepath.Join("../testdata/summary2", testData.Directory, "expected-output/failures")
	expectedExceptionMessagesPath := filepath.Join("../testdata/summary2", testData.Directory, "expected-output/exceptions")

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
	checkExceptionMessages(t, testData.HasExceptions, expectedExceptionMessagesPath)
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

// check exception messages created - for tests that generated no pass / fail
// there are 2 types of scenarios:
// 1. test summaries with no "exception" - these will not have an `exception` sub-directory and no more checking is needed
// 2. test summaries with exceptions - these will have an `exception` sub-directory with output messages saved
//    in text files (1 file/exception under separate sub-directory for each test that has exceptions)
func checkExceptionMessages(t *testing.T, HasExceptions bool, expectedExceptionMessagesPath string) {
	if !HasExceptions {
		return
	}
	checkMessagesHelper(t, expectedExceptionMessagesPath, actualExceptionMessagesPath)
}

// helps check for both failures and exception messages since they are very similar, just under
// different directories
func checkMessagesHelper(t *testing.T, expectedMessagesPath string, actualMessagesPath string) {

	// count expected failure / exception directories (1 directory/test)
	expectedMessageDirs, err := os.ReadDir(expectedMessagesPath)
	require.Nil(t, err)

	// count actual failure / exception directories
	actualMessageDirs, err := os.ReadDir(actualMessagesPath)
	require.Nil(t, err)

	// expected test summary has at least 1 failure / exception
	require.Equal(t, len(expectedMessageDirs), len(actualMessageDirs))

	// compare expected vs actual messages
	for expectedMessageDirIndex, expectedMessageDir := range expectedMessageDirs {

		// sub-directory names should be the same - each sub directory corresponds to a failed / exception test name
		require.Equal(t, expectedMessageDir.Name(), actualMessageDirs[expectedMessageDirIndex].Name())

		// under each sub-directory, there should be 1 or more text files (failure1.txt/exception1.txt, failure2.txt/exception2.txt, etc)
		// that holds the raw failure / exception message for that test
		expectedMessagesDirFiles, err := os.ReadDir(filepath.Join(expectedMessagesPath, expectedMessageDir.Name()))
		require.Nil(t, err)

		actualMessageDirFiles, err := os.ReadDir(filepath.Join(actualMessagesPath, actualMessageDirs[expectedMessageDirIndex].Name()))
		require.Nil(t, err)

		// make sure there are the expected number of failed / exception text files in the sub-directory
		require.Equal(t, len(expectedMessagesDirFiles), len(actualMessageDirFiles))

		// check contents of each text file for expected failure / exception message
		// for every test that has failures / exception, there should be 1 text file per failure / exception

		// if test has failures / exceptions, there should be directory of failure / exception messages text files
		// a sub-directory of the test name will hold all test failure / exception messages

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
