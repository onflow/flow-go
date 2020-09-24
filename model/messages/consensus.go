package messages

import (
	"github.com/onflow/flow-go/model/flow"
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
	BlockID flow.Identifier
	View    uint64
	SigData []byte
}
