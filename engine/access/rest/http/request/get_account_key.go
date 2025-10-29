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
func NewGetAccountKeyRequest(r *common.Request) (GetAccountKey, error) {
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

func parseGetAccountKeyRequest(
	rawAddress string,
	rawIndex string,
	rawHeight string,
	rawAgreeingExecutorsCount string,
	rawAgreeingExecutorsIds []string,
	rawIncludeExecutorMetadata string,
	chain flow.Chain,
) (GetAccountKey, error) {
	address, err := parser.ParseAddress(rawAddress, chain)
	if err != nil {
		return GetAccountKey{}, err
	}

	index, err := util.ToUint32(rawIndex)
	if err != nil {
		return GetAccountKey{}, fmt.Errorf("invalid key index: %w", err)
	}

	var h Height
	err = h.Parse(rawHeight)
	if err != nil {
		return GetAccountKey{}, err
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
		return GetAccountKey{}, err
	}

	return GetAccountKey{
		Address:        address,
		Index:          index,
		Height:         height,
		ExecutionState: *executionStateQuery,
	}, nil
}
