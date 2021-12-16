package rest

import (
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow-go/access"
)

// executeScript handler sends the script from the request to be executed.
func executeScript(r *request.Request, backend access.API, _ LinkGenerator) (interface{}, error) {
	req, err := r.GetScriptRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	if req.BlockID != flow.ZeroID {
		return backend.ExecuteScriptAtBlockID(r.Context(), req.BlockID, req.Script.Source, req.Script.Args)
	}

	if req.BlockHeight == request.SealedHeight {
		result, err := backend.ExecuteScriptAtLatestBlock(r.Context(), req.Script.Source, req.Script.Args)
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	if req.BlockHeight == request.FinalHeight {
		finalBlock, err := backend.GetLatestBlockHeader(r.Context(), false)
		if err != nil {
			return nil, err
		}
		req.BlockHeight = finalBlock.Height
	}

	return backend.ExecuteScriptAtBlockHeight(r.Context(), req.BlockHeight, req.Script.Source, req.Script.Args)
}
