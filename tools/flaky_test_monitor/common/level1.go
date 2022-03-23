package common

import (
	"time"

	"github.com/onflow/flow-go/utils/unittest"
)

type SkippedTestEntry struct {
	Test       string              `json:"test"`
	SkipReason unittest.SkipReason `json:"skip_reason,omitempty"`
	Package    string              `json:"package"`
	CommitDate time.Time           `json:"commit_date"`
	CommitSHA  string              `json:"commit_sha"`
	Category   string              `json:"category"`
}

// RawTestStep models single line from "go test -json" output.
type RawTestStep struct {
	Time    time.Time `json:"Time"`
	Action  string    `json:"Action"`
	Package string    `json:"Package"`
	Test    string    `json:"Test"`
	Output  string    `json:"Output"`
	Elapsed float64   `json:"Elapsed"`
}

// Level1TestResult models result of a single test
type Level1TestResult struct {
	// data that spans multiple tests - it's added at the test level because it will be used
	// by BigQuery tables and will need to be flattened
	CommitSha  string    `json:"commit_sha"`
	CommitDate time.Time `json:"commit_date"`
	JobRunDate time.Time `json:"job_run_date"`
	RunID      string    `json:"run_id"`

	// test specific data
	Test      string   `json:"test"`
	Package   string   `json:"package"`
	Output    []string `json:"output"`
	Elapsed   float64  `json:"elapsed"`
	Pass      int      `json:"pass"`
	Exception int      `json:"exception"`

	Action string `json:"-"`
}

type Level1Summary []Level1TestResult
