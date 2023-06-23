package routes

import (
	"github.com/onflow/flow-go/engine/access/rest/api"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
)

// GetTransactionByID gets a transaction by requested ID.
func GetTransactionByID(r *request.Request, srv api.RestServerApi, link models.LinkGenerator) (interface{}, error) {
	req, err := r.GetTransactionRequest()
	if err != nil {
		return nil, models.NewBadRequestError(err)
	}

	return srv.GetTransactionByID(req, r.Context(), link, r.Chain)
}

// GetTransactionResultByID retrieves transaction result by the transaction ID.
func GetTransactionResultByID(r *request.Request, srv api.RestServerApi, link models.LinkGenerator) (interface{}, error) {
	req, err := r.GetTransactionResultRequest()
	if err != nil {
		return nil, models.NewBadRequestError(err)
	}

	return srv.GetTransactionResultByID(req, r.Context(), link)
}

// CreateTransaction creates a new transaction from provided payload.
func CreateTransaction(r *request.Request, srv api.RestServerApi, link models.LinkGenerator) (interface{}, error) {
	req, err := r.CreateTransactionRequest()
	if err != nil {
		return nil, models.NewBadRequestError(err)
	}

	return srv.CreateTransaction(req, r.Context(), link)
}
