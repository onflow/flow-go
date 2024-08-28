package routes

import (
	"encoding/base64"

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

	var res []byte
	switch {
	case req.BlockID != flow.ZeroID:
		res, err = backend.ExecuteScriptAtBlockID(r.Context(), req.BlockID, req.Script.Source, req.Script.Args)
	case req.BlockHeight == request.SealedHeight || req.BlockHeight == request.EmptyHeight:
		res, err = backend.ExecuteScriptAtLatestBlock(r.Context(), req.Script.Source, req.Script.Args)
	case req.BlockHeight == request.FinalHeight:
		finalBlock, _, err := backend.GetLatestBlockHeader(r.Context(), false)
		if err != nil {
			return nil, err
		}

		req.BlockHeight = finalBlock.Height
		res, err = backend.ExecuteScriptAtBlockHeight(r.Context(), req.BlockHeight, req.Script.Source, req.Script.Args)
	}

	if err != nil {
		return nil, err
	}

	response := models.InlineResponse200{
		Value: base64.StdEncoding.EncodeToString(res),
	}

	return response, nil
}
