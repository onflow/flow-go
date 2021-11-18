package rest

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
	"github.com/rs/zerolog"
	"net/http"
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

	results := make([]generated.ExecutionResult, len(blockIds))
	for i, id := range blockIds {
		res, err := backend.GetExecutionResultForBlockID(r.Context(), id)
		if err != nil {
			return nil, NewBadRequestError("execution result fetching error", err)
		}
		results[i] = executionResultResponse(*res)
	}

	return results, nil
}
