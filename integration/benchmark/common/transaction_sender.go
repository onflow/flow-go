package common

import flowsdk "github.com/onflow/flow-go-sdk"

type TransactionSender interface {
	// Send sends a transaction to the network.
	// It blocks until the transaction result is received or an error occurs.
	// If the transaction execution fails, the returned error type is TransactionError.
	Send(tx *flowsdk.Transaction) (flowsdk.TransactionResult, error)
}
