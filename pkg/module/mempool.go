// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package module

import (
	"github.com/dapperlabs/flow-go/pkg/model/collection"
)

// Mempool implements a memory pool for collections.
type Mempool interface {
	Has(hash []byte) bool
	Add(coll *collection.GuaranteedCollection) error
	Get(hash []byte) (*collection.GuaranteedCollection, error)
	Hash() []byte
	Size() uint
	All() []*collection.GuaranteedCollection
}
