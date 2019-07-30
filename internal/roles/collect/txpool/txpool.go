package txpool

import (
	"sync"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

type TxPool struct {
	transactions map[crypto.Hash]types.SignedTransaction
	mutex        sync.RWMutex
}

func New() *TxPool {
	return &TxPool{
		transactions: make(map[crypto.Hash]types.SignedTransaction),
	}
}

func (tp *TxPool) Insert(tx types.SignedTransaction) {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()

	tp.transactions[tx.Hash()] = tx
}

func (tp *TxPool) Contains(hash crypto.Hash) bool {
	tp.mutex.RLock()
	defer tp.mutex.RUnlock()

	_, exists := tp.transactions[hash]
	return exists
}

func (tp *TxPool) Remove(hashes ...crypto.Hash) {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()

	for _, hash := range hashes {
		delete(tp.transactions, hash)
	}
}
