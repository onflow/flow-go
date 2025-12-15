package routes

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/common"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/http/models"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
)

// GetAccountBalance handler retrieves an account balance by address and block height and returns the response
func GetAccountBalance(r *common.Request, backend access.API, _ commonmodels.LinkGenerator) (interface{}, error) {
	req, err := request.NewGetAccountBalanceRequest(r)
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
			return nil, common.ErrorToStatusError(err)
		}
		req.Height = header.Height
	}

	executionState := req.ExecutionState
	balance, executorMetadata, err := backend.GetAccountBalanceAtBlockHeight(r.Context(), req.Address, req.Height, models.NewCriteria(executionState))
	if err != nil {
		return nil, common.ErrorToStatusError(err)
	}

	response := models.NewAccountBalance(balance, executorMetadata, executionState.IncludeExecutorMetadata)
	return response, nil
}
