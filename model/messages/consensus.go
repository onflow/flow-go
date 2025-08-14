package messages

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

var _ UntrustedMessage = (*BlockVote)(nil)

// BlockVote is part of the consensus protocol and represents a consensus node
// voting on the proposal of the leader of a given round.
type BlockVote struct {
	BlockID flow.Identifier
	View    uint64
	SigData []byte
}

func (b BlockVote) ToInternal() (interface{}, error) {
	// Temporary: just return the unvalidated wire struct
	return b, nil
}

// TimeoutObject is part of the consensus protocol and represents a consensus node
// timing out in given round. Contains a sequential number for deduplication purposes.
type TimeoutObject struct {
	TimeoutTick uint64
	View        uint64
	NewestQC    *flow.QuorumCertificate
	LastViewTC  *flow.TimeoutCertificate
	SigData     []byte
}

var _ UntrustedMessage = (*TimeoutObject)(nil)

func (to TimeoutObject) ToInternal() (any, error) {
	// Temporary: just return the unvalidated wire struct
	return to, nil
}

type Proposal flow.UntrustedProposal

var _ UntrustedMessage = (*Proposal)(nil)

func (p *Proposal) ToInternal() (any, error) {
	// Temporary: just return the unvalidated wire struct
	return p, nil
}

type ClusterProposal cluster.UntrustedProposal

var _ UntrustedMessage = (*ClusterProposal)(nil)

func (cp *ClusterProposal) ToInternal() (any, error) {
	// Temporary: just return the unvalidated wire struct
	return cp, nil
}
