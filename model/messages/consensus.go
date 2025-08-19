package messages

import (
	"github.com/onflow/flow-go/model/flow"
)

// BlockVote is part of the consensus protocol and represents a consensus node
// voting on the proposal of the leader of a given round.
type BlockVote struct {
	BlockID flow.Identifier
	View    uint64
	SigData []byte
}

// ToInternal converts the untrusted BlockVote into its trusted internal
// representation.
//
// This stub returns the receiver unchanged. A proper implementation
// must perform validation checks and return a constructed internal
// object.
func (b *BlockVote) ToInternal() (any, error) {
	// TODO(malleability, #7702) implement with validation checks
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

// ToInternal converts the untrusted TimeoutObject into its trusted internal
// representation.
//
// This stub returns the receiver unchanged. A proper implementation
// must perform validation checks and return a constructed internal
// object.
func (t *TimeoutObject) ToInternal() (any, error) {
	// TODO(malleability, #7700) implement with validation checks
	return t, nil
}
