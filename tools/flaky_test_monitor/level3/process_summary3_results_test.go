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
		"many level 1 summaries, many failures, many passes, many no-result tests": "test5-multi-failures-multi-no-result-tests",
	}

	for k, testDir := range testDataMap {
		t.Run(k, func(t *testing.T) {
			runProcessSummary3TestRun(t, testDir)
		})
	}

}

func runProcessSummary3TestRun(t *testing.T, testDir string) {
	testDataBaseDir := "../testdata/summary3/" + testDir
	inputTestDataPath := testDataBaseDir + "/input/"
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

	//check all details of test summary level 2 between expected and actual

	require.Equal(t, expectedTestSummary3, actualTestSummary3)
}

// test that error is thrown when there are > 1 level 2 summary files in a folder
func TestProcessSummary3TestRun_Error_Multiple_Level2_Files(t *testing.T) {
	t.FailNow()
}
