package routes

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/common"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
	"github.com/onflow/flow-go/model/flow"
)

// GetCollectionByID retrieves a light collection by ID and builds a response. The light collection
// contains only transaction IDs, not the full transaction bodies. If the expand query parameter
// includes "transactions", the full transaction bodies are fetched and included in the response.
func GetCollectionByID(r *common.Request, backend access.API, link commonmodels.LinkGenerator) (any, error) {
	req, err := request.GetCollectionRequest(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	collection, err := backend.GetCollectionByID(r.Context(), req.ID)
	if err != nil {
		return nil, common.ErrorToStatusError(err)
	}

	// if we expand transactions in the query retrieve each transaction data
	transactions := make([]*flow.TransactionBody, 0)
	if req.ExpandsTransactions {
		for _, tid := range collection.Transactions {
			tx, err := backend.GetTransaction(r.Context(), tid)
			if err != nil {
				// TODO: transactions is not updated to use the new error handling convention
				// Update this once that work is complete
				return nil, err
			}

			transactions = append(transactions, tx)
		}
	}

	var response commonmodels.Collection
	err = response.Build(collection, transactions, link, r.ExpandFields)
	if err != nil {
		// response.Build only returns errors from the link generator (router.URLPath)
		// which are internal configuration errors, not accessSentinel errors from the backend
		// the ErrorHandler's catch-all will convert this to 500 Internal Server Error.
		return nil, err
	}

	return response, nil
}
