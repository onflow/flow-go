package blueprints

import "github.com/onflow/flow-go/model/flow"

func ProcessCallbacksTransaction(chainID flow.Chain) (*flow.TransactionBody, error) {
	tx := flow.NewTransactionBody()

	return tx, nil
}

func ExecuteCallbacksTransactions(chainID flow.Chain, processEvents flow.EventsList) ([]*flow.TransactionBody, error) {
	return nil, nil
}
