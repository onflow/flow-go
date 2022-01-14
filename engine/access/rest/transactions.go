package rest

import (
	"fmt"
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
)

// GetTransactionByID gets a transaction by requested ID.
func GetTransactionByID(r *request.Request, backend access.API, link models.LinkGenerator) (interface{}, error) {
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

// GetTransactionResultByID retrieves transaction result by the transaction ID.
func GetTransactionResultByID(r *request.Request, backend access.API, link models.LinkGenerator) (interface{}, error) {
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

// CreateTransaction creates a new transaction from provided payload.
func CreateTransaction(r *request.Request, backend access.API, link models.LinkGenerator) (interface{}, error) {
	req, err := r.CreateTransactionRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	err = backend.SendTransaction(r.Context(), &req.Transaction)
	if err != nil {
		return nil, err
	}

	fmt.Println(req.Transaction.EnvelopeSignatures)
	fmt.Println(req.Transaction.EnvelopeSignatures[0].Signature)
	fmt.Println(string(req.Transaction.EnvelopeSignatures[0].Signature))
	fmt.Printf("\n%x\n", req.Transaction.EnvelopeSignatures[0].Signature)

	var response models.Transaction
	response.Build(&req.Transaction, nil, link)
	return response, nil
}
