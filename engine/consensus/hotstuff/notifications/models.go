package notifications

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// TODO this is just an example of a notification model.
type BlockCreated struct {
	Block *types.Block
}
