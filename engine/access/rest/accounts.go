package rest

import (
	"github.com/onflow/flow-go/access"
)

const ExpandableFieldContracts = "contracts"
const ExpandableFieldKeys = "keys"

func getAccount(r *requestDecorator, backend access.API, link LinkGenerator) (interface{}, StatusError) {
	address, err := toAddress(r.getParam("address"))
	if err != nil {
		return nil, NewBadRequestError("invalid address", err)
	}

	account, err := backend.GetAccount(r.Context(), address)
	if err != nil {
		return nil, NewBadRequestError("account fetching error", err)
	}

	return accountResponse(account), nil
}
