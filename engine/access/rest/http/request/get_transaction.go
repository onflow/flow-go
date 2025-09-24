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

// NewTransactionOptionals parses the request and returns a validated TransactionOptionals.
//
// All errors returned are benign and indicate invalid input data.
func NewTransactionOptionals(r *common.Request) (TransactionOptionals, error) {
	blockId, err := parser.NewID(r.GetQueryParam(blockIDQueryParam))
	if err != nil {
		return TransactionOptionals{}, fmt.Errorf("invalid block ID: %w", err)
	}

	collectionId, err := parser.NewID(r.GetQueryParam(collectionIDQueryParam))
	if err != nil {
		return TransactionOptionals{}, fmt.Errorf("invalid collection ID: %w", err)
	}

	return TransactionOptionals{
		BlockID:      blockId.Flow(),
		CollectionID: collectionId.Flow(),
	}, nil
}

type GetTransaction struct {
	GetByIDRequest
	TransactionOptionals
	ExpandsResult  bool
	ExecutionState models.ExecutionStateQuery // TODO: add this to the openapi spec
}

// NewGetTransactionRequest parses the request and returns a validated request.GetTransaction.
//
// All errors returned are benign and indicate invalid input data.
func NewGetTransactionRequest(r *common.Request) (GetTransaction, error) {
	return parseGetTransactionRequest(
		r,
		r.GetQueryParam(agreeingExecutorCountQuery),
		r.GetQueryParams(requiredExecutorIdsQuery),
		r.GetQueryParam(includeExecutorMetadataQuery),
	)
}

func parseGetTransactionRequest(
	r *common.Request,
	rawAgreeingExecutorsCount string,
	rawAgreeingExecutorsIds []string,
	rawIncludeExecutorMetadata string,
) (GetTransaction, error) {
	txOpts, err := NewTransactionOptionals(r)
	if err != nil {
		return GetTransaction{}, fmt.Errorf("invalid transaction optionals: %w", err)
	}

	var byID GetByIDRequest
	if err := byID.Build(r); err != nil {
		return GetTransaction{}, fmt.Errorf("invalid ID request: %w", err)
	}

	executionStateQuery, err := parser.NewExecutionStateQuery(
		rawAgreeingExecutorsCount,
		rawAgreeingExecutorsIds,
		rawIncludeExecutorMetadata,
	)
	if err != nil {
		return GetTransaction{}, err
	}

	return GetTransaction{
		GetByIDRequest:       byID,
		TransactionOptionals: txOpts,
		ExpandsResult:        r.Expands(resultExpandable),
		ExecutionState:       *executionStateQuery,
	}, nil
}

type GetTransactionResult struct {
	GetByIDRequest
	TransactionOptionals
	ExecutionState models.ExecutionStateQuery
}

// NewGetTransactionResult parses the request and returns a validated request.GetTransactionResult.
//
// All errors returned are benign and indicate invalid input data.
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
	txOpts, err := NewTransactionOptionals(r)
	if err != nil {
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
