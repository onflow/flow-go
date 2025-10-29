package request

import (
	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/http/models"
	"github.com/onflow/flow-go/model/flow"
)

type GetAccountBalance struct {
	Address        flow.Address
	Height         uint64
	ExecutionState models.ExecutionStateQuery
}

// NewGetAccountBalanceRequest extracts necessary variables and query parameters from the provided request,
// builds a GetAccountBalance instance, and validates it.
//
// No errors are expected during normal operation.
func NewGetAccountBalanceRequest(r *common.Request) (GetAccountBalance, error) {
	return parseGetAccountBalanceRequest(
		r.GetVar(addressVar),
		r.GetQueryParam(blockHeightQuery),
		r.GetQueryParam(agreeingExecutorCountQuery),
		r.GetQueryParams(requiredExecutorIdsQuery),
		r.GetQueryParam(includeExecutorMetadataQuery),
		r.Chain,
	)
}

func parseGetAccountBalanceRequest(
	rawAddress string,
	rawHeight string,
	rawAgreeingExecutorsCount string,
	rawAgreeingExecutorsIds []string,
	rawIncludeExecutorMetadata string,
	chain flow.Chain,
) (GetAccountBalance, error) {
	address, err := parser.ParseAddress(rawAddress, chain)
	if err != nil {
		return GetAccountBalance{}, err
	}

	var h Height
	err = h.Parse(rawHeight)
	if err != nil {
		return GetAccountBalance{}, err
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
		return GetAccountBalance{}, err
	}

	return GetAccountBalance{
		Address:        address,
		Height:         height,
		ExecutionState: *executionStateQuery,
	}, nil
}
