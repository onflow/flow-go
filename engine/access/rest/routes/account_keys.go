package routes

import (
	"fmt"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/model/flow"
)

// GetAccountKeyByID handler retrieves an account key by address and ID and returns the response
func GetAccountKeyByID(r *request.Request, backend access.API, link models.LinkGenerator) (interface{}, error) {
	req, err := r.GetAccountKeyRequest()
	if err != nil {
		return nil, models.NewBadRequestError(err)
	}

	header, _, err := backend.GetLatestBlockHeader(r.Context(), false)
	if err != nil {
		return nil, err
	}

	account, err := backend.GetAccountAtBlockHeight(r.Context(), req.Address, header.Height)
	if err != nil {
		return nil, err
	}

	var accountKey flow.AccountPublicKey
	found := false
	for _, key := range account.Keys {
		if key.Index == int(req.KeyID) {
			accountKey = key
			found = true
		}
	}
	if !found {
		return nil, models.NewNotFoundError(
			fmt.Sprintf("account key with ID: %d does not exist", req.KeyID),
			nil,
		)
	}

	var response models.AccountPublicKey
	response.Build(accountKey)
	return response, nil
}
