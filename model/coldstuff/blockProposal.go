// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package coldstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// BlockProposal is a coldstuff consensus event to propose a block.
type BlockProposal struct {
	Block *flow.Block
}
