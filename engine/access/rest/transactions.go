package rest

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
)

// getTransactionByID gets a transaction by requested ID.
func getTransactionByID(r *requestDecorator, backend access.API, link LinkGenerator) (interface{}, StatusError) {

	id, err := r.id()
	if err != nil {
		return nil, NewBadRequestError("invalid ID", err)
	}

	tx, err := backend.GetTransaction(r.Context(), id)
	if err != nil {
		return nil, NewBadRequestError("transaction result fetching error", err)
	}

	return transactionResponse(tx, link), nil
}

func getTransactionResultByID(r *requestDecorator, backend access.API, link LinkGenerator) (interface{}, StatusError) {
	id, err := r.id()
	if err != nil {
		return nil, NewBadRequestError("invalid ID", err)
	}

	txr, err := backend.GetTransactionResult(r.Context(), id)
	if err != nil {
		return nil, NewBadRequestError("transaction result fetching error", err)
	}

	return transactionResultResponse(txr, id, link), nil
}

// createTransaction creates a new transaction from provided payload.
func createTransaction(r *requestDecorator, backend access.API, link LinkGenerator) (interface{}, StatusError) {

	var txBody generated.TransactionsBody
	err := r.bodyAs(&txBody)
	if err != nil {
		return nil, NewBadRequestError("invalid transaction request", err)
	}

	tx, err := toTransaction(&txBody)
	if err != nil {
		return nil, NewBadRequestError("invalid transaction request", err)
	}

	err = backend.SendTransaction(r.Context(), &tx)
	if err != nil {
		return nil, NewBadRequestError("failed to send transaction", err)
	}

	return transactionResponse(&tx, link), nil
}
