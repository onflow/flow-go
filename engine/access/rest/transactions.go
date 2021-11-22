package rest

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
)

// getTransactionByID gets a transaction by requested ID.
func getTransactionByID(req Request, backend access.API) (interface{}, StatusError) {

	id, err := toID(req.getParam("id"))
	if err != nil {
		return nil, NewBadRequestError("invalid ID", err)
	}

	tx, err := backend.GetTransaction(req.context, id)
	if err != nil {
		return nil, NewBadRequestError("transaction fetching error", err)
	}

	return transactionResponse(tx), nil
}

func getTransactionResultByID(
	r *requestDecorator,
	backend access.API,
	generator LinkGenerator,
	logger zerolog.Logger,
) (interface{}, StatusError) {
	id, err := r.id()
	if err != nil {
		return nil, NewBadRequestError("invalid ID", err)
	}

	txr, err := backend.GetTransactionResult(r.Context(), id)
	if err != nil {
		return nil, NewBadRequestError("transaction result fetching error", err)
	}

	return transactionResultResponse(txr), nil
}

// createTransaction creates a new transaction from provided payload.
func createTransaction(
	r *requestDecorator,
	backend access.API,
	generator LinkGenerator,
	logger zerolog.Logger,
) (interface{}, StatusError) {

	var txBody generated.TransactionsBody
	err := jsonDecode(r.Body, &txBody)
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

	return transactionResponse(&tx), nil
}
