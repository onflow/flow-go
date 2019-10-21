package verification

import (
	"fmt"
	"testing"

	"github.com/dapperlabs/flow-go/pkg/types"
)

type TestCase struct {
	maxComputationLimit    uint64
	txs                    []*types.Transaction
	expectedNumberOfChunks int
	expectedError          bool
}

func TestCollectionToChunks(t *testing.T) {
	testCases := []TestCase{
		{uint64(400),
			[]*types.Transaction{&types.Transaction{Script: []byte("Some codes here!"), ComputeLimit: 100},
				&types.Transaction{Script: []byte("Some codes here! 2"), ComputeLimit: 200},
				&types.Transaction{Script: []byte("Some codes here! 3"), ComputeLimit: 100},
				&types.Transaction{Script: []byte("Some codes here! 4"), ComputeLimit: 200},
				&types.Transaction{Script: []byte("Some codes here! 5"), ComputeLimit: 100},
				&types.Transaction{Script: []byte("Some codes here! 6"), ComputeLimit: 200},
			},
			3,
			false,
		},
		{uint64(300),
			[]*types.Transaction{&types.Transaction{Script: []byte("Some codes here!"), ComputeLimit: 100},
				&types.Transaction{Script: []byte("Some codes here! 2"), ComputeLimit: 200},
				&types.Transaction{Script: []byte("Some codes here! 3"), ComputeLimit: 100},
				&types.Transaction{Script: []byte("Some codes here! 4"), ComputeLimit: 200},
				&types.Transaction{Script: []byte("Some codes here! 5"), ComputeLimit: 100},
				&types.Transaction{Script: []byte("Some codes here! 6"), ComputeLimit: 200},
			},
			3,
			false,
		},
		{uint64(200),
			[]*types.Transaction{&types.Transaction{Script: []byte("Some codes here!"), ComputeLimit: 100},
				&types.Transaction{Script: []byte("Some codes here! 2"), ComputeLimit: 200},
				&types.Transaction{Script: []byte("Some codes here! 3"), ComputeLimit: 100},
				&types.Transaction{Script: []byte("Some codes here! 4"), ComputeLimit: 200},
				&types.Transaction{Script: []byte("Some codes here! 5"), ComputeLimit: 100},
				&types.Transaction{Script: []byte("Some codes here! 6"), ComputeLimit: 200},
			},
			6,
			false,
		},
		{uint64(100),
			[]*types.Transaction{&types.Transaction{Script: []byte("Some codes here!"), ComputeLimit: 100},
				&types.Transaction{Script: []byte("Some codes here! 2"), ComputeLimit: 200},
				&types.Transaction{Script: []byte("Some codes here! 3"), ComputeLimit: 100},
				&types.Transaction{Script: []byte("Some codes here! 4"), ComputeLimit: 200},
				&types.Transaction{Script: []byte("Some codes here! 5"), ComputeLimit: 100},
				&types.Transaction{Script: []byte("Some codes here! 6"), ComputeLimit: 200},
			},
			6,
			true,
		},
	}
	for _, testCase := range testCases {
		var expectedTotalComputationLimit uint64
		for _, tx := range testCase.txs {
			expectedTotalComputationLimit += tx.ComputeLimit
		}
		chunks, err := CollectionToChunks(&types.Collection{Transactions: testCase.txs}, testCase.maxComputationLimit)
		if err == nil {
			if testCase.expectedError {
				t.Error(fmt.Sprintf("Test Failed: expected to have an err but didn't get any"))
			}
			var chunksTotalComputationLimit uint64
			for _, chunk := range chunks {
				chunksTotalComputationLimit += chunk.TotalComputationLimit
			}
			if expectedTotalComputationLimit != chunksTotalComputationLimit {
				t.Error(fmt.Sprintf("Test Failed: the chunk total computation limit doesn't match the sum of txs' computation limit."))
			}
			if len(chunks) != testCase.expectedNumberOfChunks {
				t.Error(fmt.Sprintf("Test Failed: expected %v chunks, recieved: %v chunks", testCase.expectedNumberOfChunks, len(chunks)))
			}
		} else {
			if !testCase.expectedError {
				t.Error(fmt.Sprintf("Test Failed: expected not having err but got an error: %v", err.Error()))
			}
		}

	}
}
