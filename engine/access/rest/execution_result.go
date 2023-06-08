package rest

import (
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
)

// GetExecutionResultsByBlockIDs gets Execution Result payload by block IDs.
func GetExecutionResultsByBlockIDs(r *request.Request, srv RestServerApi, link models.LinkGenerator) (interface{}, error) {
	req, err := r.GetExecutionResultByBlockIDsRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	return srv.GetExecutionResultsByBlockIDs(req, r.Context(), link)
}

// GetExecutionResultByID gets execution result by the ID.
func GetExecutionResultByID(r *request.Request, srv RestServerApi, link models.LinkGenerator) (interface{}, error) {
	req, err := r.GetExecutionResultRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	return srv.GetExecutionResultByID(req, r.Context(), link)
}
