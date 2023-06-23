package routes

import (
	"github.com/onflow/flow-go/engine/access/rest/api"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
)

// ExecuteScript handler sends the script from the request to be executed.
func ExecuteScript(r *request.Request, srv api.RestServerApi, link models.LinkGenerator) (interface{}, error) {
	req, err := r.GetScriptRequest()
	if err != nil {
		return nil, models.NewBadRequestError(err)
	}

	return srv.ExecuteScript(req, r.Context(), link)
}
