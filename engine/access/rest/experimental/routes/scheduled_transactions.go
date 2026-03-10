package routes

import (
	"net/http"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/access/backends/extended"
	"github.com/onflow/flow-go/engine/access/rest/common"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/experimental/models"
	"github.com/onflow/flow-go/engine/access/rest/experimental/request"
	accessmodel "github.com/onflow/flow-go/model/access"
)

// GetScheduledTransactions handles GET /scheduled.
func GetScheduledTransactions(r *common.Request, backend extended.API, link commonmodels.LinkGenerator) (interface{}, error) {
	req, err := request.NewGetScheduledTransactions(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	page, err := backend.GetScheduledTransactions(
		r.Context(),
		req.Limit,
		req.Cursor,
		req.Filter,
		req.ExpandOptions,
		entities.EventEncodingVersion_JSON_CDC_V0,
	)
	if err != nil {
		return nil, err
	}

	return buildScheduledTransactionsResponse(page, link, r.ExpandFields)
}

// GetScheduledTransaction handles GET /scheduled/transaction/{id}.
func GetScheduledTransaction(r *common.Request, backend extended.API, link commonmodels.LinkGenerator) (interface{}, error) {
	req, err := request.NewGetScheduledTransaction(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	tx, err := backend.GetScheduledTransaction(
		r.Context(),
		req.ID,
		req.ExpandOptions,
		entities.EventEncodingVersion_JSON_CDC_V0,
	)
	if err != nil {
		return nil, err
	}

	var m models.ScheduledTransaction
	err = m.Build(tx, link, r.ExpandFields)
	if err != nil {
		return nil, common.NewRestError(http.StatusInternalServerError, "failed to build scheduled transaction", err)
	}
	return m, nil
}

// GetScheduledTransactionsByAddress handles GET /accounts/{address}/scheduled.
func GetScheduledTransactionsByAddress(r *common.Request, backend extended.API, link commonmodels.LinkGenerator) (interface{}, error) {
	req, err := request.NewGetScheduledTransactionsByAddress(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	page, err := backend.GetScheduledTransactionsByAddress(
		r.Context(),
		req.Address,
		req.Limit,
		req.Cursor,
		req.Filter,
		req.ExpandOptions,
		entities.EventEncodingVersion_JSON_CDC_V0,
	)
	if err != nil {
		return nil, err
	}

	return buildScheduledTransactionsResponse(page, link, r.ExpandFields)
}

// buildScheduledTransactionsResponse converts a [accessmodel.ScheduledTransactionsPage] to a REST
// response, encoding the next cursor if present.
func buildScheduledTransactionsResponse(
	page *accessmodel.ScheduledTransactionsPage,
	link commonmodels.LinkGenerator,
	expandMap map[string]bool,
) (models.ScheduledTransactionsResponse, error) {
	scheduledTransactions := make([]models.ScheduledTransaction, len(page.Transactions))
	for i := range page.Transactions {
		err := scheduledTransactions[i].Build(&page.Transactions[i], link, expandMap)
		if err != nil {
			return models.ScheduledTransactionsResponse{}, common.NewRestError(http.StatusInternalServerError, "failed to build scheduled transaction", err)
		}
	}

	var nextCursor string
	if page.NextCursor != nil {
		var err error
		nextCursor, err = request.EncodeScheduledTxCursor(page.NextCursor)
		if err != nil {
			return models.ScheduledTransactionsResponse{}, common.NewRestError(http.StatusInternalServerError, "failed to encode next cursor", err)
		}
	}

	return models.ScheduledTransactionsResponse{
		ScheduledTransactions: scheduledTransactions,
		NextCursor:            nextCursor,
	}, nil
}
