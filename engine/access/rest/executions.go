package rest

import (
	"net/http"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
)

func getExecutionResultByBlockIDs(
	w http.ResponseWriter,
	r *http.Request,
	vars map[string]string,
	backend access.API,
	logger zerolog.Logger,
) (interface{}, StatusError) {
	blockIds, err := toIDs(vars["block_id"])
	if err != nil {
		return nil, NewBadRequestError("invalid IDs", err)
	}

	results := make([]*generated.ExecutionResult, len(blockIds))
	for i, id := range blockIds {
		res, err := backend.GetExecutionResultForBlockID(r.Context(), id)
		if err != nil {
			return nil, NewBadRequestError("execution result fetching error", err)
		}
		results[i] = executionResultResponse(res)
	}

	return results, nil
}
