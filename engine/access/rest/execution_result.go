package rest

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
)

const blockIDQueryParam = "block_id"

// getExecutionResultByID gets Execution Result payload by block IDs.
func getExecutionResultsByBlockIDs(r *Request, backend access.API, link LinkGenerator) (interface{}, error) {
	req, err := r.getExecutionResultByBlockIDsRequest()
	if err != nil {
		return nil, err
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
func getExecutionResultByID(r *Request, backend access.API, link LinkGenerator) (interface{}, error) {
	req, err := r.getExecutionResultRequest()
	if err != nil {
		return nil, err
	}

	res, err := backend.GetExecutionResultByID(r.Context(), req.ID)
	if err != nil {
		return nil, err
	}

	return executionResultResponse(res, link)
}
