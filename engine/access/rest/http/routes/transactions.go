package routes

import (
	entitiesproto "github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/common"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
)

// GetTransactionByID gets a transaction by requested ID.
func GetTransactionByID(r *common.Request, backend access.API, link commonmodels.LinkGenerator) (interface{}, error) {
	req, err := request.GetTransactionRequest(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	tx, err := backend.GetTransaction(r.Context(), req.ID)
	if err != nil {
		return nil, err
	}

	var txr *access.TransactionResult
	// only lookup result if transaction result is to be expanded
	if req.ExpandsResult {
		txr, err = backend.GetTransactionResult(
			r.Context(),
			req.ID,
			req.BlockID,
			req.CollectionID,
			entitiesproto.EventEncodingVersion_JSON_CDC_V0,
		)
		if err != nil {
			return nil, err
		}
	}

	var response commonmodels.Transaction
	response.Build(tx, txr, link)
	return response, nil
}

// GetTransactionResultByID retrieves transaction result by the transaction ID.
func GetTransactionResultByID(r *common.Request, backend access.API, link commonmodels.LinkGenerator) (interface{}, error) {
	req, err := request.GetTransactionResultRequest(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	txr, err := backend.GetTransactionResult(
		r.Context(),
		req.ID,
		req.BlockID,
		req.CollectionID,
		entitiesproto.EventEncodingVersion_JSON_CDC_V0,
	)
	if err != nil {
		return nil, err
	}

	var response commonmodels.TransactionResult
	response.Build(txr, req.ID, link)
	return response, nil
}

// CreateTransaction creates a new transaction from provided payload.
func CreateTransaction(r *common.Request, backend access.API, link commonmodels.LinkGenerator) (interface{}, error) {
	req, err := request.CreateTransactionRequest(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	err = backend.SendTransaction(r.Context(), &req.Transaction)
	if err != nil {
		return nil, err
	}

	var response commonmodels.Transaction
	response.Build(&req.Transaction, nil, link)
	return response, nil
}
