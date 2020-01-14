package execution

import "github.com/dapperlabs/flow-go/model/flow"

type CompleteCollection struct {
	Guarantee    *flow.CollectionGuarantee
	Transactions []flow.TransactionBody
}

type CompleteBlock struct {
	Block               flow.Block
	CompleteCollections map[flow.Identifier]*CompleteCollection
}
