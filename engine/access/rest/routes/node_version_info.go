package routes

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
)

// GetNodeVersionInfo returns node version information
func GetNodeVersionInfo(r *request.Request, backend access.API, _ models.LinkGenerator) (interface{}, error) {
	params, err := backend.GetNodeVersionInfo(r.Context())
	if err != nil {
		return nil, err
	}

	var response models.NodeVersionInfo
	response.Build(params)
	return response, nil
}
