package routes

import (
	"fmt"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/common"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/http/models"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
)

// GetAccountBalance handler retrieves an account balance by address and block height and returns the response
func GetAccountBalance(r *common.Request, backend access.API, _ commonmodels.LinkGenerator) (any, error) {
	req, err := request.GetAccountBalanceRequest(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	// In case we receive special height values 'final' and 'sealed',
	// fetch that height and overwrite request with it.
	isSealed := req.Height == request.SealedHeight
	isFinal := req.Height == request.FinalHeight
	if isFinal || isSealed {
		header, _, err := backend.GetLatestBlockHeader(r.Context(), isSealed)
		if err != nil {
			err := fmt.Errorf("block with height: %d does not exist", req.Height)
			return nil, common.NewNotFoundError(err.Error(), err)
		}
		req.Height = header.Height
	}

	balance, err := backend.GetAccountBalanceAtBlockHeight(r.Context(), req.Address, req.Height)
	if err != nil {
		err = fmt.Errorf("failed to get account balance, reason: %w", err)
		return nil, common.NewNotFoundError(err.Error(), err)
	}

	var response models.AccountBalance
	response.Build(balance)
	return response, nil
}
