// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package coldstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Commit is a coldstuff consensus event to commit a block.
type Commit struct {
	BlockID     flow.Identifier
	CommitterID flow.Identifier
}
