package routes

import (
	"github.com/onflow/flow-go/engine/access/rest/api"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
)

// GetNodeVersionInfo returns node version information
func GetNodeVersionInfo(r *request.Request, srv api.RestServerApi, _ models.LinkGenerator) (interface{}, error) {
	return srv.GetNodeVersionInfo(r)
}
