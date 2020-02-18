package types

import (
	"github.com/dapperlabs/flow-go/crypto"
)

// Proposal represent a new proposed block within HotStuff (and thus a
// a header in the bigger picture), signed by the proposer.
type Proposal struct {
	Block     *Block
	Signature crypto.Signature
}

// ProposerVote extracts the proposer vote from the proposal
func (p *Proposal) ProposerVote() *Vote {
	sig := SingleSignature{
		SignerID: p.Block.ProposerID,
		Raw:      p.Signature,
	}
	vote := Vote{
		BlockID:   p.Block.BlockID,
		View:      p.Block.View,
		Signature: &sig,
	}
	return &vote
}
