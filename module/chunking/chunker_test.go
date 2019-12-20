package chunking

import (
	"testing"

	exec "github.com/dapperlabs/flow-go/model/execution"
	"github.com/dapperlabs/flow-go/model/flow"
)

type TestCase struct {
	maxComputationLimit    uint64
	gasSpendingSeq         []uint64
	expectedNumberOfChunks int
	expectedError          bool
}

func TestGetChunks(t *testing.T) {
	testCases := []TestCase{
		{uint64(400),
			[]uint64{100, 200, 100, 200, 100, 200},
			3,
			false,
		},
		{uint64(300),
			[]uint64{100, 200, 100, 200, 100, 200},
			3,
			false,
		},
		{uint64(200),
			[]uint64{100, 200, 100, 200, 100, 200},
			6,
			false,
		},
		{uint64(100),
			[]uint64{100, 200, 100, 200, 100, 200},
			6,
			true,
		},
	}
	for _, testCase := range testCases {
		var expectedTotalComputationLimit uint64
		var txs []exec.ExecutedTransaction
		for _, spending := range testCase.gasSpendingSeq {
			expectedTotalComputationLimit += spending
			txs = append(txs, exec.ExecutedTransaction{Tx: &flow.Transaction{}, GasSpent: spending, MaxGas: 1000})
		}
		chunks, err := GetChunks(txs, testCase.maxComputationLimit)
		if err == nil {
			if testCase.expectedError {
				t.Errorf("Test Failed: expected to have an err but didn't get any")
			}
			var chunksTotalComputationLimit uint64
			for _, chunk := range chunks {
				chunksTotalComputationLimit += chunk.TotalGasSpent
			}
			if expectedTotalComputationLimit != chunksTotalComputationLimit {
				t.Errorf("Test Failed: the chunk total computation limit doesn't match the sum of txs' computation limit.")
			}
			if len(chunks) != testCase.expectedNumberOfChunks {
				t.Errorf("Test Failed: expected %v chunks, recieved: %v chunks", testCase.expectedNumberOfChunks, len(chunks))
			}
		} else {
			if !testCase.expectedError {
				t.Errorf("Test Failed: expected not having err but got an error: %v", err.Error())
			}
		}

	}
}
