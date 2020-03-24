package messages

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// BlockProposal is part of the consensus protocol and represents the the leader
// of a consensus round pushing a new proposal to the network.
type BlockProposal struct {
	Header  *flow.Header
	Payload *flow.Payload
}

// BlockVote is part of the consensus protocol and represents a consensus node
// voting on the proposal of the leader of a given round.
type BlockVote struct {
	BlockID               flow.Identifier
	View                  uint64
	StakingSignature      crypto.Signature
	RandomBeaconSignature crypto.Signature
}
