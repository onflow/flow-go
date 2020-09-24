// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package merkle

import (
	"github.com/onflow/flow-go/crypto/hash"
)

type node interface{}

type short struct {
	path  []byte // holds the common path to the next node
	count uint   // holds the count of bits in the path
	child node   // holds the child after the common path
}

type full struct {
	left  node // holds the left path node (bit 0)
	right node // holds the right path node (bit 1)
}

type leaf struct {
	key hash.Hash   // redundant copy of hash (key/path) for root hash
	val interface{} // the concrete data we actually stored
}
