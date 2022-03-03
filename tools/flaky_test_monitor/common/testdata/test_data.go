package testdata

import (
	"time"

	"github.com/onflow/flow-go/tools/flaky_test_monitor/common"
)

// Level1TestData is used by tests to store what the expected test result should be and what the raw
// JSON input file is
type Level1TestData struct {
	ExpectedTestRun    common.Level1Summary
	RawJSONTestRunFile string
}

type Level2TestData struct {
	Directory        string
	HasFailures      bool
	HasNoResultTests bool
	TestRuns         []common.Level1Summary
}

// ************** Helper Functions *****************
// following functions are used to construct expected TestRun data

func getCommitDate() time.Time {
	commitDate, err := time.Parse(time.RFC3339, "2021-09-22T01:06:25Z")
	common.AssertNoError(err, "time parse - commit date")
	return commitDate
}

func getJobRunDate() time.Time {
	jobRunDate, err := time.Parse(time.RFC3339, "2021-09-22T04:06:25Z")
	common.AssertNoError(err, "time parse - job run date")
	return jobRunDate
}

func getCommitSha() string {
	return "46baf6c6be29af9c040bc14195e195848598bbae"
}

func getCryptoHashPackage() string {
	return "github.com/onflow/flow-go/crypto/hash"
}

func getPassedTest(name string) common.Level1TestResultRow {
	return getPassedTestPackage(name, getCryptoHashPackage())
}

func getPassedTestPackage(name string, packageName string) common.Level1TestResultRow {
	row := common.Level1TestResultRow{
		TestResult: common.Level1TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       name,
			Package:    packageName,
			Result:     "1",
			Elapsed:    0,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   " + name + "\n"},
				{Item: "--- PASS: " + name + " (0.00s)\n"},
			},
		},
	}
	return row
}

func getPassedTestElapsed(name string, elapsed float32, elapsedStr string) common.Level1TestResultRow {
	row := common.Level1TestResultRow{
		TestResult: common.Level1TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       name,
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    elapsed,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   " + name + "\n"},
				{Item: "    --- PASS: " + name + " (" + elapsedStr + "s)\n"},
			},
		},
	}
	return row
}

func getPassedTestPackageElapsedOutput(name string, packageName string, elapsed float32, elapsedStr string, output string) common.Level1TestResultRow {
	row := getPassedTestElapsedOutput(name, elapsed, elapsedStr, output)
	row.TestResult.Package = packageName
	return row
}

func getPassedTestElapsedOutput(name string, elapsed float32, elapsedStr string, output string) common.Level1TestResultRow {
	row := common.Level1TestResultRow{
		TestResult: common.Level1TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       name,
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    elapsed,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   " + name + "\n"},
				{Item: output},
				{Item: "--- PASS: " + name + " (" + elapsedStr + "s)\n"},
			},
		},
	}
	return row
}

func getFailedTest_TestSanitySha2_256() common.Level1TestResultRow {
	row := common.Level1TestResultRow{
		TestResult: common.Level1TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSanitySha2_256",
			Package:    getCryptoHashPackage(),
			Result:     "0",
			Elapsed:    0,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSanitySha2_256\n"},
				{Item: "    hash_test.go:41: \n"},
				{Item: "        \tError Trace:\thash_test.go:41\n"},
				{Item: "        \tError:      \tNot equal: \n"},
				{Item: "        \t            \texpected: hash.Hash{0x9f, 0x86, 0xd0, 0x81, 0x88, 0x4c, 0x7d, 0x65, 0x9a, 0x2f, 0xea, 0xa0, 0xc5, 0x5a, 0xd0, 0x15, 0xa3, 0xbf, 0x4f, 0x1b, 0x2b, 0xb, 0x82, 0x2c, 0xd1, 0x5d, 0x6c, 0x15, 0xb0, 0xf0, 0xa, 0x9}\n"},
				{Item: "        \t            \tactual  : hash.Hash{0x9f, 0x86, 0xd0, 0x81, 0x88, 0x4c, 0x7d, 0x65, 0x9a, 0x2f, 0xea, 0xa0, 0xc5, 0x5a, 0xd0, 0x15, 0xa3, 0xbf, 0x4f, 0x1b, 0x2b, 0xb, 0x82, 0x2c, 0xd1, 0x5d, 0x6c, 0x15, 0xb0, 0xf0, 0xa, 0x8}\n"},
				{Item: "        \t            \t\n"},
				{Item: "        \t            \tDiff:\n"},
				{Item: "        \t            \t--- Expected\n"},
				{Item: "        \t            \t+++ Actual\n"},
				{Item: "        \t            \t@@ -1,2 +1,2 @@\n"},
				{Item: "        \t            \t-(hash.Hash) (len=32) 0x9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a09\n"},
				{Item: "        \t            \t+(hash.Hash) (len=32) 0x9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08\n"},
				{Item: "        \t            \t \n"},
				{Item: "        \tTest:       \tTestSanitySha2_256\n"},
				{Item: "--- FAIL: TestSanitySha2_256 (0.00s)\n"},
			},
		},
	}
	return row
}

func getFailedTest_TestSanitySha3_256() common.Level1TestResultRow {
	row := common.Level1TestResultRow{
		TestResult: common.Level1TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSanitySha3_256",
			Package:    getCryptoHashPackage(),
			Result:     "0",
			Elapsed:    0,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSanitySha3_256\n"},
				{Item: "    hash_test.go:21: \n"},
				{Item: "        \tError Trace:\thash_test.go:21\n"},
				{Item: "        \tError:      \tNot equal: \n"},
				{Item: "        \t            \texpected: hash.Hash{0x36, 0xf0, 0x28, 0x58, 0xb, 0xb0, 0x2c, 0xc8, 0x27, 0x2a, 0x9a, 0x2, 0xf, 0x42, 0x0, 0xe3, 0x46, 0xe2, 0x76, 0xae, 0x66, 0x4e, 0x45, 0xee, 0x80, 0x74, 0x55, 0x74, 0xe2, 0xf5, 0xab, 0x81}\n"},
				{Item: "        \t            \tactual  : hash.Hash{0x36, 0xf0, 0x28, 0x58, 0xb, 0xb0, 0x2c, 0xc8, 0x27, 0x2a, 0x9a, 0x2, 0xf, 0x42, 0x0, 0xe3, 0x46, 0xe2, 0x76, 0xae, 0x66, 0x4e, 0x45, 0xee, 0x80, 0x74, 0x55, 0x74, 0xe2, 0xf5, 0xab, 0x80}\n"},
				{Item: "        \t            \t\n"},
				{Item: "        \t            \tDiff:\n"},
				{Item: "        \t            \t--- Expected\n"},
				{Item: "        \t            \t+++ Actual\n"},
				{Item: "        \t            \t@@ -1,2 +1,2 @@\n"},
				{Item: "        \t            \t-(hash.Hash) (len=32) 0x36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab81\n"},
				{Item: "        \t            \t+(hash.Hash) (len=32) 0x36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80\n"},
				{Item: "        \t            \t \n"},
				{Item: "        \tTest:       \tTestSanitySha3_256\n"},
				{Item: "--- FAIL: TestSanitySha3_256 (0.00s)\n"},
			},
		},
	}
	return row
}

func getNilTest_TestEncodableRandomBeaconPrivKeyMsgPack() common.Level1TestResultRow {
	row := common.Level1TestResultRow{
		TestResult: common.Level1TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestEncodableRandomBeaconPrivKeyMsgPack",
			Package:    "github.com/onflow/flow-go/model/encodable",
			Result:     "-100",
			Elapsed:    0,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestEncodableRandomBeaconPrivKeyMsgPack\n"},
				{Item: "bytes: 194--- PASS: TestEncodableRandomBeaconPrivKeyMsgPack (0.00s)\n"},
			},
		},
	}
	return row
}

// ************** Level 1 - Expected Test Run Functions *****************
// following functions are used by unit tests for constructing expected TestRun data

func GetTestData_Level1_1CountSingleNilTest() common.Level1Summary {
	testRun := common.Level1Summary{
		Rows: []common.Level1TestResultRow{
			getNilTest_TestEncodableRandomBeaconPrivKeyMsgPack(),
		},
	}
	return testRun
}

func GetTestData_Level1_1CountPass() common.Level1Summary {
	testRun := common.Level1Summary{
		Rows: []common.Level1TestResultRow{
			getPassedTest("TestSanitySha3_256"),
			getPassedTest("TestSanitySha2_256"),
			getPassedTest("TestSanitySha3_384"),
			getPassedTest("TestSanitySha2_384"),
			getPassedTest("TestSanityKmac128"),
			getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632497249121800000\n"),
			getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1632497249122032000\n"),
			getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"),
			getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"),
		},
	}
	return testRun
}

func GetTestData_Level1_1Count1FailRestPass() common.Level1Summary {
	testRun := common.Level1Summary{
		Rows: []common.Level1TestResultRow{
			getFailedTest_TestSanitySha3_256(),
			getPassedTest("TestSanitySha3_384"),
			getPassedTest("TestSanitySha2_256"),
			getPassedTest("TestSanitySha2_384"),
			getPassedTest("TestSanityKmac128"),
			getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632498687765218000\n"),
			getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1632498687765661000\n"),
			getPassedTestElapsed("TestSha3/SHA3_256", 0.11, "0.11"),
			getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"),
		},
	}
	return testRun
}

func GetTestData_Level1_1Count2SkippedRestPass() common.Level1Summary {
	testRun := common.Level1Summary{
		Rows: []common.Level1TestResultRow{
			getPassedTest("TestSanitySha3_256"),
			getPassedTest("TestSanitySha3_384"),
			getPassedTest("TestSanitySha2_384"),
			getPassedTest("TestSanityKmac128"),
			getPassedTestElapsedOutput("TestSha3", 0.24, "0.24", "    hash_test.go:160: math rand seed is 1633518697589650000\n"),
			getPassedTestElapsed("TestSha3/SHA3_256", 0.11, "0.11"),
			getPassedTestElapsed("TestSha3/SHA3_384", 0.13, "0.13"),
		},
	}
	return testRun
}

func GetTestData_Level1_2CountPass() common.Level1Summary {
	testRun := common.Level1Summary{
		Rows: []common.Level1TestResultRow{
			getPassedTest("TestSanitySha3_256"),
			getPassedTest("TestSanitySha3_256"),
			getPassedTest("TestSanitySha2_256"),
			getPassedTest("TestSanitySha2_256"),
			getPassedTest("TestSanitySha3_384"),
			getPassedTest("TestSanitySha3_384"),
			getPassedTest("TestSanitySha2_384"),
			getPassedTest("TestSanitySha2_384"),
			getPassedTest("TestSanityKmac128"),
			getPassedTest("TestSanityKmac128"),
			getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1633358050203144000\n"),
			getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1633358050430256000\n"),
			getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1633358050203374000\n"),
			getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1633358050430467000\n"),
			getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"),
			getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"),
			getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"),
			getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"),
		},
	}
	return testRun
}

func GetTestData_Level1_10CountPass() common.Level1Summary {
	var testResultRows []common.Level1TestResultRow

	for i := 0; i < 10; i++ {
		testResultRows = append(testResultRows, getPassedTest("TestSanitySha3_256"))
		testResultRows = append(testResultRows, getPassedTest("TestSanitySha2_256"))
		testResultRows = append(testResultRows, getPassedTest("TestSanitySha3_384"))
		testResultRows = append(testResultRows, getPassedTest("TestSanitySha2_384"))
		testResultRows = append(testResultRows, getPassedTest("TestSanityKmac128"))
	}

	for i := 0; i < 9; i++ {
		testResultRows = append(testResultRows, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))
		testResultRows = append(testResultRows, getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"))
	}
	testResultRows = append(testResultRows, getPassedTestElapsed("TestSha3/SHA3_384", 0.13, "0.13"))
	testResultRows = append(testResultRows, getPassedTestElapsed("TestSha3/SHA3_256", 0.12, "0.12"))

	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739552470379000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739552696815000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739552917474000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739553140451000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739553362249000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739553605325000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739553826502000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739554054239000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739554280043000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739554500707000\n"))

	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1632739552470723000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739552697024000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739552917708000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739553140702000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestSha3", 0.24, "0.24", "    hash_test.go:158: math rand seed is 1632739553362497000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739553605582000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1632739553826733000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1632739554054464000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739554280256000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739554500935000\n"))

	testRun := common.Level1Summary{
		Rows: testResultRows,
	}
	return testRun
}

func GetTestData_Level1_10CountSomeFailures() common.Level1Summary {
	var testResultRows []common.Level1TestResultRow

	for i := 0; i < 10; i++ {
		testResultRows = append(testResultRows, getPassedTest("TestSanitySha3_256"))
		testResultRows = append(testResultRows, getFailedTest_TestSanitySha2_256())
		testResultRows = append(testResultRows, getPassedTest("TestSanitySha3_384"))
		testResultRows = append(testResultRows, getPassedTest("TestSanitySha2_384"))
		testResultRows = append(testResultRows, getPassedTest("TestSanityKmac128"))
		testResultRows = append(testResultRows, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))
	}

	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739682184421000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739682415309000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739682637108000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739682857435000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739683077064000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739683297507000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739683518492000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739683740724000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739683980033000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632739684200452000\n"))

	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1632739682184858000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739682415616000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739682637311000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739682857668000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739683077268000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739683297711000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739683518781000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestSha3", 0.24, "0.24", "    hash_test.go:158: math rand seed is 1632739683740970000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739683980266000\n"))
	testResultRows = append(testResultRows, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1632739684200658000\n"))

	for i := 0; i < 8; i++ {
		testResultRows = append(testResultRows, getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"))
	}

	testResultRows = append(testResultRows, getPassedTestElapsed("TestSha3/SHA3_256", 0.11, "0.11"))
	testResultRows = append(testResultRows, getPassedTestElapsed("TestSha3/SHA3_256", 0.12, "0.12"))

	testRun := common.Level1Summary{
		Rows: testResultRows,
	}
	return testRun
}

func GetTestData_Level1_5CountSingleNilTest() common.Level1Summary {
	var testResultRows []common.Level1TestResultRow

	for i := 0; i < 5; i++ {
		testResultRows = append(testResultRows, getNilTest_TestEncodableRandomBeaconPrivKeyMsgPack())
	}

	testRun := common.Level1Summary{
		Rows: testResultRows,
	}
	return testRun
}

func GetTestData_Level1_5CountMultipleNilTests() common.Level1Summary {
	var testResultRows []common.Level1TestResultRow
	for i := 0; i < 4; i++ {
		testResultRows = append(testResultRows, getNilTest_TestEncodableRandomBeaconPrivKeyMsgPack())
	}
	testResultRows = append(testResultRows, getPassedTestPackageElapsedOutput("TestEncodableRandomBeaconPrivKeyMsgPack", "github.com/onflow/flow-go/model/encodable", 0, "0.00", "    keys_test.go:245: bytes: 194\n"))

	testRun := common.Level1Summary{
		Rows: testResultRows,
	}
	return testRun
}

func GetTestData_Leve1_3CountNilWithNormalTests() common.Level1Summary {
	var testResultRows []common.Level1TestResultRow
	row1 := getPassedTestPackage("TestEncodableNetworkPrivKey", "github.com/onflow/flow-go/model/encodable")
	row2 := getPassedTestPackage("TestEncodableNetworkPrivKeyNil", "github.com/onflow/flow-go/model/encodable")
	row3 := getPassedTestPackage("TestEncodableNetworkPubKey", "github.com/onflow/flow-go/model/encodable")
	row4 := getPassedTestPackage("TestEncodableNetworkPubKeyNil", "github.com/onflow/flow-go/model/encodable")
	row5 := getPassedTestPackage("TestEncodableRandomBeaconPrivKey", "github.com/onflow/flow-go/model/encodable")
	row6 := getPassedTestPackage("TestEncodableRandomBeaconPrivKeyNil", "github.com/onflow/flow-go/model/encodable")
	row7 := getPassedTestPackage("TestEncodableRandomBeaconPubKey", "github.com/onflow/flow-go/model/encodable")
	row8 := getPassedTestPackage("TestEncodableRandomBeaconPubKeyNil", "github.com/onflow/flow-go/model/encodable")
	row9 := getPassedTestPackage("TestEncodableStakingPrivKey", "github.com/onflow/flow-go/model/encodable")
	row10 := getPassedTestPackage("TestEncodableStakingPrivKeyNil", "github.com/onflow/flow-go/model/encodable")
	row11 := getPassedTestPackage("TestEncodableStakingPubKey", "github.com/onflow/flow-go/model/encodable")
	row12 := getPassedTestPackage("TestEncodableStakingPubKeyNil", "github.com/onflow/flow-go/model/encodable")
	row13 := getPassedTestPackage("TestIsHexString", "github.com/onflow/flow-go/model/encodable")

	row14 := getNilTest_TestEncodableRandomBeaconPrivKeyMsgPack()

	for i := 0; i < 3; i++ {
		testResultRows = append(testResultRows, row1)
		testResultRows = append(testResultRows, row2)
		testResultRows = append(testResultRows, row3)
		testResultRows = append(testResultRows, row4)
		testResultRows = append(testResultRows, row5)
		testResultRows = append(testResultRows, row6)
		testResultRows = append(testResultRows, row7)
		testResultRows = append(testResultRows, row8)
		testResultRows = append(testResultRows, row9)
		testResultRows = append(testResultRows, row10)
		testResultRows = append(testResultRows, row11)
		testResultRows = append(testResultRows, row12)
		testResultRows = append(testResultRows, row13)
		testResultRows = append(testResultRows, row14)
	}

	testRun := common.Level1Summary{
		Rows: testResultRows,
	}
	return testRun
}

// ************** Level 2 - Expected Test Runs Functions *****************
// following functions are used by unit tests for constructing expected TestRuns data

func GetTestData_Level2_1FailureRestPass() []common.Level1Summary {
	var testResult1Rows []common.Level1TestResultRow
	testResult1Rows = append(testResult1Rows, getFailedTest_TestSanitySha3_256())
	testResult1Rows = append(testResult1Rows, getPassedTest("TestSanitySha3_384"))
	testResult1Rows = append(testResult1Rows, getPassedTest("TestSanitySha2_256"))
	testResult1Rows = append(testResult1Rows, getPassedTest("TestSanitySha2_384"))
	testResult1Rows = append(testResult1Rows, getPassedTest("TestSanityKmac128"))
	testResult1Rows = append(testResult1Rows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632498687765218000\n"))
	testResult1Rows = append(testResult1Rows, getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1632498687765661000\n"))
	testResult1Rows = append(testResult1Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.11, "0.11"))
	testResult1Rows = append(testResult1Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))

	testRun1 := common.Level1Summary{
		Rows: testResult1Rows,
	}

	var testRuns []common.Level1Summary
	testRuns = append(testRuns, testRun1)
	return testRuns
}

func GetTestsData_Level2_1NoResultNoOtherTests() []common.Level1Summary {
	row1_1 := getNilTest_TestEncodableRandomBeaconPrivKeyMsgPack()

	var testResult1Rows []common.Level1TestResultRow
	testResult1Rows = append(testResult1Rows, row1_1)

	testRun1 := common.Level1Summary{
		Rows: testResult1Rows,
	}

	var testRuns []common.Level1Summary
	testRuns = append(testRuns, testRun1)
	return testRuns
}

func GetTestData_Level2_MultipleL1SummariesNoResults() []common.Level1Summary {
	var testResult1Rows []common.Level1TestResultRow
	row1_1 := getPassedTestPackage("TestEncodableNetworkPrivKey", "github.com/onflow/flow-go/model/encodable")
	row2_1 := getPassedTestPackage("TestEncodableNetworkPrivKeyNil", "github.com/onflow/flow-go/model/encodable")
	row3_1 := getPassedTestPackage("TestEncodableNetworkPubKey", "github.com/onflow/flow-go/model/encodable")
	row4_1 := getPassedTestPackage("TestEncodableNetworkPubKeyNil", "github.com/onflow/flow-go/model/encodable")
	row5_1 := getPassedTestPackage("TestEncodableRandomBeaconPrivKey", "github.com/onflow/flow-go/model/encodable")
	row6_1 := getPassedTestPackage("TestEncodableRandomBeaconPrivKeyNil", "github.com/onflow/flow-go/model/encodable")
	row7_1 := getPassedTestPackage("TestEncodableRandomBeaconPubKey", "github.com/onflow/flow-go/model/encodable")
	row8_1 := getPassedTestPackage("TestEncodableRandomBeaconPubKeyNil", "github.com/onflow/flow-go/model/encodable")
	row9_1 := getPassedTestPackage("TestEncodableStakingPrivKey", "github.com/onflow/flow-go/model/encodable")
	row10_1 := getPassedTestPackage("TestEncodableStakingPrivKeyNil", "github.com/onflow/flow-go/model/encodable")
	row11_1 := getPassedTestPackage("TestEncodableStakingPubKey", "github.com/onflow/flow-go/model/encodable")
	row12_1 := getPassedTestPackage("TestEncodableStakingPubKeyNil", "github.com/onflow/flow-go/model/encodable")
	row13_1 := getPassedTestPackage("TestIsHexString", "github.com/onflow/flow-go/model/encodable")
	row14_1 := getNilTest_TestEncodableRandomBeaconPrivKeyMsgPack()

	for i := 0; i < 3; i++ {
		testResult1Rows = append(testResult1Rows, row1_1)
		testResult1Rows = append(testResult1Rows, row2_1)
		testResult1Rows = append(testResult1Rows, row3_1)
		testResult1Rows = append(testResult1Rows, row4_1)
		testResult1Rows = append(testResult1Rows, row5_1)
		testResult1Rows = append(testResult1Rows, row6_1)
		testResult1Rows = append(testResult1Rows, row7_1)
		testResult1Rows = append(testResult1Rows, row8_1)
		testResult1Rows = append(testResult1Rows, row9_1)
		testResult1Rows = append(testResult1Rows, row10_1)
		testResult1Rows = append(testResult1Rows, row11_1)
		testResult1Rows = append(testResult1Rows, row12_1)
		testResult1Rows = append(testResult1Rows, row13_1)
		testResult1Rows = append(testResult1Rows, row14_1)
	}

	testRun1 := common.Level1Summary{
		Rows: testResult1Rows,
	}

	row1_2 := getNilTest_TestEncodableRandomBeaconPrivKeyMsgPack()

	var testResult2Rows []common.Level1TestResultRow
	testResult2Rows = append(testResult2Rows, row1_2)
	testRun2 := common.Level1Summary{
		Rows: testResult2Rows,
	}

	row1_3 := getNilTest_TestEncodableRandomBeaconPrivKeyMsgPack()
	row2_3 := getPassedTestPackageElapsedOutput("TestEncodableRandomBeaconPrivKeyMsgPack", "github.com/onflow/flow-go/model/encodable", 0, "0.00", "    keys_test.go:245: bytes: 194\n")

	var testResult3Rows []common.Level1TestResultRow
	for i := 0; i < 4; i++ {
		testResult3Rows = append(testResult3Rows, row1_3)
	}
	testResult3Rows = append(testResult3Rows, row2_3)

	testRun3 := common.Level1Summary{
		Rows: testResult3Rows,
	}

	var testResult4Rows []common.Level1TestResultRow
	row1_4 := getNilTest_TestEncodableRandomBeaconPrivKeyMsgPack()

	for i := 0; i < 5; i++ {
		testResult4Rows = append(testResult4Rows, row1_4)
	}

	testRun4 := common.Level1Summary{
		Rows: testResult4Rows,
	}

	var testRuns []common.Level1Summary
	testRuns = append(testRuns, testRun1)
	testRuns = append(testRuns, testRun2)
	testRuns = append(testRuns, testRun3)
	testRuns = append(testRuns, testRun4)
	return testRuns
}

func GetTestData_Level2MultipleL1SummariesFailuresPasses() []common.Level1Summary {
	var level1TestResultRows1 []common.Level1TestResultRow
	level1TestResultRows1 = append(level1TestResultRows1, getFailedTest_TestSanitySha3_256())
	level1TestResultRows1 = append(level1TestResultRows1, getPassedTest("TestSanitySha3_384"))
	level1TestResultRows1 = append(level1TestResultRows1, getPassedTest("TestSanitySha2_256"))
	level1TestResultRows1 = append(level1TestResultRows1, getPassedTest("TestSanitySha2_384"))
	level1TestResultRows1 = append(level1TestResultRows1, getPassedTest("TestSanityKmac128"))
	level1TestResultRows1 = append(level1TestResultRows1, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632498687765218000\n"))
	level1TestResultRows1 = append(level1TestResultRows1, getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1632498687765661000\n"))
	level1TestResultRows1 = append(level1TestResultRows1, getPassedTestElapsed("TestSha3/SHA3_256", 0.11, "0.11"))
	level1TestResultRows1 = append(level1TestResultRows1, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))

	var level1TestResultRows2 []common.Level1TestResultRow
	level1TestResultRows2 = append(level1TestResultRows2, getPassedTest("TestSanitySha3_256"))
	level1TestResultRows2 = append(level1TestResultRows2, getPassedTest("TestSanitySha2_256"))
	level1TestResultRows2 = append(level1TestResultRows2, getPassedTest("TestSanitySha3_384"))
	level1TestResultRows2 = append(level1TestResultRows2, getPassedTest("TestSanitySha2_384"))
	level1TestResultRows2 = append(level1TestResultRows2, getPassedTest("TestSanityKmac128"))
	level1TestResultRows2 = append(level1TestResultRows2, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632497249121800000\n"))
	level1TestResultRows2 = append(level1TestResultRows2, getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1632497249122032000\n"))
	level1TestResultRows2 = append(level1TestResultRows2, getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"))
	level1TestResultRows2 = append(level1TestResultRows2, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))

	var level1TestResultRows3 []common.Level1TestResultRow
	level1TestResultRows3 = append(level1TestResultRows3, getPassedTest("TestSanitySha3_256"))
	level1TestResultRows3 = append(level1TestResultRows3, getPassedTest("TestSanitySha2_256"))
	level1TestResultRows3 = append(level1TestResultRows3, getPassedTest("TestSanitySha3_384"))
	level1TestResultRows3 = append(level1TestResultRows3, getPassedTest("TestSanitySha2_384"))
	level1TestResultRows3 = append(level1TestResultRows3, getPassedTest("TestSanityKmac128"))
	level1TestResultRows3 = append(level1TestResultRows3, getPassedTestElapsedOutput("TestSha3", 0.24, "0.24", "    hash_test.go:160: math rand seed is 1633518697589650000\n"))
	level1TestResultRows3 = append(level1TestResultRows3, getPassedTestElapsed("TestSha3/SHA3_256", 0.11, "0.11"))
	level1TestResultRows3 = append(level1TestResultRows3, getPassedTestElapsed("TestSha3/SHA3_384", 0.13, "0.13"))

	var level1TestResultRows4 []common.Level1TestResultRow
	for i := 0; i < 2; i++ {
		level1TestResultRows4 = append(level1TestResultRows4, getPassedTest("TestSanitySha3_256"))
		level1TestResultRows4 = append(level1TestResultRows4, getPassedTest("TestSanitySha2_256"))
		level1TestResultRows4 = append(level1TestResultRows4, getPassedTest("TestSanitySha3_384"))
		level1TestResultRows4 = append(level1TestResultRows4, getPassedTest("TestSanitySha2_384"))
		level1TestResultRows4 = append(level1TestResultRows4, getPassedTest("TestSanityKmac128"))
		level1TestResultRows4 = append(level1TestResultRows4, getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"))
		level1TestResultRows4 = append(level1TestResultRows4, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))
	}
	level1TestResultRows4 = append(level1TestResultRows4, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1633358050430256000\n"))
	level1TestResultRows4 = append(level1TestResultRows4, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1633358050203144000\n"))

	level1TestResultRows4 = append(level1TestResultRows4, getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1633358050203374000\n"))
	level1TestResultRows4 = append(level1TestResultRows4, getPassedTestElapsedOutput("TestSha3", 0.22, "0.22", "    hash_test.go:158: math rand seed is 1633358050430467000\n"))

	var level1TestResultRows5 []common.Level1TestResultRow
	for i := 0; i < 10; i++ {
		level1TestResultRows5 = append(level1TestResultRows5, getPassedTest("TestSanitySha3_256"))
		level1TestResultRows5 = append(level1TestResultRows5, getFailedTest_TestSanitySha2_256())
		level1TestResultRows5 = append(level1TestResultRows5, getPassedTest("TestSanitySha3_384"))
		level1TestResultRows5 = append(level1TestResultRows5, getPassedTest("TestSanitySha2_384"))
		level1TestResultRows5 = append(level1TestResultRows5, getPassedTest("TestSanityKmac128"))
		level1TestResultRows5 = append(level1TestResultRows5, getPassedTest("TestHashersAPI"))
		level1TestResultRows5 = append(level1TestResultRows5, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))
	}

	for i := 0; i < 8; i++ {
		level1TestResultRows5 = append(level1TestResultRows5, getPassedTestElapsed("TestSha3", 0.22, "0.22"))
		level1TestResultRows5 = append(level1TestResultRows5, getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"))
	}
	level1TestResultRows5 = append(level1TestResultRows5, getPassedTestElapsed("TestSha3", 0.24, "0.24"))
	level1TestResultRows5 = append(level1TestResultRows5, getPassedTestElapsed("TestSha3", 0.23, "0.23"))

	level1TestResultRows5 = append(level1TestResultRows5, getPassedTestElapsed("TestSha3/SHA3_256", 0.12, "0.12"))
	level1TestResultRows5 = append(level1TestResultRows5, getPassedTestElapsed("TestSha3/SHA3_256", 0.11, "0.11"))

	var level1TestResultRows6 []common.Level1TestResultRow

	for i := 0; i < 10; i++ {
		level1TestResultRows6 = append(level1TestResultRows6, getPassedTest("TestSanitySha3_256"))
		level1TestResultRows6 = append(level1TestResultRows6, getPassedTest("TestSanitySha2_256"))
		level1TestResultRows6 = append(level1TestResultRows6, getPassedTest("TestSanitySha3_384"))
		level1TestResultRows6 = append(level1TestResultRows6, getPassedTest("TestSanitySha2_384"))
		level1TestResultRows6 = append(level1TestResultRows6, getPassedTest("TestSanityKmac128"))
		level1TestResultRows6 = append(level1TestResultRows6, getPassedTest("TestHashersAPI"))
	}

	for i := 0; i < 6; i++ {
		level1TestResultRows6 = append(level1TestResultRows6, getPassedTestElapsed("TestSha3", 0.22, "0.22"))
	}
	level1TestResultRows6 = append(level1TestResultRows6, getPassedTestElapsed("TestSha3", 0.23, "0.23"))
	level1TestResultRows6 = append(level1TestResultRows6, getPassedTestElapsed("TestSha3", 0.24, "0.24"))
	level1TestResultRows6 = append(level1TestResultRows6, getPassedTestElapsed("TestSha3", 0.23, "0.23"))
	level1TestResultRows6 = append(level1TestResultRows6, getPassedTestElapsed("TestSha3", 0.23, "0.23"))

	for i := 0; i < 9; i++ {
		level1TestResultRows6 = append(level1TestResultRows6, getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"))
		level1TestResultRows6 = append(level1TestResultRows6, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))
	}
	level1TestResultRows6 = append(level1TestResultRows6, getPassedTestElapsed("TestSha3/SHA3_256", 0.12, "0.12"))
	level1TestResultRows6 = append(level1TestResultRows6, getPassedTestElapsed("TestSha3/SHA3_384", 0.13, "0.13"))

	level1Summary1 := common.Level1Summary{
		Rows: level1TestResultRows1,
	}

	level1Summary2 := common.Level1Summary{
		Rows: level1TestResultRows2,
	}

	level1Summary3 := common.Level1Summary{
		Rows: level1TestResultRows3,
	}

	level1Summary4 := common.Level1Summary{
		Rows: level1TestResultRows4,
	}

	level1Summary5 := common.Level1Summary{
		Rows: level1TestResultRows5,
	}

	level1Summary6 := common.Level1Summary{
		Rows: level1TestResultRows6,
	}

	var level1Summaries []common.Level1Summary
	level1Summaries = append(level1Summaries, level1Summary1)
	level1Summaries = append(level1Summaries, level1Summary2)
	level1Summaries = append(level1Summaries, level1Summary3)
	level1Summaries = append(level1Summaries, level1Summary4)
	level1Summaries = append(level1Summaries, level1Summary5)
	level1Summaries = append(level1Summaries, level1Summary6)
	return level1Summaries
}

func GetTestData_Level2MultipleL1SummariesFailuresPassesNoResults() []common.Level1Summary {
	var level1TestResult1Rows []common.Level1TestResultRow
	level1TestResult1Rows = append(level1TestResult1Rows, getFailedTest_TestSanitySha3_256())
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTest("TestSanitySha3_384"))
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTest("TestSanitySha2_256"))
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTest("TestSanitySha2_384"))
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTest("TestSanityKmac128"))
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTestElapsedOutput("TestHashersAPI", 0, "0.00", "    hash_test.go:114: math rand seed is 1632498687765218000\n"))
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTestElapsedOutput("TestSha3", 0.23, "0.23", "    hash_test.go:158: math rand seed is 1632498687765661000\n"))
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.11, "0.11"))
	level1TestResult1Rows = append(level1TestResult1Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))

	var level1TestResult2Rows []common.Level1TestResultRow
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTest("TestSanitySha3_256"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTest("TestSanitySha2_256"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTest("TestSanitySha3_384"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTest("TestSanitySha2_384"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTest("TestSanityKmac128"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTest("TestHashersAPI"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTestElapsed("TestSha3", 0.23, "0.23"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"))
	level1TestResult2Rows = append(level1TestResult2Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))

	var level1TestResult3Rows []common.Level1TestResultRow
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTest("TestSanitySha3_256"))
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTest("TestSanitySha3_384"))
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTest("TestSanitySha2_384"))
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTest("TestSanityKmac128"))
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTestElapsed("TestSha3", 0.24, "0.24"))
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.11, "0.11"))
	level1TestResult3Rows = append(level1TestResult3Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.13, "0.13"))

	var level1TestResult4Rows []common.Level1TestResultRow
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

	var level1TestResult5Rows []common.Level1TestResultRow

	for i := 0; i < 10; i++ {
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTest("TestSanitySha3_256"))
		level1TestResult5Rows = append(level1TestResult5Rows, getFailedTest_TestSanitySha2_256())
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTest("TestSanitySha3_384"))
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTest("TestSanitySha2_384"))
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTest("TestSanityKmac128"))
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTest("TestHashersAPI"))
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))
	}

	for i := 0; i < 8; i++ {
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTestElapsed("TestSha3", 0.22, "0.22"))
		level1TestResult5Rows = append(level1TestResult5Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"))
	}
	level1TestResult5Rows = append(level1TestResult5Rows, getPassedTestElapsed("TestSha3", 0.24, "0.24"))
	level1TestResult5Rows = append(level1TestResult5Rows, getPassedTestElapsed("TestSha3", 0.23, "0.23"))
	level1TestResult5Rows = append(level1TestResult5Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.12, "0.12"))
	level1TestResult5Rows = append(level1TestResult5Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.11, "0.11"))

	var level1TestResult6Rows []common.Level1TestResultRow
	for i := 0; i < 10; i++ {
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTest("TestSanitySha3_256"))
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTest("TestSanitySha2_256"))
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTest("TestSanitySha3_384"))
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTest("TestSanitySha2_384"))
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTest("TestSanityKmac128"))
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTest("TestHashersAPI"))
	}

	for i := 0; i < 9; i++ {
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.1, "0.10"))
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.12, "0.12"))
	}
	level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3/SHA3_256", 0.12, "0.12"))
	level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3/SHA3_384", 0.13, "0.13"))

	for i := 0; i < 6; i++ {
		level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3", 0.22, "0.22"))
	}
	level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3", 0.24, "0.24"))
	level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3", 0.23, "0.23"))
	level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3", 0.23, "0.23"))
	level1TestResult6Rows = append(level1TestResult6Rows, getPassedTestElapsed("TestSha3", 0.23, "0.23"))

	var level1TestResult7Rows []common.Level1TestResultRow
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
		level1TestResult7Rows = append(level1TestResult7Rows, getNilTest_TestEncodableRandomBeaconPrivKeyMsgPack())
	}

	var level1TestResult8Rows []common.Level1TestResultRow
	level1TestResult8Rows = append(level1TestResult8Rows, getNilTest_TestEncodableRandomBeaconPrivKeyMsgPack())

	var level1TestResult9Rows []common.Level1TestResultRow
	for i := 0; i < 4; i++ {
		level1TestResult9Rows = append(level1TestResult9Rows, getNilTest_TestEncodableRandomBeaconPrivKeyMsgPack())
	}
	level1TestResult9Rows = append(level1TestResult9Rows, getPassedTestPackage("TestEncodableRandomBeaconPrivKeyMsgPack", "github.com/onflow/flow-go/model/encodable"))

	var level1TestResult10Rows []common.Level1TestResultRow
	for i := 0; i < 5; i++ {
		level1TestResult10Rows = append(level1TestResult10Rows, getNilTest_TestEncodableRandomBeaconPrivKeyMsgPack())
	}

	level1Summary1 := common.Level1Summary{
		Rows: level1TestResult1Rows,
	}

	level1Summary2 := common.Level1Summary{
		Rows: level1TestResult2Rows,
	}

	level1Summary3 := common.Level1Summary{
		Rows: level1TestResult3Rows,
	}

	level1Summary4 := common.Level1Summary{
		Rows: level1TestResult4Rows,
	}

	level1Summary5 := common.Level1Summary{
		Rows: level1TestResult5Rows,
	}

	level1Summary6 := common.Level1Summary{
		Rows: level1TestResult6Rows,
	}

	level1Summary7 := common.Level1Summary{
		Rows: level1TestResult7Rows,
	}

	level1Summary8 := common.Level1Summary{
		Rows: level1TestResult8Rows,
	}

	level1Summary9 := common.Level1Summary{
		Rows: level1TestResult9Rows,
	}

	level1Summary10 := common.Level1Summary{
		Rows: level1TestResult10Rows,
	}

	var level1Summaries []common.Level1Summary
	level1Summaries = append(level1Summaries, level1Summary1)
	level1Summaries = append(level1Summaries, level1Summary2)
	level1Summaries = append(level1Summaries, level1Summary3)
	level1Summaries = append(level1Summaries, level1Summary4)
	level1Summaries = append(level1Summaries, level1Summary5)
	level1Summaries = append(level1Summaries, level1Summary6)
	level1Summaries = append(level1Summaries, level1Summary7)
	level1Summaries = append(level1Summaries, level1Summary8)
	level1Summaries = append(level1Summaries, level1Summary9)
	level1Summaries = append(level1Summaries, level1Summary10)
	return level1Summaries
}
