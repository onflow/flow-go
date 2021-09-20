package transactionstate

import "github.com/onflow/flow-go/engine/ghost/client"

type TransactionState struct {
	Reader *client.FlowMessageStreamReader
}

func NewTransactionState() *TransactionState {
	return &TransactionState{}
}
