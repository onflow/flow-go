// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package coldstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// BlockCommit is a coldstuff consensus event to commit a block.
type BlockCommit struct {
	BlockID flow.Identifier
}
