package rest

import (
	"github.com/onflow/flow-go/engine/access/rest/models"

	"github.com/onflow/flow-go/access"
)

// executeScript handler sends the script from the request to be executed.
func executeScript(r *Request, backend access.API, _ LinkGenerator) (interface{}, error) {
	req, err := r.getScriptRequest()
	if err != nil {
		return nil, err
	}

	if len(req.BlockID) > 0 {
		return backend.ExecuteScriptAtBlockID(r.Context(), req.BlockID, req.Script.Source, req.Script.Args)
	}

	if req.BlockHeight == models.SealedHeight {
		result, err := backend.ExecuteScriptAtLatestBlock(r.Context(), req.Script.Source, req.Script.Args)
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	var height uint64
	if req.BlockHeight == models.FinalHeight {
		finalBlock, err := backend.GetLatestBlockHeader(r.Context(), false)
		if err != nil {
			return nil, err
		}
		height = finalBlock.Height
	}

	return backend.ExecuteScriptAtBlockHeight(r.Context(), height, req.Script.Source, req.Script.Args)
}
