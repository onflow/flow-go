package rest

import (
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
)

// GetNodeVersionInfo returns node version information
func GetNodeVersionInfo(r *request.Request, srv RestServerApi, _ models.LinkGenerator) (interface{}, error) {
	return srv.GetNodeVersionInfo(r)
}
