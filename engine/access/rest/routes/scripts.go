package routes

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/model/flow"
)

// ExecuteScript handler sends the script from the request to be executed.
func ExecuteScript(r *request.Request, backend access.API, _ models.LinkGenerator) (interface{}, error) {
	req, err := r.GetScriptRequest()
	if err != nil {
		return nil, models.NewBadRequestError(err)
	}

	if req.BlockID != flow.ZeroID {
		return backend.ExecuteScriptAtBlockID(r.Context(), req.BlockID, req.Script.Source, req.Script.Args)
	}

	// default to sealed height
	if req.BlockHeight == request.SealedHeight || req.BlockHeight == request.EmptyHeight {
		return backend.ExecuteScriptAtLatestBlock(r.Context(), req.Script.Source, req.Script.Args)
	}

	if req.BlockHeight == request.FinalHeight {
		finalBlock, _, err := backend.GetLatestBlockHeader(r.Context(), false)
		if err != nil {
			return nil, err
		}
		req.BlockHeight = finalBlock.Height
	}

	return backend.ExecuteScriptAtBlockHeight(r.Context(), req.BlockHeight, req.Script.Source, req.Script.Args)
}
