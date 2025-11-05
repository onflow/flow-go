package request

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/http/models"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

const indexVar = "index"

// GetAccountKey represents a parsed HTTP request for retrieving an account key.
type GetAccountKey struct {
	Address        flow.Address
	Index          uint32
	Height         uint64
	ExecutionState models.ExecutionStateQuery
}

// NewGetAccountKeyRequest extracts necessary variables and query parameters from the provided request,
// builds a GetAccountKey instance, and validates it.
//
// No errors are expected during normal operation.
func NewGetAccountKeyRequest(r *common.Request) (*GetAccountKey, error) {
	return parseGetAccountKeyRequest(
		r.GetVar(addressVar),
		r.GetVar(indexVar),
		r.GetQueryParam(blockHeightQuery),
		r.GetQueryParam(agreeingExecutorCountQuery),
		r.GetQueryParams(requiredExecutorIdsQuery),
		r.GetQueryParam(includeExecutorMetadataQuery),
		r.Chain,
	)
}

// parseGetAccountKeyRequest parses raw HTTP query parameters into a GetAccountKey struct.
// It validates the account address, key index, block height, and execution state fields, applying
// defaults where necessary (using the sealed block when height is not provided).
//
// No errors are expected during normal operation.
func parseGetAccountKeyRequest(
	rawAddress string,
	rawIndex string,
	rawHeight string,
	rawAgreeingExecutorsCount string,
	rawAgreeingExecutorsIds []string,
	rawIncludeExecutorMetadata string,
	chain flow.Chain,
) (*GetAccountKey, error) {
	address, err := parser.ParseAddress(rawAddress, chain)
	if err != nil {
		return nil, err
	}

	index, err := util.ToUint32(rawIndex)
	if err != nil {
		return nil, fmt.Errorf("invalid key index: %w", err)
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

	return &GetAccountKey{
		Address:        address,
		Index:          index,
		Height:         height,
		ExecutionState: *executionStateQuery,
	}, nil
}
