package main

import (
	"encoding/json"
	"flaky-test-monitor/helpers"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProcessSummary2TestRun(t *testing.T) {
	testDataMap := map[string]string{
		"1 level 1 summary, 1 failure the rest pass":         "test1-1package-1failure",
		"1 level 1 summary, 1 nil test, no other tests":      "test2-1nil-test",
		"many level 1 summaries, many nil tests":             "test3-multi-nil-tests",
		"many level 1 summaries, many failures, many passes": "test4-multi-failures",
	}

	for k, testDir := range testDataMap {
		t.Run(k, func(t *testing.T) {
			setUp(t)
			runProcessSummary2TestRun(t, testDir)
			tearDown(t)
		})
	}
}

func setUp(t *testing.T) {
	deleteFailuresDir(t)
}

func tearDown(t *testing.T) {
	deleteFailuresDir(t)
}

// HELPERS - UTILITIES

const actualFailureTestDataPath = "./failures/"

func deleteFailuresDir(t *testing.T) {
	// delete failure test dir that stores failure messages
	err := os.RemoveAll(actualFailureTestDataPath)
	require.Nil(t, err)
}

func runProcessSummary2TestRun(t *testing.T, testDir string) {
	inputTestDataPath := "../testdata/summary2/" + testDir + "/input/"
	expectedOutputTestDataPath := "../testdata/summary2/" + testDir + "/expected-output/" + testDir + ".json"
	expectedFailureMessagesPath := "../testdata/summary2/" + testDir + "/expected-output/failures/"

	// **************************************************************
	actualTestSummary2 := processSummary2TestRun(inputTestDataPath)
	// **************************************************************

	// read in expected summary level 2
	var expectedTestSummary2 helpers.TestSummary2
	expectedTestSummary2JsonBytes, err := os.ReadFile(expectedOutputTestDataPath)
	require.Nil(t, err)
	require.NotEmpty(t, expectedTestSummary2JsonBytes)
	err = json.Unmarshal(expectedTestSummary2JsonBytes, &expectedTestSummary2)
	require.Nil(t, err)

	//check all details of test summary level 2 between expected and actual
	require.Equal(t, len(expectedTestSummary2.TestResults), len(actualTestSummary2.TestResults))

	for expectedTestResultSummaryKey := range expectedTestSummary2.TestResults {
		expectedTestSummary, isFoundExpected := expectedTestSummary2.TestResults[expectedTestResultSummaryKey]
		actualTestSummary, isFoundActual := actualTestSummary2.TestResults[expectedTestResultSummaryKey]

		require.True(t, isFoundExpected)
		require.True(t, isFoundActual)

		require.Equal(t, expectedTestSummary.Package, actualTestSummary.Package)
		require.Equal(t, expectedTestSummary.Test, actualTestSummary.Test)

		require.Equal(t, expectedTestSummary.Runs, actualTestSummary.Runs)
		require.Equal(t, expectedTestSummary.Passed, actualTestSummary.Passed)
		require.Equal(t, expectedTestSummary.Failed, actualTestSummary.Failed)
		require.Equal(t, expectedTestSummary.Skipped, actualTestSummary.Skipped)
		require.Equal(t, expectedTestSummary.NoResult, actualTestSummary.NoResult)

		require.Equal(t, expectedTestSummary.FailureRate, actualTestSummary.FailureRate)

		// check all durations
		require.Equal(t, len(expectedTestSummary.Durations), len(actualTestSummary.Durations))
		for i := range expectedTestSummary.Durations {
			require.Equal(t, expectedTestSummary.Durations[i], actualTestSummary.Durations[i])
		}
		require.Equal(t, expectedTestSummary.AverageDuration, actualTestSummary.AverageDuration)
	}

	// make sure calculated summary level 2 is what we expected
	require.Equal(t, expectedTestSummary2, actualTestSummary2)

	// check failure messages created
	// there are 2 types of scenarios to test for
	// 1. test summaries with no failures - these will have a `failures` sub-directory with a
	//    `.keep` placeholder file (since empty folders can't be added to git)
	// 2. test summaries with failures - these will have a `failures` sub-directory with failure messages saved
	//    in text files (1 file/failure under separate sub-directory for each test that has failures)

	// count expected failure directories (1 directory/test)
	expectedFailureDirEntries, err := os.ReadDir(expectedFailureMessagesPath)
	require.Nil(t, err)

	// count actual failure directories
	actualFailureDirEntries, err := os.ReadDir(actualFailureTestDataPath)
	require.Nil(t, err)

	// expected test summary `failures` directory only has placeholder file with no expected failures
	if len(expectedFailureDirEntries) == 1 && strings.HasPrefix(expectedFailureDirEntries[0].Name(), ".") {
		// all expected `failures` sub-directories with no expected failues will have a ".keep" file to force saving empty
		// directory to git when there are no expected failures - we need to not count this placeholder file
		require.Equal(t, 0, len(actualFailureDirEntries))
		return
	}

	// expected test summary has at least 1 failure
	require.Equal(t, len(expectedFailureDirEntries), len(actualFailureDirEntries))

	// compare expected vs actual failure messages

	for dirEntryIndex, expectedDirEntry := range expectedFailureDirEntries {

		// sub-directory names should be the same - each sub directory corresponds to a failed test name
		require.Equal(t, expectedDirEntry.Name(), actualFailureDirEntries[dirEntryIndex].Name())

		// under each sub-directory, there should be 1 or more text files (failure1.txt, failure2.txt, etc)
		// that holds the raw failure message for that test
		expectedTestFailuresDirEntries, err := os.ReadDir(expectedFailureMessagesPath + expectedDirEntry.Name())
		require.Nil(t, err)

		actualTestFailuresDirEntries, err := os.ReadDir(actualFailureTestDataPath + actualFailureDirEntries[dirEntryIndex].Name())
		require.Nil(t, err)

		// make sure there are the expected number of failed text files
		require.Equal(t, len(expectedTestFailuresDirEntries), len(actualTestFailuresDirEntries))

		// check contents of each text file for expected failure message
		// for every test that has failures, there should be 1 text file per failure

		// if test has failures, there should be directory of failure messages text files
		// a sub-directory of the test name will hold all test failure messages

		for expectedFailureFileIndex, expectedFailureFileDirEntry := range expectedTestFailuresDirEntries {
			expectedFailureFilePath := expectedFailureMessagesPath + expectedDirEntry.Name() + "/" + expectedFailureFileDirEntry.Name()
			expectedFailureFileBytes, err := os.ReadFile(expectedFailureFilePath)
			require.Nil(t, err)

			actualFailureFilePath := actualFailureTestDataPath + actualFailureDirEntries[dirEntryIndex].Name() + "/" + actualTestFailuresDirEntries[expectedFailureFileIndex].Name()
			actualFailureFileBytes, err := os.ReadFile(actualFailureFilePath)
			require.Nil(t, err)

			// read expected and actual text files as bytes and compare them all at once
			require.Equal(t, expectedFailureFileBytes, actualFailureFileBytes)
		}
	}
}
