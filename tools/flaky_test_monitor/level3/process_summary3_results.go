package main

import (
	"encoding/json"
	"flaky-test-monitor/common"
	"os"
	"sort"
)

// processSummary3TestRun processes a level 2 summary and produces level 3 summary which summarizes:
// most failed tests, tests with no-results, longest running tests.
func processSummary3TestRun(level2FilePath string) common.TestSummary3 {
	var testSummary2 common.TestSummary2

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

	var noResultsTRS []common.TestResultSummary = []common.TestResultSummary{}
	var failuresTRS []common.TestResultSummary = []common.TestResultSummary{}
	var durationTRS []common.TestResultSummary = []common.TestResultSummary{}

	// go through all level 2 test results to figure out grouping for tests with
	// most failures, no-results, longest running
	for _, trs := range testSummary2.TestResults {
		if trs.NoResult > 0 {
			noResultsTRS = insertSortedByNoResult(noResultsTRS, *trs)
		}
		if trs.Failed > 0 {
			failuresTRS = insertSortedByFailureRate(failuresTRS, *trs)
		}
		if trs.AverageDuration > 0 {
			durationTRS = insertSortedByDuration(durationTRS, *trs)
		}
	}

	var testSummary3 common.TestSummary3
	testSummary3.NoResults = noResultsTRS
	testSummary3.MostFailures = failuresTRS
	testSummary3.LongestRunning = durationTRS
	return testSummary3
}

func insertSortedByNoResult(testResultSummaries []common.TestResultSummary, testResultSummaryToInsert common.TestResultSummary) []common.TestResultSummary {
	indexToInsertAt := sort.Search(len(testResultSummaries), func(i int) bool {
		return float32(testResultSummaries[i].NoResult) < float32(testResultSummaryToInsert.NoResult)
	})
	return insertAt(testResultSummaries, indexToInsertAt, testResultSummaryToInsert)
}

func insertSortedByDuration(testResultSummaries []common.TestResultSummary, testResultSummaryToInsert common.TestResultSummary) []common.TestResultSummary {
	indexToInsertAt := sort.Search(len(testResultSummaries), func(i int) bool {
		return testResultSummaries[i].AverageDuration < testResultSummaryToInsert.AverageDuration
	})
	return insertAt(testResultSummaries, indexToInsertAt, testResultSummaryToInsert)
}

// from https://stackoverflow.com/a/67216833/5719544
func insertSortedByFailureRate(testResultSummaries []common.TestResultSummary, testResultSummaryToInsert common.TestResultSummary) []common.TestResultSummary {
	indexToInsertAt := sort.Search(len(testResultSummaries), func(i int) bool {
		return testResultSummaries[i].FailureRate < testResultSummaryToInsert.FailureRate
	})
	return insertAt(testResultSummaries, indexToInsertAt, testResultSummaryToInsert)
}

// insertAt inserts v into s at index i and returns the new slice.
// from https://stackoverflow.com/a/67216833/5719544
func insertAt(data []common.TestResultSummary, i int, v common.TestResultSummary) []common.TestResultSummary {
	if i == len(data) {
		// Insert at end is the easy case.
		return append(data, v)
	}

	// Make space for the inserted element by shifting
	// values at the insertion index up one index. The call
	// to append does not allocate memory when cap(data) is
	// greater ​than len(data).
	data = append(data[:i+1], data[i:]...)

	// Insert the new element.
	data[i] = v

	// Return the updated slice.
	return data
}

func main() {
	// need to pass in single argument of where level 1 summary files exist
	if len(os.Args[1:]) != 1 {
		panic("wrong number of arguments - expected single argument with name of level 2 file")
	}

	testSummary3 := processSummary3TestRun(os.Args[1])
	common.SaveToFile("level3-summary.json", testSummary3)
}
