package rest

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
)

const transactionResult = "result"

// getTransactionByID gets a transaction by requested ID.
func getTransactionByID(r *requestDecorator, backend access.API, link LinkGenerator) (interface{}, error) {
	id, err := r.id()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	tx, err := backend.GetTransaction(r.Context(), id)
	if err != nil {
		return nil, err
	}

	if r.expands(transactionResult) {
		txr, err := backend.GetTransactionResult(r.Context(), id)
		if err != nil {
			return nil, err
		}

		return transactionResponse(tx, txr, link), nil
	}

	return transactionResponse(tx, nil, link), nil
}

func getTransactionResultByID(r *requestDecorator, backend access.API, link LinkGenerator) (interface{}, error) {
	id, err := r.id()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	txr, err := backend.GetTransactionResult(r.Context(), id)
	if err != nil {
		return nil, err
	}

	return transactionResultResponse(txr, id, link), nil
}

// createTransaction creates a new transaction from provided payload.
func createTransaction(r *requestDecorator, backend access.API, link LinkGenerator) (interface{}, error) {
	var txBody generated.TransactionsBody
	err := r.bodyAs(&txBody)
	if err != nil {
		return nil, err
	}

	tx, err := toTransaction(txBody)
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	err = backend.SendTransaction(r.Context(), &tx)
	if err != nil {
		return nil, err
	}

	return transactionResponse(&tx, nil, link), nil
}
