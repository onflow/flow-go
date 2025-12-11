package request

import (
	"fmt"
	"strconv"

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
// All errors indicate the request is invalid.
func NewTransactionOptionals(r *common.Request) (*TransactionOptionals, error) {
	blockID, err := parser.NewID(r.GetQueryParam(blockIDQueryParam))
	if err != nil {
		return nil, fmt.Errorf("invalid block ID: %w", err)
	}

	collectionID, err := parser.NewID(r.GetQueryParam(collectionIDQueryParam))
	if err != nil {
		return nil, fmt.Errorf("invalid collection ID: %w", err)
	}

	return &TransactionOptionals{
		BlockID:      blockID.Flow(),
		CollectionID: collectionID.Flow(),
	}, nil
}

// GetTransaction represents a parsed HTTP request for retrieving a transaction.
type GetTransaction struct {
	GetByIDRequest
	TransactionOptionals
	ExpandsResult bool
	// TODO(Uliana): add this to the openapi
	ExecutionState models.ExecutionStateQuery
}

// NewGetTransactionRequest extracts necessary variables from the provided request,
// builds a GetTransaction instance, and validates it.
//
// All errors indicate a malformed request.
func NewGetTransactionRequest(r *common.Request) (*GetTransaction, error) {
	return parseGetTransactionRequest(
		r,
		r.GetQueryParam(agreeingExecutorCountQuery),
		r.GetQueryParams(requiredExecutorIdsQuery),
		r.GetQueryParam(includeExecutorMetadataQuery),
	)
}

// parseGetTransactionRequest parses raw HTTP query parameters into a
// GetTransaction struct. It validates the transaction ID, optional fields,
// and execution state configuration extracted from the request.
//
// All errors indicate the request is invalid.
func parseGetTransactionRequest(
	r *common.Request,
	rawAgreeingExecutorsCount string,
	rawAgreeingExecutorsIds []string,
	rawIncludeExecutorMetadata string,
) (*GetTransaction, error) {
	txOpts, err := NewTransactionOptionals(r)
	if err != nil {
		return nil, err
	}

	var getByIDRequest GetByIDRequest
	if err := getByIDRequest.Build(r); err != nil {
		return nil, err
	}

	executionStateQuery, err := parser.NewExecutionStateQuery(
		rawAgreeingExecutorsCount,
		rawAgreeingExecutorsIds,
		rawIncludeExecutorMetadata,
	)
	if err != nil {
		return nil, err
	}

	return &GetTransaction{
		GetByIDRequest:       getByIDRequest,
		TransactionOptionals: *txOpts,
		ExpandsResult:        r.Expands(resultExpandable),
		ExecutionState:       *executionStateQuery,
	}, nil
}

// GetTransactionResult represents a parsed HTTP request for retrieving a transaction result.
type GetTransactionResult struct {
	GetByIDRequest
	TransactionOptionals
	// TODO(Uliana): add this to the openapi
	ExecutionState models.ExecutionStateQuery
}

// NewGetTransactionResult extracts necessary variables from the provided request,
// builds a GetTransactionResult instance, and validates it.
//
// All errors indicate a malformed request.
func NewGetTransactionResult(r *common.Request) (*GetTransactionResult, error) {
	return parseGetTransactionResult(
		r,
		r.GetQueryParam(agreeingExecutorCountQuery),
		r.GetQueryParams(requiredExecutorIdsQuery),
		r.GetQueryParam(includeExecutorMetadataQuery),
	)
}

// parseGetTransactionResult parses raw HTTP query parameters into a
// GetTransactionResult struct. It validates the transaction ID, optional fields,
// and execution state configuration extracted from the request.
//
// All errors indicate the request is invalid.
func parseGetTransactionResult(
	r *common.Request,
	rawAgreeingExecutorsCount string,
	rawAgreeingExecutorsIds []string,
	rawIncludeExecutorMetadata string,
) (*GetTransactionResult, error) {
	txOpts, err := NewTransactionOptionals(r)
	if err != nil {
		return nil, err
	}

	var getByIDRequest GetByIDRequest
	if err := getByIDRequest.Build(r); err != nil {
		return nil, err
	}

	executionStateQuery, err := parser.NewExecutionStateQuery(
		rawAgreeingExecutorsCount,
		rawAgreeingExecutorsIds,
		rawIncludeExecutorMetadata,
	)
	if err != nil {
		return nil, err
	}

	return &GetTransactionResult{
		GetByIDRequest:       getByIDRequest,
		TransactionOptionals: *txOpts,
		ExecutionState:       *executionStateQuery,
	}, nil
}

// GetScheduledTransaction represents a request to get a scheduled transaction by its scheduled
// transaction ID, and contains the parsed and validated input parameters.
type GetScheduledTransaction struct {
	ScheduledTxID uint64
	TransactionOptionals
	ExpandsResult bool
}

// NewGetScheduledTransaction extracts the scheduled transaction ID from the request,
// builds a GetScheduledTransaction instance, and validates it.
//
// All errors indicate a malformed request.
func NewGetScheduledTransaction(r *common.Request) (GetScheduledTransaction, error) {
	raw := r.GetVar(idQuery)
	scheduledTxID, err := strconv.ParseUint(raw, 0, 64)
	if err != nil {
		return GetScheduledTransaction{}, fmt.Errorf("invalid ID format")
	}

	txOpts, err := NewTransactionOptionals(r)
	if err != nil {
		return GetScheduledTransaction{}, err
	}

	return GetScheduledTransaction{
		ScheduledTxID:        scheduledTxID,
		TransactionOptionals: *txOpts,
		ExpandsResult:        r.Expands(resultExpandable),
	}, nil
}

// GetScheduledTransactionResult represents a request to get a scheduled transaction result by its
// scheduled transaction ID, and contains the parsed and validated input parameters.
type GetScheduledTransactionResult struct {
	ScheduledTxID uint64
	TransactionOptionals
}

// NewGetScheduledTransactionResult extracts the scheduled transaction ID from the request,
// builds a GetScheduledTransactionResult instance, and validates it.
//
// All errors indicate a malformed request.
func NewGetScheduledTransactionResult(r *common.Request) (GetScheduledTransactionResult, error) {
	raw := r.GetVar(idQuery)
	scheduledTxID, err := strconv.ParseUint(raw, 0, 64)
	if err != nil {
		return GetScheduledTransactionResult{}, fmt.Errorf("invalid ID format")
	}

	txOpts, err := NewTransactionOptionals(r)
	if err != nil {
		return GetScheduledTransactionResult{}, err
	}

	return GetScheduledTransactionResult{
		ScheduledTxID:        scheduledTxID,
		TransactionOptionals: *txOpts,
	}, nil
}
