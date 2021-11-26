package rest

import (
	"fmt"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
)

func executeScript(r *requestDecorator, backend access.API, _ LinkGenerator) (interface{}, StatusError) {
	blockID := r.getVar("block_id")
	blockHeight := r.getVar("block_height")

	if blockID != "" && blockHeight != "" {
		err := fmt.Errorf("can not provide both block ID and block height")
		return nil, NewBadRequestError(err.Error(), err)
	}

	if blockID == "" && blockHeight == "" {
		err := fmt.Errorf("either block ID or block height must be provided")
		return nil, NewBadRequestError(err.Error(), err)
	}

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
	if blockID == "latest" || blockHeight == "latest" {
		result, err = backend.ExecuteScriptAtLatestBlock(r.Context(), code, args)
		if err != nil {
			return nil, NewBadRequestError(err.Error(), err)
		}
	}

	if blockID != "" {
		id, err := toID(blockID)
		if err != nil {
			return nil, NewBadRequestError(err.Error(), err)
		}

		result, err = backend.ExecuteScriptAtBlockID(r.Context(), id, code, args)
		if err != nil {
			return nil, NewBadRequestError(err.Error(), err)
		}
	}

	if blockHeight != "" {
		height, err := toHeight(blockHeight)
		if err != nil {
			return nil, NewBadRequestError(err.Error(), err)
		}

		result, err = backend.ExecuteScriptAtBlockHeight(r.Context(), height, code, args)
		if err != nil {
			return nil, NewBadRequestError(err.Error(), err)
		}
	}

	return result, nil
}
