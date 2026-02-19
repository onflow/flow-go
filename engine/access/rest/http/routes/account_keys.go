package routes

import (
	"fmt"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/common"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/http/request"

	"github.com/onflow/flow-go/engine/access/rest/http/models"
)

// GetAccountKeyByIndex handler retrieves an account key by address and index and returns the response
func GetAccountKeyByIndex(r *common.Request, backend access.API, _ commonmodels.LinkGenerator) (any, error) {
	req, err := request.GetAccountKeyRequest(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	// In case we receive special height values 'final' and 'sealed',
	// fetch that height and overwrite request with it.
	if req.Height == request.FinalHeight || req.Height == request.SealedHeight {
		isSealed := req.Height == request.SealedHeight
		header, _, err := backend.GetLatestBlockHeader(r.Context(), isSealed)
		if err != nil {
			err := fmt.Errorf("block with height: %d does not exist", req.Height)
			return nil, common.NewNotFoundError(err.Error(), err)
		}
		req.Height = header.Height
	}

	accountKey, err := backend.GetAccountKeyAtBlockHeight(r.Context(), req.Address, req.Index, req.Height)
	if err != nil {
		err = fmt.Errorf("failed to get account key with index: %d, reason: %w", req.Index, err)
		return nil, common.NewNotFoundError(err.Error(), err)
	}

	var response models.AccountPublicKey
	response.Build(*accountKey)
	return response, nil
}

// GetAccountKeys handler retrieves an account keys by address and returns the response
func GetAccountKeys(r *common.Request, backend access.API, _ commonmodels.LinkGenerator) (any, error) {
	req, err := request.GetAccountKeysRequest(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	// In case we receive special height values 'final' and 'sealed',
	// fetch that height and overwrite request with it.
	isSealed := req.Height == request.SealedHeight
	isFinalized := req.Height == request.FinalHeight
	if isFinalized || isSealed {
		header, _, err := backend.GetLatestBlockHeader(r.Context(), isSealed)
		if err != nil {
			err := fmt.Errorf("block with height: %d does not exist", req.Height)
			return nil, common.NewNotFoundError(err.Error(), err)
		}
		req.Height = header.Height
	}

	accountKeys, err := backend.GetAccountKeysAtBlockHeight(r.Context(), req.Address, req.Height)
	if err != nil {
		err = fmt.Errorf("failed to get account keys, reason: %w", err)
		return nil, common.NewNotFoundError(err.Error(), err)
	}

	var response models.AccountPublicKeys
	response.Build(accountKeys)
	return response, nil
}
