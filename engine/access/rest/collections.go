package rest

import (
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
)

// GetCollectionByID retrieves a collection by ID and builds a response
func GetCollectionByID(r *request.Request, srv RestServerApi, link models.LinkGenerator) (interface{}, error) {
	req, err := r.GetCollectionRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	return srv.GetCollectionByID(req, r.Context(), r.ExpandFields, link, r.Chain)
}
