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

// GetTransactionsByBlockID represents a request to get transactions by
// a block ID, and contains the parsed and validated input parameters.
type GetTransactionsByBlockID struct {
	TransactionOptionals
	BlockHeight   uint64
	ExpandsResult bool
}

// NewGetTransactionsByBlockIDRequest extracts necessary variables from the provided request,
// builds a GetTransactionsByBlockID instance, and validates it.
//
// All errors indicate a malformed request.
func NewGetTransactionsByBlockIDRequest(r *common.Request) (*GetTransactionsByBlockID, error) {
	req, err := parseGetTransactionsByBlockID(r)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// parseGetTransactionsByBlockID parses raw query and body parameters from an incoming request
// and constructs a validated GetTransactionsByBlockID instance.
//
// All errors indicate the request is invalid.
func parseGetTransactionsByBlockID(r *common.Request) (*GetTransactionsByBlockID, error) {
	var req GetTransactionsByBlockID
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

	if req.BlockID != flow.ZeroID && req.BlockHeight != EmptyHeight {
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

// GetTransactionResultsByBlockID represents a request to get transaction results by
// block ID or height, and contains the parsed and validated input parameters.
type GetTransactionResultsByBlockID struct {
	TransactionOptionals
	BlockHeight uint64
}

// NewGetTransactionResultsByBlockIDRequest extracts necessary variables from the provided request,
// builds a GetTransactionResultsByBlockID instance, and validates it.
//
// All errors indicate a malformed request.
func NewGetTransactionResultsByBlockIDRequest(r *common.Request) (*GetTransactionResultsByBlockID, error) {
	req, err := parseGetTransactionResultsByBlockID(r)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// parseGetTransactionResultsByBlockID parses raw query and body parameters from an incoming request
// and constructs a validated GetTransactionResultsByBlockID instance.
//
// All errors indicate the request is invalid.
func parseGetTransactionResultsByBlockID(r *common.Request) (*GetTransactionResultsByBlockID, error) {
	var req GetTransactionResultsByBlockID
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

	if req.BlockID != flow.ZeroID && req.BlockHeight != EmptyHeight {
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
