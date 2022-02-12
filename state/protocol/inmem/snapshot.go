package inmem

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/seed"
)

var (
	_ protocol.Snapshot     = new(Snapshot)
	_ protocol.GlobalParams = new(Params)
	_ protocol.EpochQuery   = new(Epochs)
	_ protocol.Epoch        = new(Epoch)
	_ protocol.Cluster      = new(Cluster)
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

func (s Snapshot) SealingSegment() (*flow.SealingSegment, error) {
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

func (s Snapshot) RandomSource() ([]byte, error) {
	return seed.FromParentQCSignature(s.enc.QuorumCertificate.SigData)
}

func (s Snapshot) Epochs() protocol.EpochQuery {
	return Epochs{s.enc.Epochs}
}

func (s Snapshot) Params() protocol.GlobalParams {
	return Params{s.enc.Params}
}

func (s Snapshot) Encodable() EncodableSnapshot {
	return s.enc
}

func SnapshotFromEncodable(enc EncodableSnapshot) *Snapshot {
	return &Snapshot{
		enc: enc,
	}
}

// StrippedInmemSnapshot removes all the networking address in the snapshot
func StrippedInmemSnapshot(snapshot EncodableSnapshot) EncodableSnapshot {
	removeAddress := func(ids flow.IdentityList) {
		for _, identity := range ids {
			identity.Address = ""
		}
	}

	removeAddressFromEpoch := func(epoch *EncodableEpoch) {
		if epoch == nil {
			return
		}
		removeAddress(epoch.InitialIdentities)
		for _, cluster := range epoch.Clustering {
			removeAddress(cluster)
		}
		for _, c := range epoch.Clusters {
			removeAddress(c.Members)
		}
	}

	removeAddress(snapshot.Identities)
	removeAddressFromEpoch(snapshot.Epochs.Previous)
	removeAddressFromEpoch(&snapshot.Epochs.Current)
	removeAddressFromEpoch(snapshot.Epochs.Next)

	for _, event := range snapshot.LatestResult.ServiceEvents {
		switch event.Type {
		case flow.ServiceEventSetup:
			removeAddress(event.Event.(*flow.EpochSetup).Participants)
		}
	}
	return snapshot
}
