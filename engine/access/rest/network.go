package rest

import (
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
)

// GetNetworkParameters returns network-wide parameters of the blockchain
func GetNetworkParameters(r *request.Request, srv RestServerApi, _ models.LinkGenerator) (interface{}, error) {
	return srv.GetNetworkParameters(r)
}
