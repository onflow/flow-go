package request

import (
	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/http/models"
	"github.com/onflow/flow-go/model/flow"
)

const addressVar = "address"
const blockHeightQuery = "block_height"

type GetAccount struct {
	Address        flow.Address
	Height         uint64
	ExecutionState models.ExecutionStateQuery
}

// NewGetAccountRequest extracts necessary variables and query parameters from the provided request,
// builds a GetAccount instance, and validates it.
//
// No errors are expected during normal operation.
func NewGetAccountRequest(r *common.Request) (GetAccount, error) {
	return parseGetAccountRequest(
		r.GetVar(addressVar),
		r.GetQueryParam(blockHeightQuery),
		r.GetQueryParam(agreeingExecutorCountQuery),
		r.GetQueryParams(requiredExecutorIdsQuery),
		r.GetQueryParam(includeExecutorMetadataQuery),
		r.Chain,
	)
}

func parseGetAccountRequest(
	rawAddress string,
	rawHeight string,
	rawAgreeingExecutorsCount string,
	rawAgreeingExecutorsIds []string,
	rawIncludeExecutorMetadata string,
	chain flow.Chain,
) (GetAccount, error) {
	address, err := parser.ParseAddress(rawAddress, chain)
	if err != nil {
		return GetAccount{}, err
	}

	var h Height
	err = h.Parse(rawHeight)
	if err != nil {
		return GetAccount{}, err
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
		return GetAccount{}, err
	}

	return GetAccount{
		Address:        address,
		Height:         height,
		ExecutionState: *executionStateQuery,
	}, nil
}
