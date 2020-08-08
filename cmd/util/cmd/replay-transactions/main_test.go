package main

import (
	"testing"

	"github.com/dapperlabs/flow-go/model/flow"
)

func test_collapse(t *testing.T) {
	collapsed := collapseByTxID([]flow.TransactionResult{
		{
			TransactionID: [32]byte{1, 2, 3},
			ErrorMessage:  "",
		},
		{
			TransactionID: [32]byte{1, 2, 3},
			ErrorMessage:  "",
		},
		{
			TransactionID: [32]byte{1, 2, 3},
			ErrorMessage:  "",
		},
	})


}
