package rest

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/model/flow"
)

const transactionsExpandable = "transactions"

// getCollectionByID retrieves a collection by ID and builds a response
func getCollectionByID(r *request.Request, backend access.API, link LinkGenerator) (interface{}, error) {
	req, err := r.GetCollectionRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
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

	return collectionResponse(collection, transactions, link, r.ExpandFields)
}
