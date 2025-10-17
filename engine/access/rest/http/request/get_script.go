package request

import (
	"fmt"
	"io"

	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/http/models"
	"github.com/onflow/flow-go/model/flow"
)

const blockIDQuery = "block_id"

// GetScript represents a parsed http request for executing a Cadence script.
type GetScript struct {
	BlockID        flow.Identifier
	BlockHeight    uint64
	Script         Script
	ExecutionState models.ExecutionStateQuery
}

// NewGetScript extracts necessary variables from the provided request,
// builds a GetScript instance, and validates it.
//
// No errors are expected during normal operation.
func NewGetScript(r *common.Request) (GetScript, error) {
	return parseGetScripts(
		r.GetQueryParam(blockHeightQuery),
		r.GetQueryParam(blockIDQuery),
		r.GetQueryParam(agreeingExecutorCountQuery),
		r.GetQueryParams(requiredExecutorIdsQuery),
		r.GetQueryParam(includeExecutorMetadataQuery),
		r.Body,
	)
}

// parseGetScripts parses raw query and body parameters from an incoming request
// and constructs a validated GetScript instance.
//
// It ensures that only one of block height or block ID is provided, defaults
// to the latest sealed block when neither is specified, and validates all
// script and execution state parameters.
func parseGetScripts(
	rawHeight string,
	rawID string,
	rawAgreeingExecutorsCount string,
	rawAgreeingExecutorsIds []string,
	rawIncludeExecutorMetadata string,
	rawScript io.Reader,
) (GetScript, error) {
	var height Height
	err := height.Parse(rawHeight)
	if err != nil {
		return GetScript{}, err
	}
	blockHeight := height.Flow()

	id, err := parser.NewID(rawID)
	if err != nil {
		return GetScript{}, err
	}
	blockID := id.Flow()

	var script Script
	err = script.Parse(rawScript)
	if err != nil {
		return GetScript{}, err
	}

	// default to last sealed block
	if blockHeight == EmptyHeight && blockID == flow.ZeroID {
		blockHeight = SealedHeight
	}

	if blockID != flow.ZeroID && blockHeight != EmptyHeight {
		return GetScript{}, fmt.Errorf("can not provide both block ID and block height")
	}

	executionStateQuery, err := parser.NewExecutionStateQuery(
		rawAgreeingExecutorsCount,
		rawAgreeingExecutorsIds,
		rawIncludeExecutorMetadata,
	)
	if err != nil {
		return GetScript{}, err
	}

	return GetScript{
		BlockHeight:    blockHeight,
		BlockID:        blockID,
		Script:         script,
		ExecutionState: *executionStateQuery,
	}, nil
}
