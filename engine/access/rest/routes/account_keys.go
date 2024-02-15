package routes

import (
	"fmt"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
)

// GetAccountKeyByIndex handler retrieves an account key by address and index and returns the response
func GetAccountKeyByIndex(r *request.Request, backend access.API, link models.LinkGenerator) (interface{}, error) {
	req, err := r.GetAccountKeyRequest()
	if err != nil {
		return nil, models.NewBadRequestError(err)
	}

	// In case we receive special height values 'final' and 'sealed',
	// fetch that height and overwrite request with it.
	if req.Height == request.FinalHeight || req.Height == request.SealedHeight {
		isSealed := req.Height == request.SealedHeight
		header, _, err := backend.GetLatestBlockHeader(r.Context(), isSealed)
		if err != nil {
			err := fmt.Errorf("block with height: %d does not exist", req.Height)
			return nil, models.NewNotFoundError(err.Error(), err)
		}
		req.Height = header.Height
	}

	account, err := backend.GetAccountAtBlockHeight(r.Context(), req.Address, req.Height)
	if err != nil {
		err := fmt.Errorf("account with address: %s does not exist", req.Address)
		return nil, models.NewNotFoundError(err.Error(), err)
	}

	var response models.AccountPublicKey
	for _, key := range account.Keys {
		if key.Index == int(req.Index) {
			response.Build(key)
			return response, nil
		}
	}

	err = fmt.Errorf("account key with index: %d does not exist", req.Index)
	return nil, models.NewNotFoundError(err.Error(), err)
}
