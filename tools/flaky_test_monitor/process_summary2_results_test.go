package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProcessSummary2TestRun(t *testing.T) {
	testDataMap := map[string]string{
		"1 level 1 summary, 1 failure the rest pass": "test1-1package-1failure",
	}

	for k, testDir := range testDataMap {
		t.Run(k, func(t *testing.T) {
			runProcessSummary2TestRun(t, testDir)
		})
	}
}

// HELPERS - UTILITIES

func runProcessSummary2TestRun(t *testing.T, testDir string) {
	//const inputTestDataPath = "./testdata/summary2/test1-1package-1failure/input/"
	inputTestDataPath := "./testdata/summary2/" + testDir + "/input/"
	expectedOutputTestDataPath := "./testdata/summary2/" + testDir + "/expected-output/" + testDir + ".json"
	expectedFailureMessagesPath := "./testdata/summary2/" + testDir + "/expected-output/" + "failures/"

	actualTestSummary2 := processSummary2TestRun(inputTestDataPath)

	//read in expected summary level 2

	var expectedTestSummary2 TestSummary2
	// read in expected JSON from file
	expectedTestSummary2JsonBytes, err := ioutil.ReadFile(expectedOutputTestDataPath)
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

		require.Equal(t, expectedTestSummary.AverageDuration, actualTestSummary.AverageDuration)
		require.Equal(t, expectedTestSummary.Failed, actualTestSummary.Failed)
		require.Equal(t, expectedTestSummary.FlakyRate, actualTestSummary.FlakyRate)
		require.Equal(t, expectedTestSummary.NoResult, actualTestSummary.NoResult)
		require.Equal(t, expectedTestSummary.Package, actualTestSummary.Package)
		require.Equal(t, expectedTestSummary.Test, actualTestSummary.Test)
	}

	//make sure calculated summary level 2 is what we expected
	require.Equal(t, expectedTestSummary2, actualTestSummary2)

	//check failure messages created
	for _, actualTestSummary := range actualTestSummary2.TestResults {
		//if test has failures, there should be directory of failure messages text files
		if actualTestSummary.Failed > 0 {
			//for every test that has failures, there should be 1 text file per failure
			//a sub-directory of the test name will hold all test failure messages
			failureEntries, err := os.ReadDir(expectedFailureMessagesPath + actualTestSummary.Test)
			require.Nil(t, err)
			require.Equal(t, actualTestSummary.Failed, len(failureEntries)+1)
		}
	}
}
