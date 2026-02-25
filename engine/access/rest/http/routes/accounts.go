package routes

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/common"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/http/models"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
)

// GetAccount handler retrieves account by address and returns the response
func GetAccount(r *common.Request, backend access.API, link commonmodels.LinkGenerator) (any, error) {
	req, err := request.GetAccountRequest(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	// in case we receive special height values 'final' and 'sealed', fetch that height and overwrite request with it
	if req.Height == request.FinalHeight || req.Height == request.SealedHeight {
		header, _, err := backend.GetLatestBlockHeader(r.Context(), req.Height == request.SealedHeight)
		if err != nil {
			return nil, err
		}
		req.Height = header.Height
	}

	account, err := backend.GetAccountAtBlockHeight(r.Context(), req.Address, req.Height)
	if err != nil {
		return nil, err
	}

	var response models.Account
	err = response.Build(account, link, r.ExpandFields)
	return response, err
}
