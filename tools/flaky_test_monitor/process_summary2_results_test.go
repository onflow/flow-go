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
	require.Equal(t, len(expectedTestSummary2.Tests), len(actualTestSummary2.Tests))

	for i := 0; i < len(expectedTestSummary2.Tests); i++ {
		require.Equal(t, expectedTestSummary2.Tests[i].AverageDuration, actualTestSummary2.Tests[i].AverageDuration)
		require.Equal(t, expectedTestSummary2.Tests[i].Failed, actualTestSummary2.Tests[i].Failed)
		require.Equal(t, expectedTestSummary2.Tests[i].FlakyRate, actualTestSummary2.Tests[i].FlakyRate)
		require.Equal(t, expectedTestSummary2.Tests[i].NoResult, actualTestSummary2.Tests[i].NoResult)
		require.Equal(t, expectedTestSummary2.Tests[i].Package, actualTestSummary2.Tests[i].Package)
		require.Equal(t, expectedTestSummary2.Tests[i].Test, actualTestSummary2.Tests[i].Test)
	}

	//make sure calculated summary level 2 is what we expected
	require.Equal(t, expectedTestSummary2, actualTestSummary2)

	//check failure messages created
	for _, testSummary := range actualTestSummary2.Tests {
		//if test has failures, there should be directory of failure messages text files
		if testSummary.Failed > 0 {
			//for every test that has failures, there should be 1 text file per failure
			//a sub-directory of the test name will hold all test failure messages
			failureEntries, err := os.ReadDir(expectedFailureMessagesPath + testSummary.Test)
			require.Nil(t, err)
			require.Equal(t, testSummary.Failed, len(failureEntries)+1)
		}
	}

	dirEntries, err := os.ReadDir(inputTestDataPath)
	require.Nil(t, err)

	var level1TestRuns []TestRun

	// go through all level 1 summaries in a folder to create level 2 summary
	for i := 0; i < len(dirEntries); i++ {
		t.Log("dirEntry:" + dirEntries[i].Name())

		// read in each level 1 summary
		var level1TestRun TestRun

		level1JsonBytes, err := ioutil.ReadFile(inputTestDataPath + dirEntries[i].Name())
		require.Nil(t, err)
		require.NotEmpty(t, level1JsonBytes)

		err = json.Unmarshal(level1JsonBytes, &level1TestRun)
		require.Nil(t, err)

		//keep building list of level 1 test runs that can go through and create level 2 summary
		level1TestRuns = append(level1TestRuns, level1TestRun)
	}

	// go through all level 1 summaries to create level 2 summary
	// for j := 0; j < len(level1TestRuns); j++ {
	// 	testRun := level1TestRuns[j]
	// }

	// var expectedTestRun TestRun
	// // read in expected JSON from file
	// expectedJsonBytes, err := ioutil.ReadFile(expectedJsonFilePath + jsonExpectedActualFile)
	// require.Nil(t, err)
	// require.NotEmpty(t, expectedJsonBytes)

	// err = json.Unmarshal(expectedJsonBytes, &expectedTestRun)
	// require.Nil(t, err)

	// // convert to UTC to remove any local time zone settings -
	// // even though the time stamp in the test json can be in UTC (or not), there will still be a local time zone set that will fail equality check - this removes the timezone setting
	// expectedTestRun.CommitDate = expectedTestRun.CommitDate.UTC()
	// expectedTestRun.JobRunDate = expectedTestRun.JobRunDate.UTC()

	// // sort all package results alphabetically
	// sort.SliceStable(expectedTestRun.PackageResults, func(i, j int) bool {
	// 	return expectedTestRun.PackageResults[i].Package < expectedTestRun.PackageResults[j].Package
	// })

	// // sort all tests alphabetically within each package - otherwise, equality check will fail
	// for k := range expectedTestRun.PackageResults {
	// 	sort.Slice(expectedTestRun.PackageResults[k].Tests, func(i, j int) bool {
	// 		return expectedTestRun.PackageResults[k].Tests[i].Test < expectedTestRun.PackageResults[k].Tests[j].Test
	// 	})

	// 	// init TestMap to empty - otherwise get comparison failure because would be nil
	// 	expectedTestRun.PackageResults[k].TestMap = make(map[string][]TestResult)
	// }

	// // these hard coded values simulate a real test run that would obtain these environment variables dynamically
	// // we are simulating this scenario by setting the environment variables explicitly in the test before calling the main processing script which will look for them
	// // these values are uses in testdata/expected/*.json files
	// require.NoError(t, os.Setenv("COMMIT_DATE", "2021-09-21T18:06:25-07:00"))
	// require.NoError(t, os.Setenv("COMMIT_SHA", "46baf6c6be29af9c040bc14195e195848598bbae"))
	// require.NoError(t, os.Setenv("JOB_STARTED", "2021-09-21T21:06:25-07:00"))

	// // simulate generating raw "go test -json" output by loading output from saved file
	// resultReader := FileResultReader{
	// 	rawJsonFile: rawJsonFilePath + jsonExpectedActualFile,
	// }
	// actualTestRun := processSummary1TestRun(&resultReader)

	// checkTestRuns(t, expectedTestRun, actualTestRun)
}
