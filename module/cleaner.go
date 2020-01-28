// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package module

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Cleaner represents a wrapper around all temporary state that needs cleaning
// up after a block is finalized, such as memory pools and caches used for
// forking awareness.
type Cleaner interface {
	CleanAfter(blockID flow.Identifier) error
}
