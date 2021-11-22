package rest

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
)

func executeScript(r *requestDecorator, backend access.API, link LinkGenerator) (interface{}, StatusError) {
	blockID := r.getParam("block_id")
	var scriptBody generated.ScriptsBody
	err := r.bodyAs(&scriptBody)
	if err != nil {
		return nil, NewBadRequestError("invalid script execution request", err)
	}

	args, err := toScriptArgs(scriptBody)
	if err != nil {
		return nil, NewBadRequestError(err.Error(), err)
	}

	code, err := toScriptSource(scriptBody)
	if err != nil {
		return nil, NewBadRequestError(err.Error(), err)
	}

	var result []byte
	if blockID == "latest" {
		result, err = backend.ExecuteScriptAtLatestBlock(r.Context(), code, args)
		if err != nil {
			return nil, NewBadRequestError(err.Error(), err)
		}

		return result, nil
	}
	// todo(sideninja) implement block height support
	id, err := toID(blockID)
	if err != nil {
		return nil, NewBadRequestError(err.Error(), err)
	}

	result, err = backend.ExecuteScriptAtBlockID(r.Context(), id, code, args)
	if err != nil {
		return nil, NewBadRequestError(err.Error(), err)
	}

	return result, nil
}
