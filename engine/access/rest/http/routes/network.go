package routes

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/common"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/http/models"
)

// GetNetworkParameters returns network-wide parameters of the blockchain
func GetNetworkParameters(r *common.Request, backend access.API, _ commonmodels.LinkGenerator) (any, error) {
	params := backend.GetNetworkParameters(r.Context())

	var response models.NetworkParameters
	response.Build(&params)
	return response, nil
}
