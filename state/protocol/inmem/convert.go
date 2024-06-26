package inmem

import (
	"fmt"

	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/state/protocol/protocol_state/kvstore"
)

// FromSnapshot generates a memory-backed snapshot from the input snapshot.
// Typically, this would be used to convert a database-backed snapshot to
// one that can easily be serialized to disk or to network.
// TODO error docs
func FromSnapshot(from protocol.Snapshot) (*Snapshot, error) {
	var (
		snap EncodableSnapshot
		err  error
	)

	// convert top-level fields
	snap.SealingSegment, err = from.SealingSegment()
	if err != nil {
		return nil, fmt.Errorf("could not get sealing segment: %w", err)
	}
	snap.QuorumCertificate, err = from.QuorumCertificate()
	if err != nil {
		return nil, fmt.Errorf("could not get qc: %w", err)
	}

	// convert global state parameters
	params, err := FromParams(from.Params())
	if err != nil {
		return nil, fmt.Errorf("could not get params: %w", err)
	}
	snap.Params = params.enc

	// convert version beacon
	versionBeacon, err := from.VersionBeacon()
	if err != nil {
		return nil, fmt.Errorf("could not get version beacon: %w", err)
	}

	snap.SealedVersionBeacon = versionBeacon

	return &Snapshot{snap}, nil
}

// FromParams converts any protocol.GlobalParams to a memory-backed Params.
// TODO error docs
func FromParams(from protocol.GlobalParams) (*Params, error) {
	params := EncodableParams{
		ChainID:                    from.ChainID(),
		SporkID:                    from.SporkID(),
		SporkRootBlockHeight:       from.SporkRootBlockHeight(),
		ProtocolVersion:            from.ProtocolVersion(),
		EpochCommitSafetyThreshold: from.EpochCommitSafetyThreshold(),
	}
	return &Params{params}, nil
}

// FromCluster converts any protocol.Cluster to a memory-backed Cluster.
// No errors are expected during normal operation.
func FromCluster(from protocol.Cluster) (*Cluster, error) {
	cluster := EncodableCluster{
		Counter:   from.EpochCounter(),
		Index:     from.Index(),
		Members:   from.Members(),
		RootBlock: from.RootBlock(),
		RootQC:    from.RootQC(),
	}
	return &Cluster{cluster}, nil
}

// FromDKG converts any protocol.DKG to a memory-backed DKG.
//
// The given participant list must exactly match the DKG members.
// All errors indicate inconsistent or invalid inputs.
// No errors are expected during normal operation.
func FromDKG(from protocol.DKG, participants flow.IdentitySkeletonList) (*DKG, error) {
	var dkg EncodableDKG
	dkg.GroupKey = encodable.RandomBeaconPubKey{PublicKey: from.GroupKey()}

	lookup, err := protocol.ToDKGParticipantLookup(from, participants)
	if err != nil {
		return nil, fmt.Errorf("could not generate dkg participant lookup: %w", err)
	}
	dkg.Participants = lookup

	return &DKG{dkg}, nil
}

// DKGFromEncodable returns a DKG backed by the given encodable representation.
func DKGFromEncodable(enc EncodableDKG) (*DKG, error) {
	return &DKG{enc}, nil
}

// EncodableDKGFromEvents returns an EncodableDKG constructed from epoch setup and commit events.
// No errors are expected during normal operations.
func EncodableDKGFromEvents(setup *flow.EpochSetup, commit *flow.EpochCommit) (EncodableDKG, error) {
	// filter initial participants to valid DKG participants
	participants := setup.Participants.Filter(filter.IsValidDKGParticipant)
	lookup, err := flow.ToDKGParticipantLookup(participants, commit.DKGParticipantKeys)
	if err != nil {
		return EncodableDKG{}, fmt.Errorf("could not construct dkg lookup: %w", err)
	}

	return EncodableDKG{
		GroupKey: encodable.RandomBeaconPubKey{
			PublicKey: commit.DKGGroupKey,
		},
		Participants: lookup,
	}, nil
}

// ClusterFromEncodable returns a Cluster backed by the given encodable representation.
func ClusterFromEncodable(enc EncodableCluster) (*Cluster, error) {
	return &Cluster{enc}, nil
}

// SnapshotFromBootstrapState generates a protocol.Snapshot representing a
// root bootstrap state. This is used to bootstrap the protocol state for
// genesis or post-spork states.
func SnapshotFromBootstrapState(root *flow.Block, result *flow.ExecutionResult, seal *flow.Seal, qc *flow.QuorumCertificate) (*Snapshot, error) {
	version := flow.DefaultProtocolVersion
	threshold, err := protocol.DefaultEpochCommitSafetyThreshold(root.Header.ChainID)
	if err != nil {
		return nil, fmt.Errorf("could not get default epoch commit safety threshold: %w", err)
	}
	return SnapshotFromBootstrapStateWithParams(root, result, seal, qc, version, threshold, kvstore.NewDefaultKVStore)
}

// SnapshotFromBootstrapStateWithParams is SnapshotFromBootstrapState
// with a caller-specified protocol version.
func SnapshotFromBootstrapStateWithParams(
	root *flow.Block,
	result *flow.ExecutionResult,
	seal *flow.Seal,
	qc *flow.QuorumCertificate,
	protocolVersion uint,
	epochCommitSafetyThreshold uint64,
	kvStoreFactory func(epochStateID flow.Identifier) protocol_state.KVStoreAPI,
) (*Snapshot, error) {
	setup, ok := result.ServiceEvents[0].Event.(*flow.EpochSetup)
	if !ok {
		return nil, fmt.Errorf("invalid setup event type (%T)", result.ServiceEvents[0].Event)
	}
	commit, ok := result.ServiceEvents[1].Event.(*flow.EpochCommit)
	if !ok {
		return nil, fmt.Errorf("invalid commit event type (%T)", result.ServiceEvents[1].Event)
	}

	clustering, err := ClusteringFromSetupEvent(setup)
	if err != nil {
		return nil, fmt.Errorf("setup event has invalid clustering: %w", err)
	}

	// sanity check the commit event has the same number of cluster QC as the number clusters
	if len(clustering) != len(commit.ClusterQCs) {
		return nil, fmt.Errorf("mismatching number of ClusterQCs, expect %v but got %v",
			len(clustering), len(commit.ClusterQCs))
	}

	// sanity check the QC in the commit event, which should be found in the identities in
	// the setup event
	for i, cluster := range clustering {
		rootQCVoteData := commit.ClusterQCs[i]
		_, err = signature.EncodeSignersToIndices(cluster.NodeIDs(), rootQCVoteData.VoterIDs)
		if err != nil {
			return nil, fmt.Errorf("mismatching cluster and qc: %w", err)
		}
	}

	params := EncodableParams{
		ChainID:                    root.Header.ChainID,        // chain ID must match the root block
		SporkID:                    root.ID(),                  // use root block ID as the unique spork identifier
		SporkRootBlockHeight:       root.Header.Height,         // use root block height as the spork root block height
		ProtocolVersion:            protocolVersion,            // major software version for this spork
		EpochCommitSafetyThreshold: epochCommitSafetyThreshold, // see protocol.Params for details
	}

	rootEpochState := EpochProtocolStateFromServiceEvents(setup, commit)
	rootEpochStateID := rootEpochState.ID()
	rootKvStore := kvStoreFactory(rootEpochStateID)
	if rootKvStore.ID() != root.Payload.ProtocolStateID {
		return nil, fmt.Errorf("incorrect protocol state ID in root block, expected (%x) but got (%x)",
			root.Payload.ProtocolStateID, rootKvStore.ID())
	}
	kvStoreVersion, kvStoreData, err := rootKvStore.VersionedEncode()
	if err != nil {
		return nil, fmt.Errorf("could not encode kvstore: %w", err)
	}

	richRootEpochState, err := flow.NewEpochRichStateEntry(rootEpochState, nil, nil, setup, commit, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("could not construct root rich epoch state entry: %w", err)
	}

	rootProtocolStateEntryWrapper := &flow.ProtocolStateEntryWrapper{
		KVStore: flow.PSKeyValueStoreData{
			Version: kvStoreVersion,
			Data:    kvStoreData,
		},
		EpochEntry: richRootEpochState,
	}

	snap := SnapshotFromEncodable(EncodableSnapshot{
		SealingSegment: &flow.SealingSegment{
			Blocks:           []*flow.Block{root},
			ExecutionResults: flow.ExecutionResultList{result},
			LatestSeals:      map[flow.Identifier]flow.Identifier{root.ID(): seal.ID()},
			ProtocolStateEntries: map[flow.Identifier]*flow.ProtocolStateEntryWrapper{
				rootKvStore.ID(): rootProtocolStateEntryWrapper,
			},
			FirstSeal:   seal,
			ExtraBlocks: make([]*flow.Block, 0),
		},
		QuorumCertificate:   qc,
		Params:              params,
		SealedVersionBeacon: nil,
	})

	return snap, nil
}

// EpochProtocolStateFromServiceEvents generates a protocol.EpochMinStateEntry for a root protocol state which is used for bootstrapping.
//
// CONTEXT: The EpochSetup event contains the IdentitySkeletons for each participant, thereby specifying active epoch members.
// While ejection status is not part of the EpochSetup event, we can supplement this information as follows:
//   - Per convention, service events are delivered (asynchronously) in an *order-preserving* manner. Furthermore,
//     node ejection is also mediated by system smart contracts and delivered via service events.
//   - Therefore, the EpochSetup event contains the up-to-date snapshot of the epoch participants. Any node ejection
//     that happened before should be reflected in the EpochSetup event. Specifically, ejected
//     nodes should be no longer listed in the EpochSetup event.
//     Hence, when the EpochSetup event is emitted / processed, the ejected flag is false for all epoch participants.
func EpochProtocolStateFromServiceEvents(setup *flow.EpochSetup, commit *flow.EpochCommit) *flow.EpochMinStateEntry {
	identities := make(flow.DynamicIdentityEntryList, 0, len(setup.Participants))
	for _, identity := range setup.Participants {
		identities = append(identities, &flow.DynamicIdentityEntry{
			NodeID:  identity.NodeID,
			Ejected: false,
		})
	}
	return &flow.EpochMinStateEntry{
		PreviousEpoch: nil,
		CurrentEpoch: flow.EpochStateContainer{
			SetupID:          setup.ID(),
			CommitID:         commit.ID(),
			ActiveIdentities: identities,
		},
		NextEpoch:              nil,
		EpochFallbackTriggered: false,
	}
}
