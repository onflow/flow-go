package routes

import (
	entitiesproto "github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/common"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

const idQuery = "id"

// GetTransactionByID gets a transaction by requested ID.
// The ID may be either:
//  1. the hex-encoded 32-byte hash of a user-submitted transaction, or
//  2. the integral system-assigned identifier of a scheduled transaction
func GetTransactionByID(r *common.Request, backend access.API, link commonmodels.LinkGenerator) (interface{}, error) {
	if !isTransactionID(r.GetVar(idQuery)) {
		return GetScheduledTransaction(r, backend, link)
	}

	req, err := request.GetTransactionRequest(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	tx, err := backend.GetTransaction(r.Context(), req.ID)
	if err != nil {
		return nil, err
	}

	var txr *accessmodel.TransactionResult
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
// The ID may be either:
//  1. the hex-encoded 32-byte hash of a user-submitted transaction, or
//  2. the integral system-assigned identifier of a scheduled transaction
func GetTransactionResultByID(r *common.Request, backend access.API, link commonmodels.LinkGenerator) (interface{}, error) {
	if !isTransactionID(r.GetVar(idQuery)) {
		return GetScheduledTransactionResult(r, backend, link)
	}

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

// GetScheduledTransaction gets a scheduled transaction by scheduled transaction ID.
func GetScheduledTransaction(r *common.Request, backend access.API, link commonmodels.LinkGenerator) (interface{}, error) {
	req, err := request.NewGetScheduledTransaction(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	tx, err := backend.GetScheduledTransaction(r.Context(), req.ScheduledTxID)
	if err != nil {
		return nil, err
	}

	var txr *accessmodel.TransactionResult
	if req.ExpandsResult {
		txr, err = backend.GetScheduledTransactionResult(r.Context(), req.ScheduledTxID, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
		if err != nil {
			return nil, err
		}
	}

	var response commonmodels.Transaction
	response.Build(tx, txr, link)
	return response, nil
}

// GetScheduledTransactionResult gets a scheduled transaction result by scheduled transaction ID.
func GetScheduledTransactionResult(r *common.Request, backend access.API, link commonmodels.LinkGenerator) (interface{}, error) {
	req, err := request.NewGetScheduledTransactionResult(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	txr, err := backend.GetScheduledTransactionResult(r.Context(), req.ScheduledTxID, entitiesproto.EventEncodingVersion_JSON_CDC_V0)
	if err != nil {
		return nil, err
	}

	var response commonmodels.TransactionResult
	response.Build(txr, txr.TransactionID, link)
	return response, nil
}

// isTransactionID returns true if the provided string is a valid hex-encoded 32-byte flow.Identifier indicating it is a transaction ID.
// In particular, this method returns false if the input is a *scheduled transaction* ID.
func isTransactionID(raw string) bool {
	_, err := flow.HexStringToIdentifier(raw)
	return err == nil
}
