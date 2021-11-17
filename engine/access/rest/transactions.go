package rest

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
	"github.com/rs/zerolog"
	"net/http"
)

// getTransactionByID gets a transaction by requested ID.
func getTransactionByID(
	w http.ResponseWriter,
	r *http.Request,
	vars map[string]string,
	backend access.API,
	logger zerolog.Logger,
) (interface{}, StatusError) {

	id, err := toID(vars["id"])
	if err != nil {
		return nil, NewBadRequestError("invalid ID", err)
	}

	tx, err := backend.GetTransaction(r.Context(), id)
	if err != nil {
		return nil, NewBadRequestError("transaction fetching error", err)
	}

	return transactionResponse(tx), nil
}

// createTransaction creates a new transaction from provided payload.
func createTransaction(
	w http.ResponseWriter,
	r *http.Request,
	vars map[string]string,
	backend access.API,
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

	return tx, nil
}
