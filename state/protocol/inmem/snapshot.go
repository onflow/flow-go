package inmem

import (
	"github.com/onflow/flow-go/model/encodable"
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

type Snapshot struct {
	encodable.Snapshot
}

func (s *Snapshot) Head() (*flow.Header, error) {
	return s.Snapshot.Head, nil
}

func (s *Snapshot) QuorumCertificate() (*flow.QuorumCertificate, error) {
	return s.Snapshot.QuorumCertificate, nil
}

func (s *Snapshot) Identities(selector flow.IdentityFilter) (flow.IdentityList, error) {
	return s.Snapshot.Identities.Filter(selector), nil
}

func (s *Snapshot) Identity(nodeID flow.Identifier) (*flow.Identity, error) {
	identity, ok := s.Snapshot.Identities.ByNodeID(nodeID)
	if !ok {
		return nil, protocol.IdentityNotFoundError{NodeID: nodeID}
	}
	return identity, nil
}

func (s *Snapshot) Commit() (flow.StateCommitment, error) {
	return s.Snapshot.Commit, nil
}

func (s *Snapshot) Pending() ([]flow.Identifier, error) {
	// canonical snapshots don't have any pending blocks
	return nil, nil
}

func (s *Snapshot) Phase() (flow.EpochPhase, error) {
	return s.Snapshot.Phase, nil
}

func (s *Snapshot) Seed(indices ...uint32) ([]byte, error) {
	return seed.FromParentSignature(indices, s.Snapshot.QuorumCertificate.SigData)
}

func (s *Snapshot) Epochs() protocol.EpochQuery {
	return Epochs{s.Snapshot.Epochs}
}
