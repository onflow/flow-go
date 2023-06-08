package rest

import (
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
)

const blockQueryParam = "block_ids"
const eventTypeQuery = "type"

// GetEvents for the provided block range or list of block IDs filtered by type.
func GetEvents(r *request.Request, srv RestServerApi, _ models.LinkGenerator) (interface{}, error) {
	req, err := r.GetEventsRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	return srv.GetEvents(req, r.Context())
}
