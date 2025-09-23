package routes

import (
	entitiesproto "github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/common"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/http/models"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
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

	var txr *accessmodel.TransactionResult
	// only lookup result if transaction result is to be expanded
	if req.ExpandsResult {
		txr, _, err = backend.GetTransactionResult(
			r.Context(),
			req.ID,
			req.BlockID,
			req.CollectionID,
			entitiesproto.EventEncodingVersion_JSON_CDC_V0,
			optimistic_sync.Criteria{}, // TODO: add support for passing criteria in the request
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
	req, err := request.NewGetTransactionResult(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	txr, executorMetadata, err := backend.GetTransactionResult(
		r.Context(),
		req.ID,
		req.BlockID,
		req.CollectionID,
		entitiesproto.EventEncodingVersion_JSON_CDC_V0,
		models.NewCriteria(req.ExecutionState),
	)
	if err != nil {
		return nil, err
	}

	var response commonmodels.TransactionResult
	response.Build(txr, req.ID, link)

	if req.ExecutionState.IncludeExecutorMetadata {
		response.Metadata = commonmodels.NewMetadata(executorMetadata)
	}

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
