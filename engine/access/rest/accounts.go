package rest

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/model/flow"
)

func getAccount(r *requestDecorator, backend access.API, link LinkGenerator) (interface{}, error) {
	address, err := toAddress(r.getVar("address"))
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	var account *flow.Account
	height := r.getQueryParam("height")
	if height == "latest" || height == "" {
		account, err = backend.GetAccountAtLatestBlock(r.Context(), address)
		if err != nil {
			return nil, err
		}
	} else {
		h, err := toHeight(height)
		if err != nil {
			return nil, NewBadRequestError(err)
		}
		account, err = backend.GetAccountAtBlockHeight(r.Context(), address, h)
		if err != nil {
			return nil, err
		}
	}

	return accountResponse(account), nil
}
