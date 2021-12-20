package common

import (
	"time"
)

// RawTestStep models single line from "go test -json" output.
type RawTestStep struct {
	Time    time.Time `json:"Time"`
	Action  string    `json:"Action"`
	Package string    `json:"Package"`
	Test    string    `json:"Test"`
	Output  string    `json:"Output"`
	Elapsed float32   `json:"Elapsed"`
}

// TestRun models full level 1 summary of a test run from "go test -json".
type TestRun struct {
	CommitSha      string          `json:"commit_sha"`
	CommitDate     time.Time       `json:"commit_date"`
	JobRunDate     time.Time       `json:"job_run_date"`
	PackageResults []PackageResult `json:"results"`
}

// PackageResult models test result of an entire package which can have multiple tests
type PackageResult struct {
	Package string                  `json:"package"`
	Result  string                  `json:"result"`
	Elapsed float32                 `json:"elapsed"`
	Output  []string                `json:"output"`
	Tests   []TestResult            `json:"tests"`
	TestMap map[string][]TestResult `json:"-"`
}

// TestResult models result of a single test that's part of a larger package result
type TestResult struct {
	Test    string   `json:"test"`
	Package string   `json:"package"`
	Output  []string `json:"output"`
	Result  string   `json:"result"`
	Elapsed float32  `json:"elapsed"`
}
