package rest

import (
	"github.com/onflow/flow-go/access"
)

const resultExpandable = "result"

// getTransactionByID gets a transaction by requested ID.
func getTransactionByID(r *Request, backend access.API, link LinkGenerator) (interface{}, error) {
	req, err := r.getTransactionRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	tx, err := backend.GetTransaction(r.Context(), req.ID)
	if err != nil {
		return nil, err
	}

	var txr *access.TransactionResult
	// only lookup result if transaction result is to be expanded
	if req.ExpandsResult {
		txr, err = backend.GetTransactionResult(r.Context(), req.ID)
		if err != nil {
			return nil, err
		}
	}

	return transactionResponse(tx, txr, link), nil
}

// getTransactionResultByID retrieves transaction result by the transaction ID.
func getTransactionResultByID(r *Request, backend access.API, link LinkGenerator) (interface{}, error) {
	req, err := r.getTransactionResultRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	txr, err := backend.GetTransactionResult(r.Context(), req.ID)
	if err != nil {
		return nil, err
	}

	return transactionResultResponse(txr, req.ID, link), nil
}

// createTransaction creates a new transaction from provided payload.
func createTransaction(r *Request, backend access.API, link LinkGenerator) (interface{}, error) {
	req, err := r.createTransactionRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	err = backend.SendTransaction(r.Context(), &req.Transaction)
	if err != nil {
		return nil, err
	}

	return transactionResponse(&req.Transaction, nil, link), nil
}
