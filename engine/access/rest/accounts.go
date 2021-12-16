package rest

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/request"
)

const blockHeightQueryParam = "block_height"

// getAccount handler retrieves account by address and returns the response
func getAccount(r *Request, backend access.API, link LinkGenerator) (interface{}, error) {
	req, err := r.getAccountRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	// in case we provide special height values 'final' and 'sealed' fetch that height and overwrite request wtih it
	if req.Height == request.FinalHeight || req.Height == request.SealedHeight {
		header, err := backend.GetLatestBlockHeader(r.Context(), req.Height == request.SealedHeight)
		if err != nil {
			return nil, err
		}
		req.Height = header.Height
	}

	account, err := backend.GetAccountAtBlockHeight(r.Context(), req.Address, req.Height)
	if err != nil {
		return nil, err
	}

	return accountResponse(account, link, r.expandFields)
}
