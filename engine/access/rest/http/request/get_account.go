package request

import (
	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/http/models"
	"github.com/onflow/flow-go/model/flow"
)

const addressVar = "address"
const blockHeightQuery = "block_height"

// GetAccount represents a parsed HTTP request for retrieving an account.
type GetAccount struct {
	Address        flow.Address
	Height         uint64
	ExecutionState models.ExecutionStateQuery
}

// NewGetAccountRequest extracts necessary variables and query parameters from the provided request,
// builds a GetAccount instance, and validates it.
//
// All errors indicate the request is invalid.
func NewGetAccountRequest(r *common.Request) (*GetAccount, error) {
	return parseGetAccountRequest(
		r.GetVar(addressVar),
		r.GetQueryParam(blockHeightQuery),
		r.GetQueryParam(agreeingExecutorCountQuery),
		r.GetQueryParams(requiredExecutorIdsQuery),
		r.GetQueryParam(includeExecutorMetadataQuery),
		r.Chain,
	)
}

// parseGetAccountRequest parses raw HTTP query parameters into a GetAccount struct.
// It validates the account address, height, and execution state fields, applying
// defaults where necessary (using the sealed block when height is not provided).
//
// All errors indicate the request is invalid.
func parseGetAccountRequest(
	rawAddress string,
	rawHeight string,
	rawAgreeingExecutorsCount string,
	rawAgreeingExecutorsIds []string,
	rawIncludeExecutorMetadata string,
	chain flow.Chain,
) (*GetAccount, error) {
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

	return &GetAccount{
		Address:        address,
		Height:         height,
		ExecutionState: *executionStateQuery,
	}, nil
}
