package routes

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/common"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/http/models"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
)

// GetAccount handler retrieves account by address and returns the response
func GetAccount(r *common.Request, backend access.API, link commonmodels.LinkGenerator) (interface{}, error) {
	req, err := request.NewGetAccountRequest(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	// in case we receive special height values 'final' and 'sealed', fetch that height and overwrite request with it
	if req.Height == request.FinalHeight || req.Height == request.SealedHeight {
		header, _, err := backend.GetLatestBlockHeader(r.Context(), req.Height == request.SealedHeight)
		if err != nil {
			return nil, common.ErrorToStatusError(err)
		}
		req.Height = header.Height
	}

	executionState := req.ExecutionState
	account, executorMetadata, err := backend.GetAccountAtBlockHeight(r.Context(), req.Address, req.Height, models.NewCriteria(executionState))
	if err != nil {
		return nil, common.ErrorToStatusError(err)
	}

	return models.NewAccount(account, link, r.ExpandFields, executorMetadata, executionState.IncludeExecutorMetadata)
}
