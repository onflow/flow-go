package main

import (
	"encoding/json"
	"os"
	"sort"

	"github.com/onflow/flow-go/tools/flaky_test_monitor/common"
)

// processSummary3TestRun processes a level 2 summary and produces level 3 summary which summarizes:
// most failed tests, tests with no-results, longest running tests.
func processSummary3TestRun(level2FilePath string, propertyFileDirectory string) common.TestSummary3 {

	config := common.ReadProperties(propertyFileDirectory)

	var testSummary2 common.TestsLevel2Summary

	level2JsonBytes, err := os.ReadFile(level2FilePath)
	common.AssertNoError(err, "error reading level 2 json")

	err = json.Unmarshal(level2JsonBytes, &testSummary2)
	common.AssertNoError(err, "error unmarshalling level 2 test run")

	// there should be at least 1 level 2 test result in the supplied file
	// if the json format is different in the supplied file, there won't be a marshalling error thrown
	// this is an indirect way to tell if the json format was wrong (i.e. not a level 2 json format)
	if len(testSummary2.TestResults) == 0 {
		panic("invalid summary 2 file - no test results found")
	}

	// create lists to keep track of 3 main things
	// 1. tests with no-results (ordered by most no-results)
	// 2. tests with failures (ordered by most failures)
	// 3. tests with durations > 0 (ordered by longest durations)

	noResultsTRS := []common.TestRunsLevel2Summary{}
	failuresTRS := []common.TestRunsLevel2Summary{}
	durationTRS := []common.TestRunsLevel2Summary{}

	// go through all level 2 test results to figure out grouping for tests with
	// most failures, no-results, longest running
	for _, trs := range testSummary2.TestResults {
		if trs.NoResult > 0 {
			noResultsTRS = append(noResultsTRS, *trs)
		}
		if trs.Failed > 0 && trs.FailureRate >= config.FailureThresholdPercent {
			failuresTRS = append(failuresTRS, *trs)
		}
		if trs.AverageDuration > 0 && trs.AverageDuration >= config.DurationThresholdSeconds {
			durationTRS = append(durationTRS, *trs)
		}
	}

	// sort no result slice from most no results to least - that's why less function compares in reverse order
	sort.Slice(noResultsTRS, func(i, j int) bool {
		return (noResultsTRS[i].NoResult > noResultsTRS[j].NoResult)
	})

	// sort failures slice from most failures to least - that's why less function compares in reverse order
	sort.Slice(failuresTRS, func(i, j int) bool {
		return failuresTRS[i].FailureRate > failuresTRS[j].FailureRate
	})

	// sort duration slice from longest duration to shortest - that's why less function compares in reverse order
	sort.Slice(durationTRS, func(i, j int) bool {
		return durationTRS[i].AverageDuration > durationTRS[j].AverageDuration
	})

	var testSummary3 common.TestSummary3
	testSummary3.NoResults = noResultsTRS

	// total # of failed tests that satisfy min failure threshold
	testSummary3.MostFailuresTotal = len(failuresTRS)

	// check if # of failures exceeded max failures to return
	if len(failuresTRS) > config.FailuresSliceMax {
		// truncate slice to return only the first config.FailuresSliceMax failures
		failuresTRS = failuresTRS[:config.FailuresSliceMax]
	}
	testSummary3.MostFailures = failuresTRS

	// total # of long tests that satisfy min duration threshold
	testSummary3.LongestRunningTotal = len(durationTRS)

	// check if # of durations exceeded max durations to return
	if len(durationTRS) > config.DurationSliceMax {
		// truncate slice to return only the first config.DurationSliceMax durations
		durationTRS = durationTRS[:config.DurationSliceMax]
	}
	testSummary3.LongestRunning = durationTRS

	return testSummary3
}

func main() {
	// need to pass in single argument of where level 1 summary files exist
	if len(os.Args[1:]) != 2 {
		panic("wrong number of arguments - expected arguments 1) path of level 2 file 2) directory of property file")
	}

	testSummary3 := processSummary3TestRun(os.Args[1], os.Args[2])
	common.SaveToFile("level3-summary.json", testSummary3)
}
