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
	var blockId parser.ID
	err := blockId.Parse(r.GetQueryParam(blockIDQueryParam))
	if err != nil {
		return err
	}
	t.BlockID = blockId.Flow()

	var collectionId parser.ID
	err = collectionId.Parse(r.GetQueryParam(collectionIDQueryParam))
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

// GetTransactionsByBlock represents a request to get transactions by
// a block ID, and contains the parsed and validated input parameters.
type GetTransactionsByBlock struct {
	TransactionOptionals
	BlockHeight   uint64
	ExpandsResult bool
}

// NewGetTransactionsByBlockRequest extracts necessary variables from the provided request,
// builds a GetTransactionsByBlock instance, and validates it.
//
// All errors indicate a malformed request.
func NewGetTransactionsByBlockRequest(r *common.Request) (*GetTransactionsByBlock, error) {
	req, err := parseGetTransactionsByBlock(r)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// parseGetTransactionsByBlock parses raw query and body parameters from an incoming request
// and constructs a validated GetTransactionsByBlock instance.
//
// All errors indicate the request is invalid.
func parseGetTransactionsByBlock(r *common.Request) (*GetTransactionsByBlock, error) {
	var req GetTransactionsByBlock
	err := req.TransactionOptionals.Parse(r)
	if err != nil {
		return nil, err
	}

	var height Height
	err = height.Parse(r.GetQueryParam(blockHeightQuery))
	if err != nil {
		return nil, err
	}
	req.BlockHeight = height.Flow()
	req.ExpandsResult = r.Expands(resultExpandable)

	if req.BlockHeight == EmptyHeight && req.BlockID == flow.ZeroID {
		req.BlockHeight = SealedHeight
	}

	if req.BlockHeight != EmptyHeight && req.BlockID != flow.ZeroID {
		return nil, fmt.Errorf("can not provide both block ID and block height")
	}

	return &req, err
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

// GetTransactionResultsByBlock represents a request to get transaction results by
// block ID or height, and contains the parsed and validated input parameters.
type GetTransactionResultsByBlock struct {
	TransactionOptionals
	BlockHeight uint64
}

// NewGetTransactionResultsByBlockRequest extracts necessary variables from the provided request,
// builds a GetTransactionResultsByBlock instance, and validates it.
//
// All errors indicate a malformed request.
func NewGetTransactionResultsByBlockRequest(r *common.Request) (*GetTransactionResultsByBlock, error) {
	req, err := parseGetTransactionResultsByBlock(r)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// parseGetTransactionResultsByBlock parses raw query and body parameters from an incoming request
// and constructs a validated GetTransactionResultsByBlock instance.
//
// All errors indicate the request is invalid.
func parseGetTransactionResultsByBlock(r *common.Request) (*GetTransactionResultsByBlock, error) {
	var req GetTransactionResultsByBlock
	err := req.TransactionOptionals.Parse(r)
	if err != nil {
		return nil, err
	}

	var height Height
	err = height.Parse(r.GetQueryParam(blockHeightQuery))
	if err != nil {
		return nil, err
	}
	req.BlockHeight = height.Flow()

	if req.BlockHeight == EmptyHeight && req.BlockID == flow.ZeroID {
		req.BlockHeight = SealedHeight
	}

	if req.BlockHeight != EmptyHeight && req.BlockID != flow.ZeroID {
		return nil, fmt.Errorf("can not provide both block ID and block height")
	}

	return &req, err
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
