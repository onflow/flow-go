package handlers

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/engine/access/rest/util"
)

// getTransactionByID gets a transaction by requested ID.
func getTransactionByID(r *request.Request, backend access.API, link util.LinkGenerator) (interface{}, error) {
	req, err := r.GetTransactionRequest()
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

	var response models.Transaction
	response.Build(tx, txr, link)
	return response, nil
}

// getTransactionResultByID retrieves transaction result by the transaction ID.
func getTransactionResultByID(r *request.Request, backend access.API, link util.LinkGenerator) (interface{}, error) {
	req, err := r.GetTransactionResultRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	txr, err := backend.GetTransactionResult(r.Context(), req.ID)
	if err != nil {
		return nil, err
	}

	var response models.TransactionResult
	response.Build(txr, req.ID, link)
	return response, nil
}

// createTransaction creates a new transaction from provided payload.
func createTransaction(r *request.Request, backend access.API, link util.LinkGenerator) (interface{}, error) {
	req, err := r.CreateTransactionRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	err = backend.SendTransaction(r.Context(), &req.Transaction)
	if err != nil {
		return nil, err
	}

	var response models.Transaction
	response.Build(&req.Transaction, nil, link)
	return response, nil
}
