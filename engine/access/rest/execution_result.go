package rest

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
	"github.com/onflow/flow-go/engine/access/rest/request"
)

// getExecutionResultByID gets Execution Result payload by block IDs.
func getExecutionResultsByBlockIDs(r *request.Request, backend access.API, link LinkGenerator) (interface{}, error) {
	req, err := r.GetExecutionResultByBlockIDsRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	// for each block ID we retrieve execution result
	results := make([]*generated.ExecutionResult, len(req.BlockIDs))
	for i, id := range req.BlockIDs {
		res, err := backend.GetExecutionResultForBlockID(r.Context(), id)
		if err != nil {
			return nil, err
		}
		results[i], err = executionResultResponse(res, link)
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}

// getExecutionResultByID gets execution result by the ID.
func getExecutionResultByID(r *request.Request, backend access.API, link LinkGenerator) (interface{}, error) {
	req, err := r.GetExecutionResultRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	res, err := backend.GetExecutionResultByID(r.Context(), req.ID)
	if err != nil {
		return nil, err
	}

	return executionResultResponse(res, link)
}
