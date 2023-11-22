package routes

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/model/flow"
)

// GetCollectionByID retrieves a collection by ID and builds a response
func GetCollectionByID(r *request.Request, backend access.API, link models.LinkGenerator) (interface{}, error) {
	req, err := r.GetCollectionRequest()
	if err != nil {
		return nil, models.NewBadRequestError(err)
	}

	collection, err := backend.GetCollectionByID(r.Context(), req.ID)
	if err != nil {
		return nil, err
	}

	// if we expand transactions in the query retrieve each transaction data
	transactions := make([]*flow.TransactionBody, 0)
	if req.ExpandsTransactions {
		for _, tid := range collection.Transactions {
			tx, err := backend.GetTransaction(r.Context(), tid)
			if err != nil {
				return nil, err
			}

			transactions = append(transactions, tx)
		}
	}

	var response models.Collection
	err = response.Build(collection, transactions, link, r.ExpandFields)
	if err != nil {
		return nil, err
	}

	return response, nil
}
