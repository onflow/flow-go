package routes

import (
	"net/http"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/access/backends/extended"
	"github.com/onflow/flow-go/engine/access/rest/common"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/experimental/models"
	"github.com/onflow/flow-go/engine/access/rest/experimental/request"
)

// GetAccountFungibleTokenTransfers returns a paginated list of fungible token transfers for the given account address.
func GetAccountFungibleTokenTransfers(r *common.Request, backend extended.API, link commonmodels.LinkGenerator) (interface{}, error) {
	req, err := request.NewGetAccountFTTransfers(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	expandOptions := extended.AccountTransferExpandOptions{
		Transaction: r.Expands("transaction"),
		Result:      r.Expands("result"),
	}

	page, err := backend.GetAccountFungibleTokenTransfers(r.Context(), req.Address, req.Limit, req.Cursor, req.Filter, expandOptions, entities.EventEncodingVersion_JSON_CDC_V0)
	if err != nil {
		return nil, err
	}

	resp := models.AccountFungibleTransfersResponse{
		Transfers: make([]models.FungibleTokenTransfer, len(page.Transfers)),
	}
	for i := range page.Transfers {
		err := resp.Transfers[i].Build(&page.Transfers[i], link)
		if err != nil {
			return nil, common.NewRestError(http.StatusInternalServerError, "failed to build transfer", err)
		}
	}
	if page.NextCursor != nil {
		resp.NextCursor, err = request.EncodeTransferCursor(page.NextCursor)
		if err != nil {
			return nil, common.NewRestError(http.StatusInternalServerError, "failed to encode next cursor", err)
		}
	}

	return resp, nil
}
