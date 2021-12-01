package rest

import (
	"fmt"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
)

func executeScript(r *requestDecorator, backend access.API, _ LinkGenerator) (interface{}, error) {
	blockID := r.getQueryParam("block_id")
	blockHeight := r.getQueryParam("block_height")

	if blockID == "" && blockHeight == "" {
		return nil, NewBadRequestError(fmt.Errorf("either block ID or block height must be provided"))
	}
	if blockID != "" && blockHeight != "" {
		return nil, NewBadRequestError(fmt.Errorf("can not provide both block ID and block height"))
	}

	var scriptBody generated.ScriptsBody
	err := r.bodyAs(&scriptBody)
	if err != nil {
		return nil, err
	}

	args, err := toScriptArgs(scriptBody)
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	code, err := toScriptSource(scriptBody)
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	if blockID != "" {
		id, err := toID(blockID)
		if err != nil {
			return nil, NewBadRequestError(err)
		}

		result, err := backend.ExecuteScriptAtBlockID(r.Context(), id, code, args)
		if err != nil {
			return nil, err
		}

		return result, nil
	}

	if blockHeight == sealedHeightQueryParam {
		result, err := backend.ExecuteScriptAtLatestBlock(r.Context(), code, args)
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	var height uint64
	if blockHeight == finalHeightQueryParam {
		finalBlock, err := backend.GetLatestBlockHeader(r.Context(), false)
		if err != nil {
			return nil, err
		}
		height = finalBlock.Height
	} else {
		height, err = toHeight(blockHeight)
		if err != nil {
			return nil, NewBadRequestError(err)
		}
	}

	result, err := backend.ExecuteScriptAtBlockHeight(r.Context(), height, code, args)
	if err != nil {
		return nil, err
	}

	return result, nil
}
