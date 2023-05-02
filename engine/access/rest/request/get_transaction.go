package request

import "github.com/onflow/flow-go/model/flow"

const resultExpandable = "result"
const blockIDQueryParam = "block_id"
const collectionIDQueryParam = "collection_id"

type TransactionOptionals struct {
	BlockID      flow.Identifier
	CollectionID flow.Identifier
}

func (t *TransactionOptionals) Parse(r *Request) error {
	var blockId ID
	err := blockId.Parse(r.GetQueryParam(blockIDQueryParam))
	if err != nil {
		return err
	}
	t.BlockID = blockId.Flow()

	var collectionId ID
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

func (g *GetTransaction) Build(r *Request) error {
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

func (g *GetTransactionResult) Build(r *Request) error {
	err := g.TransactionOptionals.Parse(r)
	if err != nil {
		return err
	}

	err = g.GetByIDRequest.Build(r)

	return err
}
