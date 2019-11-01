// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package coldstuff

import (
	"github.com/dapperlabs/flow-go/model/coldstuff"
)

// Hasher implements canonical hashing of consensus entities.
type Hasher interface {

	// BlockHash returns the canonical block hash of a block header.
	BlockHash(*coldstuff.BlockHeader) []byte
}
