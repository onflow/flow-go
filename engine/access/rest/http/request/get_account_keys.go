package request

import (
	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/http/models"
	"github.com/onflow/flow-go/model/flow"
)

// GetAccountKeys represents a parsed HTTP request for retrieving account keys.
type GetAccountKeys struct {
	Address        flow.Address
	Height         uint64
	ExecutionState models.ExecutionStateQuery
}

// NewGetAccountKeysRequest extracts necessary variables and query parameters from the provided request,
// builds a GetAccountKeys instance, and validates it.
//
// No errors are expected during normal operation.
func NewGetAccountKeysRequest(r *common.Request) (*GetAccountKeys, error) {
	return parseGetAccountKeysRequest(
		r.GetVar(addressVar),
		r.GetQueryParam(blockHeightQuery),
		r.GetQueryParam(agreeingExecutorCountQuery),
		r.GetQueryParams(requiredExecutorIdsQuery),
		r.GetQueryParam(includeExecutorMetadataQuery),
		r.Chain,
	)
}

// parseGetAccountKeysRequest parses raw HTTP query parameters into a GetAccountKeys struct.
// It validates the account address and block height, and execution state fields, applying
// defaults where necessary (using the sealed block when height is not provided).
//
// No errors are expected during normal operation.
func parseGetAccountKeysRequest(
	rawAddress string,
	rawHeight string,
	rawAgreeingExecutorsCount string,
	rawAgreeingExecutorsIds []string,
	rawIncludeExecutorMetadata string,
	chain flow.Chain,
) (*GetAccountKeys, error) {
	address, err := parser.ParseAddress(rawAddress, chain)
	if err != nil {
		return nil, err
	}

	var h Height
	err = h.Parse(rawHeight)
	if err != nil {
		return nil, err
	}
	height := h.Flow()

	// default to last block
	if height == EmptyHeight {
		height = SealedHeight
	}

	executionStateQuery, err := parser.NewExecutionStateQuery(
		rawAgreeingExecutorsCount,
		rawAgreeingExecutorsIds,
		rawIncludeExecutorMetadata,
	)
	if err != nil {
		return nil, err
	}

	return &GetAccountKeys{
		Address:        address,
		Height:         height,
		ExecutionState: *executionStateQuery,
	}, nil
}
