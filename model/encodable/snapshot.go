package encodable

import (
	"github.com/onflow/flow-go/model/flow"
)

// Snapshot is the encoding format for protocol.Snapshot
type Snapshot struct {
	Head              *flow.Header
	Identities        flow.IdentityList
	Commit            flow.StateCommitment
	QuorumCertificate *flow.QuorumCertificate
	Phase             flow.EpochPhase
	Epochs            Epochs
}
