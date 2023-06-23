package routes

import (
	"github.com/onflow/flow-go/engine/access/rest/api"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
)

// GetAccount handler retrieves account by address and returns the response
func GetAccount(r *request.Request, srv api.RestServerApi, link models.LinkGenerator) (interface{}, error) {
	req, err := r.GetAccountRequest()
	if err != nil {
		return nil, models.NewBadRequestError(err)
	}

	return srv.GetAccount(req, r.Context(), r.ExpandFields, link)
}
