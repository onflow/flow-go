package routes

import (
	"fmt"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
)

// GetAccountBalance handler retrieves an account balance by address and block height and returns the response
func GetAccountBalance(r *request.Request, backend access.API, _ models.LinkGenerator) (interface{}, error) {
	req, err := r.GetAccountBalanceRequest()
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

	balance, err := backend.GetAccountBalanceAtBlockHeight(r.Context(), req.Address, req.Height)
	if err != nil {
		err := fmt.Errorf("account with address: %s does not exist", req.Address)
		return nil, models.NewNotFoundError(err.Error(), err)
	}

	var accountBalance models.Block

	//var response models.AccountPublicKey
	//for _, key := range account.Keys {
	//	if key.Index == req.Index {
	//		response.Build(key)
	//		return response, nil
	//	}
	//}
	//
	//err = fmt.Errorf("account key with index: %d does not exist", req.Index)
	//return nil, models.NewNotFoundError(err.Error(), err)
}
