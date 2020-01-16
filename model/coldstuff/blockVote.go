// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package coldstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// BlockVote is a coldstuff consensus event to vote for a block.
type BlockVote struct {
	BlockID flow.Identifier
}
