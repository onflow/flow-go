package routes

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/common"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/http/models"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

// ExecuteScript handler sends the script from the request to be executed.
func ExecuteScript(r *common.Request, backend access.API, _ commonmodels.LinkGenerator) (interface{}, error) {
	req, err := request.NewGetScript(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	executionState := req.ExecutionState
	includeExecutorMetadata := executionState.IncludeExecutorMetadata

	// TODO(mainnet #): remove this conditional: [TODO(Uliana): create an issue]
	// "legacyParams" is only to temporarily support current behaviour.
	// In the next spork, we should update this to always return an ExecuteScriptResponse.
	legacyParams := executionState.AgreeingExecutorsCount == 0 &&
		len(executionState.RequiredExecutorIDs) == 0 &&
		!includeExecutorMetadata

	buildResponse := func(value []byte, executorMetadata *accessmodel.ExecutorMetadata) interface{} {
		if legacyParams {
			return value
		}
		return commonmodels.NewExecuteScriptResponse(value, executorMetadata, includeExecutorMetadata)
	}

	if req.BlockID != flow.ZeroID {
		value, executorMetadata, err := backend.ExecuteScriptAtBlockID(
			r.Context(),
			req.BlockID,
			req.Script.Source,
			req.Script.Args,
			models.NewCriteria(req.ExecutionState),
		)
		if err != nil {
			return nil, common.ErrorToStatusError(err)
		}

		return buildResponse(value, executorMetadata), nil
	}

	// default to sealed height
	if req.BlockHeight == request.SealedHeight || req.BlockHeight == request.EmptyHeight {
		value, executorMetadata, err := backend.ExecuteScriptAtLatestBlock(
			r.Context(),
			req.Script.Source,
			req.Script.Args,
			models.NewCriteria(req.ExecutionState),
		)
		if err != nil {
			return nil, common.ErrorToStatusError(err)
		}

		return buildResponse(value, executorMetadata), nil
	}

	if req.BlockHeight == request.FinalHeight {
		finalBlock, _, err := backend.GetLatestBlockHeader(r.Context(), false)
		if err != nil {
			return nil, common.ErrorToStatusError(err)
		}
		req.BlockHeight = finalBlock.Height
	}

	value, executorMetadata, err := backend.ExecuteScriptAtBlockHeight(
		r.Context(),
		req.BlockHeight,
		req.Script.Source,
		req.Script.Args,
		models.NewCriteria(req.ExecutionState),
	)
	if err != nil {
		return nil, common.ErrorToStatusError(err)
	}

	return buildResponse(value, executorMetadata), nil
}
