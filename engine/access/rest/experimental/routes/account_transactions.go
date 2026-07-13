package routes

import (
	"net/http"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/access/backends/extended"
	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/experimental/models"
	"github.com/onflow/flow-go/engine/access/rest/experimental/request"
)

// GetAccountTransactions returns a paginated list of transactions for the given account address.
func GetAccountTransactions(r *common.Request, backend extended.API, link models.LinkGenerator) (interface{}, error) {
	req, err := request.NewGetAccountTransactions(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	expandOptions := extended.AccountTransactionExpandOptions{
		Transaction: r.Expands("transaction"),
		Result:      r.Expands("result"),
	}

	page, err := backend.GetAccountTransactions(r.Context(), req.Address, req.Limit, req.Cursor, req.Filter, expandOptions, entities.EventEncodingVersion_JSON_CDC_V0)
	if err != nil {
		return nil, err
	}

	resp := models.AccountTransactionsResponse{
		Transactions: make([]models.AccountTransaction, len(page.Transactions)),
	}
	for i := range page.Transactions {
		err := resp.Transactions[i].Build(&page.Transactions[i], link)
		if err != nil {
			return nil, common.NewRestError(http.StatusInternalServerError, "failed to build transaction", err)
		}
	}
	if page.NextCursor != nil {
		resp.NextCursor, err = request.EncodeAccountTransactionCursor(page.NextCursor)
		if err != nil {
			return nil, common.NewRestError(http.StatusInternalServerError, "failed to encode next cursor", err)
		}
	}

	return resp, nil
}
