package inmem

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state/kvstore"
)

// Snapshot is a memory-backed implementation of protocol.Snapshot. The snapshot
// data is stored in the embedded encodable snapshot model, which defines the
// canonical structure of an encoded snapshot for the purposes of serialization.
type Snapshot struct {
	enc EncodableSnapshot
}

var _ protocol.Snapshot = (*Snapshot)(nil)

func (s Snapshot) Head() (*flow.Header, error) {
	return s.enc.GetHead(), nil
}

func (s Snapshot) QuorumCertificate() (*flow.QuorumCertificate, error) {
	return s.enc.QuorumCertificate, nil
}

func (s Snapshot) Identities(selector flow.IdentityFilter[flow.Identity]) (flow.IdentityList, error) {
	protocolState, err := s.EpochProtocolState()
	if err != nil {
		return nil, fmt.Errorf("could not access protocol state: %w", err)
	}
	return protocolState.Identities().Filter(selector), nil
}

func (s Snapshot) Identity(nodeID flow.Identifier) (*flow.Identity, error) {
	// filter identities at snapshot for node ID
	identities, err := s.Identities(filter.HasNodeID[flow.Identity](nodeID))
	if err != nil {
		return nil, fmt.Errorf("could not get identities: %w", err)
	}

	// check if node ID is part of identities
	if len(identities) == 0 {
		return nil, protocol.IdentityNotFoundError{NodeID: nodeID}
	}
	return identities[0], nil
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

func (s Snapshot) Phase() (flow.EpochPhase, error) {
	epochProtocolState, err := s.EpochProtocolState()
	if err != nil {
		return flow.EpochPhaseUndefined, fmt.Errorf("could not get epoch protocol state: %w", err)
	}
	return epochProtocolState.EpochPhase(), nil
}

func (s Snapshot) RandomSource() ([]byte, error) {
	return model.BeaconSignature(s.enc.QuorumCertificate)
}

func (s Snapshot) Epochs() protocol.EpochQuery {
	return Epochs{
		entry: *s.enc.SealingSegment.LatestProtocolStateEntry().EpochEntry,
	}
}

func (s Snapshot) Params() protocol.GlobalParams {
	return Params{s.enc.Params}
}

func (s Snapshot) Encodable() EncodableSnapshot {
	return s.enc
}

func (s Snapshot) EpochProtocolState() (protocol.DynamicProtocolState, error) {
	entry := s.enc.SealingSegment.LatestProtocolStateEntry()
	return NewDynamicProtocolStateAdapter(entry.EpochEntry, s.Params()), nil
}

func (s Snapshot) ProtocolState() (protocol.KVStoreReader, error) {
	entry := s.enc.SealingSegment.LatestProtocolStateEntry()
	return kvstore.VersionedDecode(entry.KVStore.Version, entry.KVStore.Data)
}

func (s Snapshot) VersionBeacon() (*flow.SealedVersionBeacon, error) {
	return s.enc.SealedVersionBeacon, nil
}

func SnapshotFromEncodable(enc EncodableSnapshot) *Snapshot {
	return &Snapshot{
		enc: enc,
	}
}
