package routes

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
)

// GetNetworkParameters returns network-wide parameters of the blockchain
func GetNetworkParameters(r *request.Request, backend access.API, _ models.LinkGenerator) (interface{}, error) {
	params := backend.GetNetworkParameters(r.Context())

	var response models.NetworkParameters
	response.Build(&params)
	return response, nil
}
