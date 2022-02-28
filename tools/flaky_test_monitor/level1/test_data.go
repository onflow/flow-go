package main

import (
	"time"

	"github.com/onflow/flow-go/tools/flaky_test_monitor/common"
)

type TestData struct {
	ExpectedTestRun    common.TestRun
	RawJSONTestRunFile string
}

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

func GetTestData_Level1_1CountSingleNilTest() common.TestRun {
	row1 := common.TestResultRow{
		TestResult: common.TestResult{
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

	testRun := common.TestRun{
		Rows: []common.TestResultRow{
			row1,
		},
	}
	return testRun
}

func GetTestData_Level1_1CountPass() common.TestRun {
	row1 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSanitySha3_256",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSanitySha3_256\n"},
				{Item: "--- PASS: TestSanitySha3_256 (0.00s)\n"},
			},
		},
	}

	row2 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSanitySha2_256",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSanitySha2_256\n"},
				{Item: "--- PASS: TestSanitySha2_256 (0.00s)\n"},
			},
		},
	}

	row3 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSanitySha3_384",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSanitySha3_384\n"},
				{Item: "--- PASS: TestSanitySha3_384 (0.00s)\n"},
			},
		},
	}

	row4 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSanitySha2_384",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSanitySha2_384\n"},
				{Item: "--- PASS: TestSanitySha2_384 (0.00s)\n"},
			},
		},
	}

	row5 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSanityKmac128",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSanityKmac128\n"},
				{Item: "--- PASS: TestSanityKmac128 (0.00s)\n"},
			},
		},
	}

	row6 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestHashersAPI",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestHashersAPI\n"},
				{Item: "    hash_test.go:114: math rand seed is 1632497249121800000\n"},
				{Item: "--- PASS: TestHashersAPI (0.00s)\n"},
			},
		},
	}

	row7 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSha3",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0.23,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSha3\n"},
				{Item: "    hash_test.go:158: math rand seed is 1632497249122032000\n"},
				{Item: "--- PASS: TestSha3 (0.23s)\n"},
			},
		},
	}

	row8 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSha3/SHA3_256",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0.1,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSha3/SHA3_256\n"},
				{Item: "    --- PASS: TestSha3/SHA3_256 (0.10s)\n"},
			},
		},
	}

	row9 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSha3/SHA3_384",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0.12,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSha3/SHA3_384\n"},
				{Item: "    --- PASS: TestSha3/SHA3_384 (0.12s)\n"},
			},
		},
	}

	testRun := common.TestRun{
		Rows: []common.TestResultRow{
			row1,
			row2,
			row3,
			row4,
			row5,
			row6,
			row7,
			row8,
			row9,
		},
	}
	return testRun
}

func GetTestData_Level1_1Count1FailRestPass() common.TestRun {
	row1 := common.TestResultRow{
		TestResult: common.TestResult{
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

	row2 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSanitySha3_384",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSanitySha3_384\n"},
				{Item: "--- PASS: TestSanitySha3_384 (0.00s)\n"},
			},
		},
	}

	row3 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSanitySha2_256",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSanitySha2_256\n"},
				{Item: "--- PASS: TestSanitySha2_256 (0.00s)\n"},
			},
		},
	}

	row4 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSanitySha2_384",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSanitySha2_384\n"},
				{Item: "--- PASS: TestSanitySha2_384 (0.00s)\n"},
			},
		},
	}

	row5 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSanityKmac128",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSanityKmac128\n"},
				{Item: "--- PASS: TestSanityKmac128 (0.00s)\n"},
			},
		},
	}

	row6 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestHashersAPI",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestHashersAPI\n"},
				{Item: "    hash_test.go:114: math rand seed is 1632498687765218000\n"},
				{Item: "--- PASS: TestHashersAPI (0.00s)\n"},
			},
		},
	}

	row7 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSha3",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0.23,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSha3\n"},
				{Item: "    hash_test.go:158: math rand seed is 1632498687765661000\n"},
				{Item: "--- PASS: TestSha3 (0.23s)\n"},
			},
		},
	}

	row8 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSha3/SHA3_256",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0.11,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSha3/SHA3_256\n"},
				{Item: "    --- PASS: TestSha3/SHA3_256 (0.11s)\n"},
			},
		},
	}

	row9 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSha3/SHA3_384",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0.12,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSha3/SHA3_384\n"},
				{Item: "    --- PASS: TestSha3/SHA3_384 (0.12s)\n"},
			},
		},
	}

	testRun := common.TestRun{
		Rows: []common.TestResultRow{
			row1,
			row2,
			row3,
			row4,
			row5,
			row6,
			row7,
			row8,
			row9,
		},
	}
	return testRun
}

func GetTestData_Level1_1Count2SkippedRestPass() common.TestRun {
	row1 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSanitySha3_256",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSanitySha3_256\n"},
				{Item: "--- PASS: TestSanitySha3_256 (0.00s)\n"},
			},
		},
	}

	row2 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSanitySha3_384",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSanitySha3_384\n"},
				{Item: "--- PASS: TestSanitySha3_384 (0.00s)\n"},
			},
		},
	}

	row3 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSanitySha2_384",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSanitySha2_384\n"},
				{Item: "--- PASS: TestSanitySha2_384 (0.00s)\n"},
			},
		},
	}

	row4 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSanityKmac128",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSanityKmac128\n"},
				{Item: "--- PASS: TestSanityKmac128 (0.00s)\n"},
			},
		},
	}

	row5 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSha3",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0.24,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSha3\n"},
				{Item: "    hash_test.go:160: math rand seed is 1633518697589650000\n"},
				{Item: "--- PASS: TestSha3 (0.24s)\n"},
			},
		},
	}

	row6 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSha3/SHA3_256",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0.11,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSha3/SHA3_256\n"},
				{Item: "    --- PASS: TestSha3/SHA3_256 (0.11s)\n"},
			},
		},
	}

	row7 := common.TestResultRow{
		TestResult: common.TestResult{
			CommitSha:  getCommitSha(),
			CommitDate: getCommitDate(),
			JobRunDate: getJobRunDate(),
			Test:       "TestSha3/SHA3_384",
			Package:    getCryptoHashPackage(),
			Result:     "1",
			Elapsed:    0.13,
			Output: []struct {
				Item string "json:\"item\""
			}{
				{Item: "=== RUN   TestSha3/SHA3_384\n"},
				{Item: "    --- PASS: TestSha3/SHA3_384 (0.13s)\n"},
			},
		},
	}

	testRun := common.TestRun{
		Rows: []common.TestResultRow{
			row1,
			row2,
			row3,
			row4,
			row5,
			row6,
			row7,
		},
	}
	return testRun
}
