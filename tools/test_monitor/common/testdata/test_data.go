package testdata

import (
	"time"

	"github.com/onflow/flow-go/tools/test_monitor/common"
)

const COMMIT_DATE = "2021-09-21T18:06:25-07:00"
const COMMIT_SHA = "46baf6c6be29af9c040bc14195e195848598bbae"
const JOB_STARTED = "2021-09-21T21:06:25-07:00"
const CRYPTO_HASH_PACKAGE = "github.com/onflow/crypto/hash"
const RUN_ID = "12345"

// Level1TestData is used by tests to store what the expected test result should be and what the raw
// JSON input file is
type Level1TestData struct {
	ExpectedLevel1Summary common.Level1Summary
	RawJSONTestRunFile    string
}

type Level2TestData struct {
	Directory       string
	Level1DataPath  string
	HasFailures     bool
	HasExceptions   bool
	Level1Summaries []common.Level1Summary
}

// ************** Helper Functions *****************
// following functions are used to construct expected TestRun data

func getCommitDate() time.Time {
	commitDate, err := time.Parse(time.RFC3339, COMMIT_DATE)
	common.AssertNoError(err, "time parse - commit date")
	return commitDate
}

func getJobRunDate() time.Time {
	jobRunDate, err := time.Parse(time.RFC3339, JOB_STARTED)
	common.AssertNoError(err, "time parse - job run date")
	return jobRunDate
}

func getPassedTest(name string) common.Level1TestResult {
	return getPassedTestPackage(name, CRYPTO_HASH_PACKAGE)
}

func getPassedTestPackage(name string, packageName string) common.Level1TestResult {
	return common.Level1TestResult{
		CommitSha:  COMMIT_SHA,
		RunID:      RUN_ID,
		CommitDate: getCommitDate(),
		JobRunDate: getJobRunDate(),
		Test:       name,
		Package:    packageName,
		Pass:       1,
		Exception:  0,
		Elapsed:    0,
	}
}

func getPassedTestElapsed(name string, elapsed float64, elapsedStr string) common.Level1TestResult {
	return common.Level1TestResult{
		CommitSha:  COMMIT_SHA,
		RunID:      RUN_ID,
		CommitDate: getCommitDate(),
		JobRunDate: getJobRunDate(),
		Test:       name,
		Package:    CRYPTO_HASH_PACKAGE,
		Pass:       1,
		Exception:  0,
		Elapsed:    elapsed,
	}
}

func getPassedTestPackageElapsedOutput(name string, packageName string, elapsed float64, elapsedStr string, output string) common.Level1TestResult {
	row := getPassedTestElapsedOutput(name, elapsed, elapsedStr, output)
	row.Package = packageName
	return row
}

func getPassedTestElapsedOutput(name string, elapsed float64, elapsedStr string, output string) common.Level1TestResult {
	return common.Level1TestResult{
		CommitSha:  COMMIT_SHA,
		RunID:      RUN_ID,
		CommitDate: getCommitDate(),
		JobRunDate: getJobRunDate(),
		Test:       name,
		Package:    CRYPTO_HASH_PACKAGE,
		Pass:       1,
		Exception:  0,
		Elapsed:    elapsed,
	}
}

func getFailedTest_TestSanitySha2_256() common.Level1TestResult {
	return common.Level1TestResult{
		CommitSha:  COMMIT_SHA,
		RunID:      RUN_ID,
		CommitDate: getCommitDate(),
		JobRunDate: getJobRunDate(),
		Test:       "TestSanitySha2_256",
		Package:    CRYPTO_HASH_PACKAGE,
		Pass:       0,
		Exception:  0,
		Elapsed:    0,
		Output: []string{
			"=== RUN   TestSanitySha2_256\n",
			"    hash_test.go:41: \n",
			"        \tError Trace:\thash_test.go:41\n",
			"        \tError:      \tNot equal: \n",
			"        \t            \texpected: hash.Hash{0x9f, 0x86, 0xd0, 0x81, 0x88, 0x4c, 0x7d, 0x65, 0x9a, 0x2f, 0xea, 0xa0, 0xc5, 0x5a, 0xd0, 0x15, 0xa3, 0xbf, 0x4f, 0x1b, 0x2b, 0xb, 0x82, 0x2c, 0xd1, 0x5d, 0x6c, 0x15, 0xb0, 0xf0, 0xa, 0x9}\n",
			"        \t            \tactual  : hash.Hash{0x9f, 0x86, 0xd0, 0x81, 0x88, 0x4c, 0x7d, 0x65, 0x9a, 0x2f, 0xea, 0xa0, 0xc5, 0x5a, 0xd0, 0x15, 0xa3, 0xbf, 0x4f, 0x1b, 0x2b, 0xb, 0x82, 0x2c, 0xd1, 0x5d, 0x6c, 0x15, 0xb0, 0xf0, 0xa, 0x8}\n",
			"        \t            \t\n",
			"        \t            \tDiff:\n",
			"        \t            \t--- Expected\n",
			"        \t            \t+++ Actual\n",
			"        \t            \t@@ -1,2 +1,2 @@\n",
			"        \t            \t-(hash.Hash) (len=32) 0x9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a09\n",
			"        \t            \t+(hash.Hash) (len=32) 0x9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08\n",
			"        \t            \t \n",
			"        \tTest:       \tTestSanitySha2_256\n",
			"--- FAIL: TestSanitySha2_256 (0.00s)\n",
		},
	}
}

func getFailedTest_TestSanitySha3_256() common.Level1TestResult {
	return common.Level1TestResult{
		CommitSha:  COMMIT_SHA,
		RunID:      RUN_ID,
		CommitDate: getCommitDate(),
		JobRunDate: getJobRunDate(),
		Test:       "TestSanitySha3_256",
		Package:    CRYPTO_HASH_PACKAGE,
		Pass:       0,
		Exception:  0,
		Elapsed:    0,
		Output: []string{
			"=== RUN   TestSanitySha3_256\n",
			"    hash_test.go:21: \n",
			"        \tError Trace:\thash_test.go:21\n",
			"        \tError:      \tNot equal: \n",
			"        \t            \texpected: hash.Hash{0x36, 0xf0, 0x28, 0x58, 0xb, 0xb0, 0x2c, 0xc8, 0x27, 0x2a, 0x9a, 0x2, 0xf, 0x42, 0x0, 0xe3, 0x46, 0xe2, 0x76, 0xae, 0x66, 0x4e, 0x45, 0xee, 0x80, 0x74, 0x55, 0x74, 0xe2, 0xf5, 0xab, 0x81}\n",
			"        \t            \tactual  : hash.Hash{0x36, 0xf0, 0x28, 0x58, 0xb, 0xb0, 0x2c, 0xc8, 0x27, 0x2a, 0x9a, 0x2, 0xf, 0x42, 0x0, 0xe3, 0x46, 0xe2, 0x76, 0xae, 0x66, 0x4e, 0x45, 0xee, 0x80, 0x74, 0x55, 0x74, 0xe2, 0xf5, 0xab, 0x80}\n",
			"        \t            \t\n",
			"        \t            \tDiff:\n",
			"        \t            \t--- Expected\n",
			"        \t            \t+++ Actual\n",
			"        \t            \t@@ -1,2 +1,2 @@\n",
			"        \t            \t-(hash.Hash) (len=32) 0x36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab81\n",
			"        \t            \t+(hash.Hash) (len=32) 0x36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80\n",
			"        \t            \t \n",
			"        \tTest:       \tTestSanitySha3_256\n",
			"--- FAIL: TestSanitySha3_256 (0.00s)\n",
		},
	}
}

func getExceptionTest_TestEncodableRandomBeaconPrivKeyMsgPack() common.Level1TestResult {
	return common.Level1TestResult{
		CommitSha:  COMMIT_SHA,
		RunID:      RUN_ID,
		CommitDate: getCommitDate(),
		JobRunDate: getJobRunDate(),
		Test:       "TestEncodableRandomBeaconPrivKeyMsgPack",
		Package:    "github.com/onflow/flow-go/model/encodable",
		Pass:       0,
		Exception:  1,
		Elapsed:    0,
		Output: []string{
			"=== RUN   TestEncodableRandomBeaconPrivKeyMsgPack\n",
			"bytes: 194--- PASS: TestEncodableRandomBeaconPrivKeyMsgPack (0.00s)\n",
		},
	}
}

// **********************************************************
// ************** Level 1 Summaries Testing *****************
// **********************************************************

// The following GetTestData_Level1_*() functions are used by level 1 unit tests for constructing expected level 1 summaries.
// These expected level 1 summaries will be compared with the generated level 1 summaries created by the level 1 parser.
// Instead of having to represent these level 1 summaries as JSON files, they are represented as structs
// to make them easier to maintain.

// GetTestData_Level1_1CountSingleExceptionTest represents a level 1 summary (as exptected output from level 1 parser)
// with a single no result test and no other tests, count=1.
func GetTestData_Level1_1CountSingleExceptionTest() common.Level1Summary {
	return []common.Level1TestResult{
		getExceptionTest_TestEncodableRandomBeaconPrivKeyMsgPack(),
	}
}

// GetTestData_Level1_1CountPass represents a level 1 summary (as exptected output from level 1 parser)
// with multiple passed tests, count=1.
func GetTestData_Level1_1CountPass() common.Level1Summary {
	return []common.Level1TestResult{
		getPassedTest("TestSanitySha3_256"),
		getPassedTest("TestSanitySha2_256"),
		getPassedTest("TestSanitySha3_384"),
		getPassedTest("TestSanitySha2_384"),
		getPassedTest("TestSanityKmac128"),
		getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632497249121800000\n"),
		getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1632497249122032000\n"),
		getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"),
		getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"),
	}
}

// GetTestData_Level1_1Count1FailRestPass represents a level 1 summary (as exptected output from level 1 parser)
// with multiple passed tests and a single failed test, count=1.
func GetTestData_Level1_1Count1FailRestPass() common.Level1Summary {
	return []common.Level1TestResult{
		getFailedTest_TestSanitySha3_256(),
		getPassedTest("TestSanitySha3_384"),
		getPassedTest("TestSanitySha2_256"),
		getPassedTest("TestSanitySha2_384"),
		getPassedTest("TestSanityKmac128"),
		getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632498687765218000\n"),
		getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1632498687765661000\n"),
		getPassedTestElapsed("TestSha3/SHA3_256", 0.11, "0.11"),
		getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"),
	}
}

// GetTestData_Level1_1CountAllPass represents a level 1 summary (as expected output from level 1 parser)
// with multiple passed tests, count=1.
func GetTestData_Level1_1CountAllPass() common.Level1Summary {
	return []common.Level1TestResult{
		getPassedTest("TestSanitySha3_256"),
		getPassedTest("TestSanitySha3_384"),
		getPassedTest("TestSanitySha2_384"),
		getPassedTest("TestSanityKmac128"),
		getPassedTestElapsedOutput("TestSha3", 0.24, "0.24", "    hash_test.go:160: math rand seed is 1633518697589650000\n"),
		getPassedTestElapsed("TestSha3/SHA3_256", 0.11, "0.11"),
		getPassedTestElapsed("TestSha3/SHA3_384", 0.13, "0.13"),
	}
}

// GetTestData_Level1_2CountPass represents a level 1 summary (as exptected output from level 1 parser)
// with multiple passed tests, count=2.
func GetTestData_Level1_2CountPass() common.Level1Summary {
	var level1TestResults []common.Level1TestResult
	for i := 0; i < 2; i++ {
		level1TestResults = append(level1TestResults, getPassedTest("TestSanitySha3_256"))
		level1TestResults = append(level1TestResults, getPassedTest("TestSanitySha2_256"))
		level1TestResults = append(level1TestResults, getPassedTest("TestSanitySha3_384"))
		level1TestResults = append(level1TestResults, getPassedTest("TestSanitySha2_384"))
		level1TestResults = append(level1TestResults, getPassedTest("TestSanityKmac128"))
		level1TestResults = append(level1TestResults, getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"))
		level1TestResults = append(level1TestResults, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))
	}

	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1633358050203144000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1633358050430256000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1633358050203374000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1633358050430467000\n"))

	return level1TestResults
}

// GetTestData_Level1_10CountPass represents a level 1 summary (as exptected output from level 1 parser)
// with multiple passed tests, count=10.
func GetTestData_Level1_10CountPass() common.Level1Summary {
	var level1TestResults []common.Level1TestResult

	for i := 0; i < 10; i++ {
		level1TestResults = append(level1TestResults, getPassedTest("TestSanitySha3_256"))
		level1TestResults = append(level1TestResults, getPassedTest("TestSanitySha2_256"))
		level1TestResults = append(level1TestResults, getPassedTest("TestSanitySha3_384"))
		level1TestResults = append(level1TestResults, getPassedTest("TestSanitySha2_384"))
		level1TestResults = append(level1TestResults, getPassedTest("TestSanityKmac128"))
	}

	for i := 0; i < 9; i++ {
		level1TestResults = append(level1TestResults, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))
		level1TestResults = append(level1TestResults, getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"))
	}
	level1TestResults = append(level1TestResults, getPassedTestElapsed("TestSha3/SHA3_384", 0.13, "0.13"))
	level1TestResults = append(level1TestResults, getPassedTestElapsed("TestSha3/SHA3_256", 0.12, "0.12"))

	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739552470379000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739552696815000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739552917474000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739553140451000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739553362249000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739553605325000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739553826502000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739554054239000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739554280043000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739554500707000\n"))

	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1632739552470723000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739552697024000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739552917708000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739553140702000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.24, "0.24", "    hash_test.go:158: math rand seed is 1632739553362497000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739553605582000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1632739553826733000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1632739554054464000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739554280256000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739554500935000\n"))

	return level1TestResults
}

// GetTestData_Level1_10CountSomeFailures represents a level 1 summary (as exptected output from level 1 parser)
// with multiple passed tests and a single failed test, count=10.
func GetTestData_Level1_10CountSomeFailures() common.Level1Summary {
	var level1TestResults []common.Level1TestResult

	for i := 0; i < 10; i++ {
		level1TestResults = append(level1TestResults, getPassedTest("TestSanitySha3_256"))
		level1TestResults = append(level1TestResults, getFailedTest_TestSanitySha2_256())
		level1TestResults = append(level1TestResults, getPassedTest("TestSanitySha3_384"))
		level1TestResults = append(level1TestResults, getPassedTest("TestSanitySha2_384"))
		level1TestResults = append(level1TestResults, getPassedTest("TestSanityKmac128"))
		level1TestResults = append(level1TestResults, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))
	}

	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739682184421000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739682415309000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739682637108000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739682857435000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739683077064000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739683297507000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739683518492000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739683740724000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739683980033000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739684200452000\n"))

	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1632739682184858000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739682415616000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739682637311000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739682857668000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739683077268000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739683297711000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739683518781000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.24, "0.24", "    hash_test.go:158: math rand seed is 1632739683740970000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739683980266000\n"))
	level1TestResults = append(level1TestResults, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739684200658000\n"))

	for i := 0; i < 8; i++ {
		level1TestResults = append(level1TestResults, getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"))
	}

	level1TestResults = append(level1TestResults, getPassedTestElapsed("TestSha3/SHA3_256", 0.11, "0.11"))
	level1TestResults = append(level1TestResults, getPassedTestElapsed("TestSha3/SHA3_256", 0.12, "0.12"))

	return level1TestResults
}

// GetTestData_Level1_5CountSingleExceptionTest represents a level 1 summary (as exptected output from level 1 parser)
// with single no result test, count=5.
func GetTestData_Level1_5CountSingleExceptionTest() common.Level1Summary {
	var level1TestResults []common.Level1TestResult

	for i := 0; i < 5; i++ {
		level1TestResults = append(level1TestResults, getExceptionTest_TestEncodableRandomBeaconPrivKeyMsgPack())
	}

	return level1TestResults
}

// GetTestData_Level1_5CountMultipleExceptionTests represents a level 1 summary (as expected output from level 1 parser)
// with single no result test and a passed test, count=5.
func GetTestData_Level1_5CountMultipleExceptionTests() common.Level1Summary {
	var level1TestResults []common.Level1TestResult
	for i := 0; i < 4; i++ {
		level1TestResults = append(level1TestResults, getExceptionTest_TestEncodableRandomBeaconPrivKeyMsgPack())
	}
	level1TestResults = append(level1TestResults, getPassedTestPackageElapsedOutput("TestEncodableRandomBeaconPrivKeyMsgPack", "github.com/onflow/flow-go/model/encodable", 0, "0.00", "    keys_test.go:245: bytes: 194\n"))

	return level1TestResults
}

// GetTestData_Leve1_3CountExceptionWithNormalTests represents a level 1 summary (as exptected output from level 1 parser)
// with single no result test and many passed tests, count=3.
func GetTestData_Leve1_3CountExceptionWithNormalTests() common.Level1Summary {
	var level1TestResults []common.Level1TestResult

	for i := 0; i < 3; i++ {
		level1TestResults = append(level1TestResults, getPassedTestPackage("TestEncodableNetworkPrivKey", "github.com/onflow/flow-go/model/encodable"))
		level1TestResults = append(level1TestResults, getPassedTestPackage("TestEncodableNetworkPrivKeyNil", "github.com/onflow/flow-go/model/encodable"))
		level1TestResults = append(level1TestResults, getPassedTestPackage("TestEncodableNetworkPubKey", "github.com/onflow/flow-go/model/encodable"))
		level1TestResults = append(level1TestResults, getPassedTestPackage("TestEncodableNetworkPubKeyNil", "github.com/onflow/flow-go/model/encodable"))
		level1TestResults = append(level1TestResults, getPassedTestPackage("TestEncodableRandomBeaconPrivKey", "github.com/onflow/flow-go/model/encodable"))
		level1TestResults = append(level1TestResults, getPassedTestPackage("TestEncodableRandomBeaconPrivKeyNil", "github.com/onflow/flow-go/model/encodable"))
		level1TestResults = append(level1TestResults, getPassedTestPackage("TestEncodableRandomBeaconPubKey", "github.com/onflow/flow-go/model/encodable"))
		level1TestResults = append(level1TestResults, getPassedTestPackage("TestEncodableRandomBeaconPubKeyNil", "github.com/onflow/flow-go/model/encodable"))
		level1TestResults = append(level1TestResults, getPassedTestPackage("TestEncodableStakingPrivKey", "github.com/onflow/flow-go/model/encodable"))
		level1TestResults = append(level1TestResults, getPassedTestPackage("TestEncodableStakingPrivKeyNil", "github.com/onflow/flow-go/model/encodable"))
		level1TestResults = append(level1TestResults, getPassedTestPackage("TestEncodableStakingPubKey", "github.com/onflow/flow-go/model/encodable"))
		level1TestResults = append(level1TestResults, getPassedTestPackage("TestEncodableStakingPubKeyNil", "github.com/onflow/flow-go/model/encodable"))
		level1TestResults = append(level1TestResults, getPassedTestPackage("TestIsHexString", "github.com/onflow/flow-go/model/encodable"))
		level1TestResults = append(level1TestResults, getExceptionTest_TestEncodableRandomBeaconPrivKeyMsgPack())
	}

	return level1TestResults
}

// **********************************************************
// ************** Level 2 Summaries Testing *****************
// **********************************************************
// The following GetTestData_Level2_*() functions are used by level 2 unit tests for constructing level 1 summaries
// that are used as inputs to generate level 2 summaries.
// Instead of having to represent these level 1 summaries as JSON files, they are represented as a list
// of structs to make them easier to maintain.

// GetTestData_Level2_1FailureRestPass represents a level 1 summary (as input into a level 2 parser)
// with count=1, many passed tests and a single failed test.
func GetTestData_Level2_1FailureRestPass() []common.Level1Summary {
	var testResult1Rows []common.Level1TestResult
	testResult1Rows = append(testResult1Rows, getFailedTest_TestSanitySha3_256())
	testResult1Rows = append(testResult1Rows, getPassedTest("TestSanitySha3_384"))
	testResult1Rows = append(testResult1Rows, getPassedTest("TestSanitySha2_256"))
	testResult1Rows = append(testResult1Rows, getPassedTest("TestSanitySha2_384"))
	testResult1Rows = append(testResult1Rows, getPassedTest("TestSanityKmac128"))
	testResult1Rows = append(testResult1Rows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632498687765218000\n"))
	testResult1Rows = append(testResult1Rows, getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1632498687765661000\n"))
	testResult1Rows = append(testResult1Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.11, "0.11"))
	testResult1Rows = append(testResult1Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))

	var level1Summaries []common.Level1Summary
	level1Summaries = append(level1Summaries, testResult1Rows)
	return level1Summaries
}

// GetTestsData_Level2_1ExceptionNoOtherTests represents a level 1 summary (as input into a level 2 parser)
// with a single no result test and no other tests.
func GetTestsData_Level2_1ExceptionNoOtherTests() []common.Level1Summary {
	var testResult1Rows []common.Level1TestResult
	testResult1Rows = append(testResult1Rows, getExceptionTest_TestEncodableRandomBeaconPrivKeyMsgPack())

	var leve1Summaries []common.Level1Summary
	leve1Summaries = append(leve1Summaries, testResult1Rows)
	return leve1Summaries
}

// GetTestData_Level2_MultipleL1SummariesExceptions represents multiple level 1 summaries (as input into a level 2 parser)
// and many tests with exceptions within the level level 1 summaries.
func GetTestData_Level2_MultipleL1SummariesExceptions() []common.Level1Summary {

	// models level 1 summary with many passed tests and a single no result test, count=3
	var testResult1Rows []common.Level1TestResult
	for i := 0; i < 3; i++ {
		testResult1Rows = append(testResult1Rows, getPassedTestPackage("TestEncodableNetworkPrivKey", "github.com/onflow/flow-go/model/encodable"))
		testResult1Rows = append(testResult1Rows, getPassedTestPackage("TestEncodableNetworkPrivKeyNil", "github.com/onflow/flow-go/model/encodable"))
		testResult1Rows = append(testResult1Rows, getPassedTestPackage("TestEncodableNetworkPubKey", "github.com/onflow/flow-go/model/encodable"))
		testResult1Rows = append(testResult1Rows, getPassedTestPackage("TestEncodableNetworkPubKeyNil", "github.com/onflow/flow-go/model/encodable"))
		testResult1Rows = append(testResult1Rows, getPassedTestPackage("TestEncodableRandomBeaconPrivKey", "github.com/onflow/flow-go/model/encodable"))
		testResult1Rows = append(testResult1Rows, getPassedTestPackage("TestEncodableRandomBeaconPrivKeyNil", "github.com/onflow/flow-go/model/encodable"))
		testResult1Rows = append(testResult1Rows, getPassedTestPackage("TestEncodableRandomBeaconPubKey", "github.com/onflow/flow-go/model/encodable"))
		testResult1Rows = append(testResult1Rows, getPassedTestPackage("TestEncodableRandomBeaconPubKeyNil", "github.com/onflow/flow-go/model/encodable"))
		testResult1Rows = append(testResult1Rows, getPassedTestPackage("TestEncodableStakingPrivKey", "github.com/onflow/flow-go/model/encodable"))
		testResult1Rows = append(testResult1Rows, getPassedTestPackage("TestEncodableStakingPrivKeyNil", "github.com/onflow/flow-go/model/encodable"))
		testResult1Rows = append(testResult1Rows, getPassedTestPackage("TestEncodableStakingPubKey", "github.com/onflow/flow-go/model/encodable"))
		testResult1Rows = append(testResult1Rows, getPassedTestPackage("TestEncodableStakingPubKeyNil", "github.com/onflow/flow-go/model/encodable"))
		testResult1Rows = append(testResult1Rows, getPassedTestPackage("TestIsHexString", "github.com/onflow/flow-go/model/encodable"))
		testResult1Rows = append(testResult1Rows, getExceptionTest_TestEncodableRandomBeaconPrivKeyMsgPack())
	}

	// models a level 1 summary with a single no result test, count=1
	var testResult2Rows []common.Level1TestResult
	testResult2Rows = append(testResult2Rows, getExceptionTest_TestEncodableRandomBeaconPrivKeyMsgPack())

	// models level 1 summary with count=5 where 4 of the results are "no result" and the 5th one passed
	var testResult3Rows []common.Level1TestResult
	for i := 0; i < 4; i++ {
		testResult3Rows = append(testResult3Rows, getExceptionTest_TestEncodableRandomBeaconPrivKeyMsgPack())
	}

	// the remaining 1 test runs (out of 5) has to be added manually since it wasn't an exception
	testResult3Rows = append(testResult3Rows, getPassedTestPackageElapsedOutput("TestEncodableRandomBeaconPrivKeyMsgPack", "github.com/onflow/flow-go/model/encodable", 0, "0.00", "    keys_test.go:245: bytes: 194\n"))

	// models level 1 summary for a single no result test, count=5
	var testResult4Rows []common.Level1TestResult
	for i := 0; i < 5; i++ {
		testResult4Rows = append(testResult4Rows, getExceptionTest_TestEncodableRandomBeaconPrivKeyMsgPack())
	}

	var level1Summaries []common.Level1Summary
	level1Summaries = append(level1Summaries, testResult1Rows)
	level1Summaries = append(level1Summaries, testResult2Rows)
	level1Summaries = append(level1Summaries, testResult3Rows)
	level1Summaries = append(level1Summaries, testResult4Rows)
	return level1Summaries
}

// GetTestData_Level2MultipleL1SummariesFailuresPasses represents multiple level 1 summaries (as input into a level 2 parser)
// where there are many passed and failed tests within the level level 1 summaries.
func GetTestData_Level2MultipleL1SummariesFailuresPasses() []common.Level1Summary {
	// level 1 summary with many passed tests and 1 failed test, count=1
	var level1TestResult1Rows []common.Level1TestResult
	level1TestResult1Rows = append(level1TestResult1Rows, getFailedTest_TestSanitySha3_256())
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTest("TestSanitySha3_384"))
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTest("TestSanitySha2_256"))
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTest("TestSanitySha2_384"))
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTest("TestSanityKmac128"))
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632498687765218000\n"))
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1632498687765661000\n"))
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.11, "0.11"))
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))

	// level 1 summary with many passed tests, count=1
	var level1TestResult2Rows []common.Level1TestResult
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTest("TestSanitySha3_256"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTest("TestSanitySha2_256"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTest("TestSanitySha3_384"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTest("TestSanitySha2_384"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTest("TestSanityKmac128"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632497249121800000\n"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1632497249122032000\n"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))

	// level 1 summary with many passed tests, count=1
	var level1TestResult3Rows []common.Level1TestResult
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTest("TestSanitySha3_256"))
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTest("TestSanitySha2_256"))
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTest("TestSanitySha3_384"))
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTest("TestSanitySha2_384"))
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTest("TestSanityKmac128"))
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTestElapsedOutput("TestSha3", 0.24, "0.24", "    hash_test.go:160: math rand seed is 1633518697589650000\n"))
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.11, "0.11"))
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.13, "0.13"))

	// level 1 summary with many passed tests, count=2
	var level1TestResult4Rows []common.Level1TestResult
	for i := 0; i < 2; i++ {
		level1TestResult4Rows = append(level1TestResult4Rows, getPassedTest("TestSanitySha3_256"))
		level1TestResult4Rows = append(level1TestResult4Rows, getPassedTest("TestSanitySha2_256"))
		level1TestResult4Rows = append(level1TestResult4Rows, getPassedTest("TestSanitySha3_384"))
		level1TestResult4Rows = append(level1TestResult4Rows, getPassedTest("TestSanitySha2_384"))
		level1TestResult4Rows = append(level1TestResult4Rows, getPassedTest("TestSanityKmac128"))
		level1TestResult4Rows = append(level1TestResult4Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"))
		level1TestResult4Rows = append(level1TestResult4Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))
	}

	// the following test results have to be added manually (i.e. not in a loop) because
	// they have unique data generated in the output / duration each time they're run
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1633358050430256000\n"))
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1633358050203144000\n"))

	// the following test results have to be added manually (i.e. not in a loop) because
	// they have unique data generated in the output / duration each time they're run
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1633358050203374000\n"))
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1633358050430467000\n"))

	// level 1 summary with many passed tests, count=10
	var level1TestResult5Rows []common.Level1TestResult
	for i := 0; i < 10; i++ {
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTest("TestSanitySha3_256"))
		level1TestResult5Rows = append(level1TestResult5Rows, getFailedTest_TestSanitySha2_256())
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTest("TestSanitySha3_384"))
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTest("TestSanitySha2_384"))
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTest("TestSanityKmac128"))
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTest("TestHashersAPI"))
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))
	}

	// the following test results have to be added in a separate loop because their results are the same 8 times out 10 test runs
	for i := 0; i < 8; i++ {
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTestElapsed("TestSha3", 0.22, "0.22"))
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"))
	}

	// the remaining 2 test runs (out of 10) have to be added manually since they have unique duration data not present in the other test runs
	level1TestResult5Rows = append(level1TestResult5Rows, getPassedTestElapsed("TestSha3", 0.24, "0.24"))
	level1TestResult5Rows = append(level1TestResult5Rows, getPassedTestElapsed("TestSha3", 0.23, "0.23"))

	// the remaining 2 test runs (out of 10) have to be added manually since they have unique duration data not present in the other test runs
	level1TestResult5Rows = append(level1TestResult5Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.12, "0.12"))
	level1TestResult5Rows = append(level1TestResult5Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.11, "0.11"))

	// level 1 summary with many passed tests, count=10
	var level1TestResult6Rows []common.Level1TestResult
	for i := 0; i < 10; i++ {
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTest("TestSanitySha3_256"))
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTest("TestSanitySha2_256"))
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTest("TestSanitySha3_384"))
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTest("TestSanitySha2_384"))
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTest("TestSanityKmac128"))
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTest("TestHashersAPI"))
	}

	// the following test results have to be added in a separate loop because their results are the same 6 times out 10 test runs
	for i := 0; i < 6; i++ {
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3", 0.22, "0.22"))
	}
	// the remaining 4 test runs (out of 10) have to be added manually since they have unique duration data not present in the other test runs
	level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3", 0.23, "0.23"))
	level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3", 0.24, "0.24"))
	level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3", 0.23, "0.23"))
	level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3", 0.23, "0.23"))

	// the following test results have to be added in a separate loop because their results are the same 9 times out 10 test runs
	for i := 0; i < 9; i++ {
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"))
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))
	}

	// the remaining 1 test run (out of 10) has to be added manually since it has unique duration data not present in the other test runs
	level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.12, "0.12"))

	// the remaining 1 test run (out of 10) has to be added manually since it has unique duration data not present in the other test runs
	level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.13, "0.13"))

	var level1Summaries []common.Level1Summary
	level1Summaries = append(level1Summaries, level1TestResult1Rows)
	level1Summaries = append(level1Summaries, level1TestResult2Rows)
	level1Summaries = append(level1Summaries, level1TestResult3Rows)
	level1Summaries = append(level1Summaries, level1TestResult4Rows)
	level1Summaries = append(level1Summaries, level1TestResult5Rows)
	level1Summaries = append(level1Summaries, level1TestResult6Rows)
	return level1Summaries
}

// GetTestData_Level2MultipleL1SummariesFailuresPassesExceptions represents multiple level 1 summaries (as input into a level 2 parser)
// where there are many passed, failed and no result tests within the level level 1 summaries.
func GetTestData_Level2MultipleL1SummariesFailuresPassesExceptions() []common.Level1Summary {
	// level 1 summary with many passed tests, 1 failed test, count=1
	var level1TestResult1Rows []common.Level1TestResult
	level1TestResult1Rows = append(level1TestResult1Rows, getFailedTest_TestSanitySha3_256())
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTest("TestSanitySha3_384"))
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTest("TestSanitySha2_256"))
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTest("TestSanitySha2_384"))
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTest("TestSanityKmac128"))
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632498687765218000\n"))
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1632498687765661000\n"))
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.11, "0.11"))
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))

	// level 1 summary with many passed tests, count=1
	var level1TestResult2Rows []common.Level1TestResult
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTest("TestSanitySha3_256"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTest("TestSanitySha2_256"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTest("TestSanitySha3_384"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTest("TestSanitySha2_384"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTest("TestSanityKmac128"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTest("TestHashersAPI"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTestElapsed("TestSha3", 0.23, "0.23"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))

	// level 1 summary with many passed tests, count=1
	var level1TestResult3Rows []common.Level1TestResult
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTest("TestSanitySha3_256"))
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTest("TestSanitySha3_384"))
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTest("TestSanitySha2_384"))
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTest("TestSanityKmac128"))
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTestElapsed("TestSha3", 0.24, "0.24"))
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.11, "0.11"))
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.13, "0.13"))

	// level 1 summary with many passed tests, count=1
	var level1TestResult4Rows []common.Level1TestResult
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTest("TestSanitySha3_256"))
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTest("TestSanitySha3_256"))
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTest("TestSanitySha2_256"))
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTest("TestSanitySha2_256"))
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTest("TestSanitySha3_384"))
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTest("TestSanitySha3_384"))
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTest("TestSanitySha2_384"))
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTest("TestSanitySha2_384"))
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTest("TestSanityKmac128"))
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTest("TestSanityKmac128"))
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTest("TestHashersAPI"))
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTest("TestHashersAPI"))
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTestElapsed("TestSha3", 0.23, "0.23"))
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTestElapsed("TestSha3", 0.22, "0.22"))
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"))
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"))
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))
	level1TestResult4Rows = append(level1TestResult4Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))

	// level 1 summary with many passed tests, 1 failed test, count=10
	var level1TestResult5Rows []common.Level1TestResult

	for i := 0; i < 10; i++ {
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTest("TestSanitySha3_256"))
		level1TestResult5Rows = append(level1TestResult5Rows, getFailedTest_TestSanitySha2_256())
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTest("TestSanitySha3_384"))
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTest("TestSanitySha2_384"))
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTest("TestSanityKmac128"))
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTest("TestHashersAPI"))
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))
	}

	// the following test results have to be added in a separate loop because their results are the same 8 times out 10 test runs
	for i := 0; i < 8; i++ {
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTestElapsed("TestSha3", 0.22, "0.22"))
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"))
	}

	// the remaining 2 test runs (out of 10) have to be added manually since they have unique duration data not present in the other test runs
	level1TestResult5Rows = append(level1TestResult5Rows, getPassedTestElapsed("TestSha3", 0.24, "0.24"))
	level1TestResult5Rows = append(level1TestResult5Rows, getPassedTestElapsed("TestSha3", 0.23, "0.23"))

	// the remaining 2 test runs (out of 10) have to be added manually since they have unique duration data not present in the other test runs
	level1TestResult5Rows = append(level1TestResult5Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.12, "0.12"))
	level1TestResult5Rows = append(level1TestResult5Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.11, "0.11"))

	// level 1 summary with many passed tests, count=10
	var level1TestResult6Rows []common.Level1TestResult
	for i := 0; i < 10; i++ {
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTest("TestSanitySha3_256"))
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTest("TestSanitySha2_256"))
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTest("TestSanitySha3_384"))
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTest("TestSanitySha2_384"))
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTest("TestSanityKmac128"))
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTest("TestHashersAPI"))
	}

	// the following test results have to be added in a separate loop because their results are the same 9 times out 10 test runs
	for i := 0; i < 9; i++ {
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"))
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))
	}
	// the remaining 1 test run (out of 10) have to be added manually since they have unique duration data not present in the other test runs
	level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.12, "0.12"))
	level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.13, "0.13"))

	// the following test results have to be added in a separate loop because their results are the same 6 times out 10 test runs
	for i := 0; i < 6; i++ {
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3", 0.22, "0.22"))
	}
	// the remaining 4 test runs (out of 10) have to be added manually since they have unique duration data not present in the other test runs
	level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3", 0.24, "0.24"))
	level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3", 0.23, "0.23"))
	level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3", 0.23, "0.23"))
	level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3", 0.23, "0.23"))

	// level 1 summary with many passed tests, 1 no result test, count=3
	var level1TestResult7Rows []common.Level1TestResult
	for i := 0; i < 3; i++ {
		level1TestResult7Rows = append(level1TestResult7Rows, getPassedTestPackage("TestEncodableNetworkPrivKey", "github.com/onflow/flow-go/model/encodable"))
		level1TestResult7Rows = append(level1TestResult7Rows, getPassedTestPackage("TestEncodableNetworkPrivKeyNil", "github.com/onflow/flow-go/model/encodable"))
		level1TestResult7Rows = append(level1TestResult7Rows, getPassedTestPackage("TestEncodableNetworkPubKey", "github.com/onflow/flow-go/model/encodable"))
		level1TestResult7Rows = append(level1TestResult7Rows, getPassedTestPackage("TestEncodableNetworkPubKeyNil", "github.com/onflow/flow-go/model/encodable"))
		level1TestResult7Rows = append(level1TestResult7Rows, getPassedTestPackage("TestEncodableRandomBeaconPrivKey", "github.com/onflow/flow-go/model/encodable"))
		level1TestResult7Rows = append(level1TestResult7Rows, getPassedTestPackage("TestEncodableRandomBeaconPrivKeyNil", "github.com/onflow/flow-go/model/encodable"))
		level1TestResult7Rows = append(level1TestResult7Rows, getPassedTestPackage("TestEncodableRandomBeaconPubKey", "github.com/onflow/flow-go/model/encodable"))
		level1TestResult7Rows = append(level1TestResult7Rows, getPassedTestPackage("TestEncodableRandomBeaconPubKeyNil", "github.com/onflow/flow-go/model/encodable"))
		level1TestResult7Rows = append(level1TestResult7Rows, getPassedTestPackage("TestEncodableStakingPrivKey", "github.com/onflow/flow-go/model/encodable"))
		level1TestResult7Rows = append(level1TestResult7Rows, getPassedTestPackage("TestEncodableStakingPrivKeyNil", "github.com/onflow/flow-go/model/encodable"))
		level1TestResult7Rows = append(level1TestResult7Rows, getPassedTestPackage("TestEncodableStakingPubKey", "github.com/onflow/flow-go/model/encodable"))
		level1TestResult7Rows = append(level1TestResult7Rows, getPassedTestPackage("TestEncodableStakingPubKeyNil", "github.com/onflow/flow-go/model/encodable"))
		level1TestResult7Rows = append(level1TestResult7Rows, getPassedTestPackage("TestIsHexString", "github.com/onflow/flow-go/model/encodable"))
		level1TestResult7Rows = append(level1TestResult7Rows, getExceptionTest_TestEncodableRandomBeaconPrivKeyMsgPack())
	}

	// level 1 summary with 1 no result test, count=1
	var level1TestResult8Rows []common.Level1TestResult
	level1TestResult8Rows = append(level1TestResult8Rows, getExceptionTest_TestEncodableRandomBeaconPrivKeyMsgPack())

	// level 1 summary with 1 no result test, 1 passed test, count=5
	var level1TestResult9Rows []common.Level1TestResult
	for i := 0; i < 4; i++ {
		level1TestResult9Rows = append(level1TestResult9Rows, getExceptionTest_TestEncodableRandomBeaconPrivKeyMsgPack())
	}
	// the remaining 1 test runs (out of 5) have to be added manually since it wasn't an exception
	level1TestResult9Rows = append(level1TestResult9Rows, getPassedTestPackage("TestEncodableRandomBeaconPrivKeyMsgPack", "github.com/onflow/flow-go/model/encodable"))

	// level 1 summary with 1 no result test, count=5
	var level1TestResult10Rows []common.Level1TestResult
	for i := 0; i < 5; i++ {
		level1TestResult10Rows = append(level1TestResult10Rows, getExceptionTest_TestEncodableRandomBeaconPrivKeyMsgPack())
	}

	var level1Summaries []common.Level1Summary
	level1Summaries = append(level1Summaries, level1TestResult1Rows)
	level1Summaries = append(level1Summaries, level1TestResult2Rows)
	level1Summaries = append(level1Summaries, level1TestResult3Rows)
	level1Summaries = append(level1Summaries, level1TestResult4Rows)
	level1Summaries = append(level1Summaries, level1TestResult5Rows)
	level1Summaries = append(level1Summaries, level1TestResult6Rows)
	level1Summaries = append(level1Summaries, level1TestResult7Rows)
	level1Summaries = append(level1Summaries, level1TestResult8Rows)
	level1Summaries = append(level1Summaries, level1TestResult9Rows)
	level1Summaries = append(level1Summaries, level1TestResult10Rows)
	return level1Summaries
}
