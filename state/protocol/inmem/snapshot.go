package inmem

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/seed"
)

var (
	_ protocol.Snapshot   = new(Snapshot)
	_ protocol.EpochQuery = new(Epochs)
	_ protocol.Epoch      = new(Epoch)
	_ protocol.Cluster    = new(Cluster)
)

// Snapshot is a memory-backed implementation of protocol.Snapshot. The snapshot
// data is stored in the embedded encodable snapshot model, which defines the
// canonical structure of an encoded snapshot for the purposes of serialization.
type Snapshot struct {
	enc EncodableSnapshot
}

func (s Snapshot) Head() (*flow.Header, error) {
	return s.enc.Head, nil
}

func (s Snapshot) QuorumCertificate() (*flow.QuorumCertificate, error) {
	return s.enc.QuorumCertificate, nil
}

func (s Snapshot) Identities(selector flow.IdentityFilter) (flow.IdentityList, error) {
	return s.enc.Identities.Filter(selector), nil
}

func (s Snapshot) Identity(nodeID flow.Identifier) (*flow.Identity, error) {
	identity, ok := s.enc.Identities.ByNodeID(nodeID)
	if !ok {
		return nil, protocol.IdentityNotFoundError{NodeID: nodeID}
	}
	return identity, nil
}

func (s Snapshot) Commit() (flow.StateCommitment, error) {
	return s.enc.LatestSeal.FinalState, nil
}

func (s Snapshot) SealedResult() (*flow.ExecutionResult, *flow.Seal, error) {
	return s.enc.LatestResult, s.enc.LatestSeal, nil
}

func (s Snapshot) SealingSegment() ([]*flow.Block, error) {
	return s.enc.SealingSegment, nil
}

func (s Snapshot) Descendants() ([]flow.Identifier, error) {
	// canonical snapshots don't have any descendants
	return nil, nil
}

func (s Snapshot) ValidDescendants() ([]flow.Identifier, error) {
	// canonical snapshots don't have any descendants
	return nil, nil
}

func (s Snapshot) Phase() (flow.EpochPhase, error) {
	return s.enc.Phase, nil
}

func (s Snapshot) Seed(indices ...uint32) ([]byte, error) {
	return seed.FromParentSignature(indices, s.enc.QuorumCertificate.SigData)
}

func (s Snapshot) Epochs() protocol.EpochQuery {
	return Epochs{s.enc.Epochs}
}

func (s Snapshot) Encodable() EncodableSnapshot {
	return s.enc
}

func SnapshotFromEncodable(enc EncodableSnapshot) *Snapshot {
	return &Snapshot{
		enc: enc,
	}
}
