package rest

import (
	"context"
	"fmt"
	"net/http"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
	"github.com/onflow/flow-go/model/flow"
)

const blockIDQueryParam = "block_id"

// getExecutionResultByID gets Execution Result payload by ID
func getExecutionResultsByBlockIDs(req *requestDecorator, backend access.API, link LinkGenerator) (interface{}, error) {

	blockIDs := req.getQueryParams(blockIDQueryParam)
	if len(blockIDs) == 0 {
		return nil, NewBadRequestError(fmt.Errorf("no blocks IDs specified"))
	}

	var executionResults []*generated.ExecutionResult
	for _, id := range blockIDs {
		blkID, err := toID(id)
		if err != nil {
			return nil, NewBadRequestError(err)
		}
		executionResult, err := executionResultLookup(req.Context(), blkID, backend, link)
		if err != nil {
			return nil, NewBadRequestError(err)
		}
		executionResults = append(executionResults, executionResult)
	}
	return executionResults, nil
}

func executionResultLookup(ctx context.Context, id flow.Identifier, backend access.API, linkGenerator LinkGenerator) (*generated.ExecutionResult, error) {
	executionResult, err := backend.GetExecutionResultForBlockID(ctx, id)
	if err != nil {
		return nil, idLookupError(&id, "execution result", err)
	}

	executionResultResp := executionResultResponse(executionResult)
	executionResultResp.Links, err = selfLink(executionResult.ID(), linkGenerator.ExecutionResultLink)
	if err != nil {
		msg := fmt.Sprintf("failed to generate response for execution result ID %s", id.String())
		return nil, NewRestError(http.StatusInternalServerError, msg, err)
	}
	return executionResultResp, nil
}
