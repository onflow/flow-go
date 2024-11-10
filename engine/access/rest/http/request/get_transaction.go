package request

import (
	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/common/convert"
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
	var blockId convert.ID
	err := blockId.Parse(r.GetQueryParam(blockIDQueryParam))
	if err != nil {
		return err
	}
	t.BlockID = blockId.Flow()

	var collectionId convert.ID
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
}

// GetTransactionResultRequest extracts necessary variables from the provided request,
// builds a GetTransactionResult instance, and validates it.
//
// No errors are expected during normal operation.
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
