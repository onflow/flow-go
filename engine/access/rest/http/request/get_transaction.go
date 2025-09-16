package request

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/http/models"
	"github.com/onflow/flow-go/model/flow"
)

const resultExpandable = "result"
const blockIDQueryParam = "block_id"
const collectionIDQueryParam = "collection_id"

type TransactionOptionals struct {
	BlockID      flow.Identifier
	CollectionID flow.Identifier
}

func (t *TransactionOptionals) Parse(r *common.Request) error {
	blockId, err := parser.NewID(r.GetQueryParam(blockIDQueryParam))
	if err != nil {
		return err
	}
	t.BlockID = blockId.Flow()

	collectionId, err := parser.NewID(r.GetQueryParam(collectionIDQueryParam))
	if err != nil {
		return err
	}
	t.CollectionID = collectionId.Flow()

	return nil
}

type GetTransaction struct {
	GetByIDRequest
	TransactionOptionals
	ExpandsResult bool
}

// GetTransactionRequest extracts necessary variables from the provided request,
// builds a GetTransaction instance, and validates it.
//
// No errors are expected during normal operation.
func GetTransactionRequest(r *common.Request) (GetTransaction, error) {
	var req GetTransaction
	err := req.Build(r)
	return req, err
}

func (g *GetTransaction) Build(r *common.Request) error {
	err := g.TransactionOptionals.Parse(r)
	if err != nil {
		return err
	}

	err = g.GetByIDRequest.Build(r)
	g.ExpandsResult = r.Expands(resultExpandable)

	return err
}

type GetTransactionResult struct {
	GetByIDRequest
	TransactionOptionals
	ExecutionState models.ExecutionStateQuery
}

// NewGetTransactionResult extracts necessary variables from the provided request
// and returns a validated GetTransactionResult.
//
// No errors are expected during normal operation.
func NewGetTransactionResult(r *common.Request) (GetTransactionResult, error) {
	return parseGetTransactionResult(
		r,
		r.GetQueryParam(agreeingExecutorCountQuery),
		r.GetQueryParams(requiredExecutorIdsQuery),
		r.GetQueryParam(includeExecutorMetadataQuery),
	)
}

func parseGetTransactionResult(
	r *common.Request,
	rawAgreeingExecutorsCount string,
	rawAgreeingExecutorsIds []string,
	rawIncludeExecutorMetadata string,
) (GetTransactionResult, error) {
	var txOpts TransactionOptionals
	if err := txOpts.Parse(r); err != nil {
		return GetTransactionResult{}, fmt.Errorf("invalid transaction optionals: %w", err)
	}

	var byID GetByIDRequest
	if err := byID.Build(r); err != nil {
		return GetTransactionResult{}, fmt.Errorf("invalid ID request: %w", err)
	}

	executionStateQuery, err := parser.NewExecutionStateQuery(
		rawAgreeingExecutorsCount,
		rawAgreeingExecutorsIds,
		rawIncludeExecutorMetadata,
	)
	if err != nil {
		return GetTransactionResult{}, err
	}

	return GetTransactionResult{
		GetByIDRequest:       byID,
		TransactionOptionals: txOpts,
		ExecutionState:       *executionStateQuery,
	}, nil
}
