package routes

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/common"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/http/models"
)

// GetNodeVersionInfo returns node version information
func GetNodeVersionInfo(r *common.Request, backend access.API, _ commonmodels.LinkGenerator) (any, error) {
	params, err := backend.GetNodeVersionInfo(r.Context())
	if err != nil {
		return nil, err
	}

	var response models.NodeVersionInfo
	response.Build(params)
	return response, nil
}
