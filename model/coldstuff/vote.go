// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package coldstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Vote is a coldstuff consensus event to vote for a block.
type Vote struct {
	BlockID flow.Identifier
	VoterID flow.Identifier
}
