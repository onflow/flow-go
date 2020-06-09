package committee

import "github.com/dapperlabs/flow-go/model/flow"

// BlockTranslator is a support component for determining a reference point in
// the protocol state to use when assessing validity of a block.
type BlockTranslator interface {

	// Translate returns the ID of the block on the main chain that should be
	// used as a reference point when assessing validity of the input block.
	// For consensus nodes, this is the identity function, since the main
	// consensus chain directly contains all staking information. For collector
	// clusters, this returns the reference block ID from the cluster block's
	// payload (ideally the most recently finalized block on the main chain).
	Translate(blockID flow.Identifier) (flow.Identifier, error)
}

type blockTranslatorFunc func(flow.Identifier) (flow.Identifier, error)

func (b blockTranslatorFunc) Translate(blockID flow.Identifier) (flow.Identifier, error) {
	return b(blockID)
}

// NewBlockTranslator returns a block translator that simply uses the input
// function.
//
// NOTE: for convenience in testing and within this package.
func NewBlockTranslator(b blockTranslatorFunc) BlockTranslator {
	return b
}
