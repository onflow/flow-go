package hotstuff

import (
	"time"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

// TODO I will consolidate these with the current definitions in `types` before merging

// Block represents a HotStuff block.
type Block struct {

	// QC is the quorum certificate justifying this block.
	QC *types.QuorumCertificate
	// PayloadHash is the hash of the block's payload.
	PayloadHash flow.Identifier

	// ChainID a unique identifier for the chain used to prevent maliciously
	// replaying a block generated in one chain to another.
	ChainID uint64

	// TODO should we have this if there is no mechanism to ensure it's accurate?
	Timestamp time.Time
}

// TODO is this correct? Does a block's QC have the same view number?
func (b *Block) View() uint64 {
	return b.QC.View
}

// BlockProposal represents a proposal for a new block. This is used generating
// a new block
type BlockProposal struct {
	Block

	// Signature is the signature over the proposal by the proposer.
	Signature crypto.Signature
}
