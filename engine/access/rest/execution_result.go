package rest

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/engine/access/rest/util"
)

// getExecutionResultByID gets Execution Result payload by block IDs.
func getExecutionResultsByBlockIDs(r *request.Request, backend access.API, link util.LinkGenerator) (interface{}, error) {
	req, err := r.GetExecutionResultByBlockIDsRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	// for each block ID we retrieve execution result
	results := make([]models.ExecutionResult, len(req.BlockIDs))
	for i, id := range req.BlockIDs {
		res, err := backend.GetExecutionResultForBlockID(r.Context(), id)
		if err != nil {
			return nil, err
		}

		var response models.ExecutionResult
		err = response.Build(res, link)
		if err != nil {
			return nil, err
		}
		results[i] = response
	}

	return results, nil
}

// getExecutionResultByID gets execution result by the ID.
func getExecutionResultByID(r *request.Request, backend access.API, link util.LinkGenerator) (interface{}, error) {
	req, err := r.GetExecutionResultRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	res, err := backend.GetExecutionResultByID(r.Context(), req.ID)
	if err != nil {
		return nil, err
	}

	var response models.ExecutionResult
	err = response.Build(res, link)
	if err != nil {
		return nil, err
	}

	return response, nil
}
