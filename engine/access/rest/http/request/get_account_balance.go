package request

import (
	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/http/models"
	"github.com/onflow/flow-go/model/flow"
)

// GetAccountBalance represents a parsed HTTP request for retrieving an account balance.
type GetAccountBalance struct {
	Address        flow.Address
	Height         uint64
	ExecutionState models.ExecutionStateQuery
}

// NewGetAccountBalanceRequest extracts necessary variables and query parameters from the provided request,
// builds a GetAccountBalance instance, and validates it.
//
// All errors indicate the request is invalid.
func NewGetAccountBalanceRequest(r *common.Request) (*GetAccountBalance, error) {
	return parseGetAccountBalanceRequest(
		r.GetVar(addressVar),
		r.GetQueryParam(blockHeightQuery),
		r.GetQueryParam(agreeingExecutorCountQuery),
		r.GetQueryParams(requiredExecutorIdsQuery),
		r.GetQueryParam(includeExecutorMetadataQuery),
		r.Chain,
	)
}

// parseGetAccountBalanceRequest parses raw HTTP query parameters into a GetAccountBalance struct.
// It validates the account address, block height, and execution state parameters, applying
// defaults where necessary (using the sealed block when height is not provided).
//
// All errors indicate the request is invalid.
func parseGetAccountBalanceRequest(
	rawAddress string,
	rawHeight string,
	rawAgreeingExecutorsCount string,
	rawAgreeingExecutorsIds []string,
	rawIncludeExecutorMetadata string,
	chain flow.Chain,
) (*GetAccountBalance, error) {
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

	return &GetAccountBalance{
		Address:        address,
		Height:         height,
		ExecutionState: *executionStateQuery,
	}, nil
}
