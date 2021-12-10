package rest

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/model/flow"
)

const transactionsExpandable = "transactions"

// getCollectionByID retrieves a collection by ID and builds a response
func getCollectionByID(r *request, backend access.API, link LinkGenerator) (interface{}, error) {
	id, err := r.id()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	collection, err := backend.GetCollectionByID(r.Context(), id)
	if err != nil {
		return nil, err
	}

	// if we expand transactions in the query retrieve each transaction data
	transactions := make([]*flow.TransactionBody, 0)
	if r.expands(transactionsExpandable) {
		for _, tid := range collection.Transactions {
			tx, err := backend.GetTransaction(r.Context(), tid)
			if err != nil {
				return nil, err
			}

			transactions = append(transactions, tx)
		}
	}

	return collectionResponse(collection, transactions, link, r.expandFields)
}
