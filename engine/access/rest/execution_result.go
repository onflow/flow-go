package rest

import (
	"context"
	"fmt"
	"net/http"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
	"github.com/onflow/flow-go/model/flow"
)

// getExecutionResultByID gets Execution Result payload by ID
func getExecutionResultByID(req *requestDecorator, backend access.API, link LinkGenerator) (interface{}, StatusError) {

	id, err := req.id()
	if err != nil {
		return nil, NewBadRequestError(err.Error(), err)
	}

	executionResult, err := executionResultLookup(req.Context(), id, backend, link)
	if err != nil {
		msg := fmt.Sprintf("failed to generate response for execution result ID %s", id.String())
		return nil, NewRestError(http.StatusInternalServerError, msg, err)
	}
	return executionResult, nil
}

func executionResultLookup(ctx context.Context, id flow.Identifier, backend access.API, linkGenerator LinkGenerator) (*generated.ExecutionResult, StatusError) {
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
