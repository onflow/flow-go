package execution

import "github.com/dapperlabs/flow-go/model/flow"

type CompleteCollection struct {
	Collection   flow.CollectionGuarantee
	Transactions []flow.TransactionBody
}

type CompleteBlock struct {
	Block flow.Block
	//TODO change to proper Hash once/if it becomes usable as maps key
	CompleteCollections map[string]CompleteCollection
}
