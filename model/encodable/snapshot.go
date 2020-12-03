package encodable

import (
	"github.com/onflow/flow-go/model/flow"
)

type Snapshot struct {
	Head              *flow.Header
	Identities        flow.IdentityList
	Commit            flow.StateCommitment
	QuorumCertificate *flow.QuorumCertificate
	Phase             flow.EpochPhase
	Epochs            Epochs
}
