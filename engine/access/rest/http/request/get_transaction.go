package request

import (
	"fmt"
	"strconv"

	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
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
// All errors indicate a malformed request.
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
}

// GetTransactionResultRequest extracts necessary variables from the provided request,
// builds a GetTransactionResult instance, and validates it.
//
// All errors indicate a malformed request.
func GetTransactionResultRequest(r *common.Request) (GetTransactionResult, error) {
	var req GetTransactionResult
	err := req.Build(r)
	return req, err
}

func (g *GetTransactionResult) Build(r *common.Request) error {
	err := g.TransactionOptionals.Parse(r)
	if err != nil {
		return err
	}

	err = g.GetByIDRequest.Build(r)

	return err
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

	var transactionOptionals TransactionOptionals
	err = transactionOptionals.Parse(r)
	if err != nil {
		return GetScheduledTransaction{}, err
	}

	return GetScheduledTransaction{
		ScheduledTxID:        scheduledTxID,
		TransactionOptionals: transactionOptionals,
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

	var transactionOptionals TransactionOptionals
	err = transactionOptionals.Parse(r)
	if err != nil {
		return GetScheduledTransactionResult{}, err
	}

	return GetScheduledTransactionResult{
		ScheduledTxID:        scheduledTxID,
		TransactionOptionals: transactionOptionals,
	}, nil
}
