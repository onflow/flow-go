// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package coldstuff

import (
	"github.com/dapperlabs/flow-go/model/coldstuff"
)

// Chain implements a very dumb blockchain interface.
type Chain interface {

	// Head retrieves the block header with the highest number.
	Head() *coldstuff.BlockHeader

	// Commit tries to add given block header to the chain.
	Commit(*coldstuff.BlockHeader) error
}
