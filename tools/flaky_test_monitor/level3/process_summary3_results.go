package main

import (
	"flaky-test-monitor/common"
	"os"
)

// processSummary3TestRun processes a level 2 summary and produces level 3 summary which summarizes:
// most failed tests, tests with no-results, longest running tests.
func processSummary3TestRun(level2FilePath string) common.TestSummary3 {
	var testSummary3 common.TestSummary3
	return testSummary3
}

func main() {
	// need to pass in single argument of where level 1 summary files exist
	if len(os.Args[1:]) != 1 {
		panic("wrong number of arguments - expected single argument with name of level 2 file")
	}

	testSummary3 := processSummary3TestRun(os.Args[1])
	common.SaveToFile("level3-summary.json", testSummary3)
}
