package messages

import (
	"github.com/onflow/flow-go/model"
	"github.com/onflow/flow-go/model/flow"
)

// BlockProposal is part of the consensus protocol and represents the leader
// of a consensus round pushing a new proposal to the network.
type BlockProposal struct {
	Header  *flow.Header
	Payload *flow.Payload
}

// StructureValid checks basic structural validity of the message and ensures no required fields are nil.
func (m *BlockProposal) StructureValid() error {
	if m.Header == nil {
		return model.NewStructureInvalidError("nil block header")
	}
	if m.Payload == nil {
		return model.NewStructureInvalidError("nil block payload")
	}
	return nil
}

// BlockVote is part of the consensus protocol and represents a consensus node
// voting on the proposal of the leader of a given round.
type BlockVote struct {
	BlockID flow.Identifier
	View    uint64
	SigData []byte
}

// TimeoutObject is part of the consensus protocol and represents a consensus node
// timing out in given round.
type TimeoutObject struct {
	View       uint64
	NewestQC   *flow.QuorumCertificate
	LastViewTC *flow.TimeoutCertificate
	SigData    []byte
}
