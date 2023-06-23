package routes

import (
	"github.com/onflow/flow-go/engine/access/rest/api"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
)

const BlockQueryParam = "block_ids"
const EventTypeQuery = "type"

// GetEvents for the provided block range or list of block IDs filtered by type.
func GetEvents(r *request.Request, srv api.RestServerApi, _ models.LinkGenerator) (interface{}, error) {
	req, err := r.GetEventsRequest()
	if err != nil {
		return nil, models.NewBadRequestError(err)
	}

	return srv.GetEvents(req, r.Context())
}
