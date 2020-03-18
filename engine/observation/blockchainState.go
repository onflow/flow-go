package observation

import (
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
)

// BlockChainState represents the entire block chain including blocks, collections and transactions
type BlockchainState struct {
	headers      storage.Headers
	payloads     storage.Payloads
	collections  storage.Collections
	transactions storage.Transactions //stores TransactionBody (not the complete transaction)

	// reverse lookup storage
	transactionInfo storage.TransactionInfos

	//lock since storage interface doesn't has an atomic upsert
	sync.Mutex
}

func NewBlockchainState(h storage.Headers,
	p storage.Payloads, c storage.Collections,
	t storage.Transactions,
	ti storage.TransactionInfos) *BlockchainState {
	return &BlockchainState{
		headers:         h,
		payloads:        p,
		collections:     c,
		transactions:    t,
		transactionInfo: ti,
	}
}

// UpsertCollection adds ors updates a collection to the block chain state
func (b *BlockchainState) UpsertCollection(collection *flow.Collection) error {
	b.Lock()
	defer b.Unlock()
	_, err := b.collections.ByID(collection.ID())
	if err != nil {
		if err == storage.ErrNotFound {
			err = b.collections.Store(collection)
			return err
		}
		return err
	}
	err = b.collections.Remove(collection.ID())
	if err != nil {
		return err
	}
	err = b.collections.Store(collection)
	return err
}

// AddTransaction adds the transaction body to the state if not present and adds the transactionInfo
func (b *BlockchainState) AddTransaction(transaction *flow.TransactionBody, collID flow.Identifier) error {
	b.Lock()
	defer b.Unlock()
	_, err := b.transactions.ByID(transaction.ID())
	if err == nil {
		return nil
	}
	if err == storage.ErrNotFound {
		err = b.transactions.Store(transaction)
		if err != nil {
			tInfo := &flow.TransactionInfo{
				TransactionID: transaction.ID(),
				CollectionID:  collID,
			}
			err = b.transactionInfo.Store(tInfo)
		}
	}
	return err
}

// Block retrieves a Block by ID
func (b *BlockchainState) Block(id flow.Identifier) (*flow.Header, error) {
	return b.headers.ByBlockID(id)
}
