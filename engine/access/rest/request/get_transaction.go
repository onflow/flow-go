package request

import "github.com/onflow/flow-go/model/flow"

const resultExpandable = "result"
const blockIDQueryParam = "block_id"
const collectionIDQueryParam = "collection_id"

type TransactionOptionals struct {
	BlockID      flow.Identifier
	CollectionID flow.Identifier
}

func (t *TransactionOptionals) Build(r *Request) {
	var blockId ID
	// NOTE: blockId could be flow.ZeroID, as it is an optional parameter. The error here is intentionally ignored.
	_ = blockId.Parse(r.GetQueryParam(blockIDQueryParam))
	t.BlockID = blockId.Flow()

	var collectionId ID
	// NOTE: collectionId could be flow.ZeroID, as it is an optional parameter. The error here is intentionally ignored.
	_ = collectionId.Parse(r.GetQueryParam(collectionIDQueryParam))
	t.CollectionID = blockId.Flow()
}

type GetTransaction struct {
	GetByIDRequest
	TransactionOptionals
	ExpandsResult bool
}

func (g *GetTransaction) Build(r *Request) error {
	g.TransactionOptionals.Build(r)

	err := g.GetByIDRequest.Build(r)
	g.ExpandsResult = r.Expands(resultExpandable)

	return err
}

type GetTransactionResult struct {
	GetByIDRequest
	TransactionOptionals
}

func (g *GetTransactionResult) Build(r *Request) error {
	g.TransactionOptionals.Build(r)

	err := g.GetByIDRequest.Build(r)

	return err
}
