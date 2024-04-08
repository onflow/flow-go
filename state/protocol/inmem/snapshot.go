package inmem

import (
	"errors"
	"fmt"
	"github.com/onflow/flow-go/state/protocol/protocol_state/kvstore"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
)

// Snapshot is a memory-backed implementation of protocol.Snapshot. The snapshot
// data is stored in the embedded encodable snapshot model, which defines the
// canonical structure of an encoded snapshot for the purposes of serialization.
type Snapshot struct {
	enc EncodableSnapshot
}

var _ protocol.Snapshot = (*Snapshot)(nil)

func (s Snapshot) Head() (*flow.Header, error) {
	return s.enc.Head, nil
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
	return s.enc.EpochProtocolState.EpochPhase(), nil
}

func (s Snapshot) RandomSource() ([]byte, error) {
	return model.BeaconSignature(s.enc.QuorumCertificate)
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

func (s Snapshot) EpochProtocolState() (protocol.DynamicProtocolState, error) {
	epochs := s.Epochs()
	previous := epochs.Previous()
	current := epochs.Current()
	next := epochs.Next()
	var (
		err                                                      error
		previousEpochSetup, currentEpochSetup, nextEpochSetup    *flow.EpochSetup
		previousEpochCommit, currentEpochCommit, nextEpochCommit *flow.EpochCommit
	)

	if _, err := previous.Counter(); err == nil {
		// if there is a previous epoch, both setup and commit events must exist
		previousEpochSetup, err = protocol.ToEpochSetup(previous)
		if err != nil {
			return nil, fmt.Errorf("could not get previous epoch setup event: %w", err)
		}
		previousEpochCommit, err = protocol.ToEpochCommit(previous)
		if err != nil {
			return nil, fmt.Errorf("could not get previous epoch commit event: %w", err)
		}
	}

	// insert current epoch - both setup and commit events must exist
	currentEpochSetup, err = protocol.ToEpochSetup(current)
	if err != nil {
		return nil, fmt.Errorf("could not get current epoch setup event: %w", err)
	}
	currentEpochCommit, err = protocol.ToEpochCommit(current)
	if err != nil {
		return nil, fmt.Errorf("could not get current epoch commit event: %w", err)
	}

	if _, err := next.Counter(); err == nil {
		// if there is a next epoch, both setup event should exist, but commit event may not
		nextEpochSetup, err = protocol.ToEpochSetup(next)
		if err != nil {
			return nil, fmt.Errorf("could not get next epoch setup event: %w", err)
		}
		nextEpochCommit, err = protocol.ToEpochCommit(next)
		if err != nil && !errors.Is(err, protocol.ErrNextEpochNotCommitted) {
			return nil, fmt.Errorf("could not get next epoch commit event: %w", err)
		}
	}

	protocolStateEntry, err := flow.NewRichProtocolStateEntry(
		s.enc.EpochProtocolState,
		previousEpochSetup,
		previousEpochCommit,
		currentEpochSetup,
		currentEpochCommit,
		nextEpochSetup,
		nextEpochCommit)
	if err != nil {
		return nil, fmt.Errorf("could not create protocol state entry: %w", err)
	}

	return NewDynamicProtocolStateAdapter(protocolStateEntry, s.Params()), nil
}

func (s Snapshot) ProtocolState() (protocol.KVStoreReader, error) {
	return kvstore.VersionedDecode(s.enc.KVStore.Version, s.enc.KVStore.Data)
}

func (s Snapshot) VersionBeacon() (*flow.SealedVersionBeacon, error) {
	return s.enc.SealedVersionBeacon, nil
}

func SnapshotFromEncodable(enc EncodableSnapshot) *Snapshot {
	return &Snapshot{
		enc: enc,
	}
}
