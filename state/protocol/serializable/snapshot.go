package serializable

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/seed"
)

var (
	_ protocol.Snapshot   = new(snapshot)
	_ protocol.EpochQuery = new(epochQuery)
	_ protocol.Epoch      = new(epoch)
	_ protocol.Cluster    = new(cluster)
)

type snapshot struct {
	head       *flow.Header
	identities flow.IdentityList
	commit     flow.StateCommitment
	qc         *flow.QuorumCertificate
	phase      flow.EpochPhase
	epochs     *epochQuery
}

func (s *snapshot) Head() (*flow.Header, error) {
	return s.head, nil
}

func (s *snapshot) QuorumCertificate() (*flow.QuorumCertificate, error) {
	return s.qc, nil
}

func (s *snapshot) Identities(selector flow.IdentityFilter) (flow.IdentityList, error) {
	return s.identities.Filter(selector), nil
}

func (s *snapshot) Identity(nodeID flow.Identifier) (*flow.Identity, error) {
	identity, ok := s.identities.ByNodeID(nodeID)
	if !ok {
		return nil, protocol.IdentityNotFoundError{NodeID: nodeID}
	}
	return identity, nil
}

func (s *snapshot) Commit() (flow.StateCommitment, error) {
	return s.commit, nil
}

func (s *snapshot) Pending() ([]flow.Identifier, error) {
	// canonical snapshots don't consider the pending tree
	return nil, nil
}

func (s *snapshot) Phase() (flow.EpochPhase, error) {
	return s.phase, nil
}

func (s *snapshot) Seed(indices ...uint32) ([]byte, error) {
	return seed.FromParentSignature(indices, s.qc.SigData)
}

func (s *snapshot) Epochs() protocol.EpochQuery {
	return s.epochs
}
