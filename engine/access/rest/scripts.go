package rest

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
	"github.com/rs/zerolog"
	"net/http"
)

func executeScript(
	w http.ResponseWriter,
	r *http.Request,
	vars map[string]string,
	backend access.API,
	logger zerolog.Logger,
) (interface{}, StatusError) {
	blockID := vars["block_id"]
	var scriptBody generated.ScriptsBody
	err := jsonDecode(r.Body, &scriptBody)
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
