package inmem

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
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
		ChainID:              from.ChainID(),
		SporkID:              from.SporkID(),
		SporkRootBlockHeight: from.SporkRootBlockHeight(),
		SporkRootBlockView:   from.SporkRootBlockView(),
	}
	return &Params{params}, nil
}

// ClusterFromEncodable returns a Cluster backed by the given encodable representation.
func ClusterFromEncodable(enc EncodableCluster) (*Cluster, error) {
	return &Cluster{enc}, nil
}

// SnapshotFromBootstrapStateWithParams is SnapshotFromBootstrapState
// with a caller-specified protocol version.
func SnapshotFromBootstrapStateWithParams(
	root *flow.Block,
	result *flow.ExecutionResult,
	seal *flow.Seal,
	qc *flow.QuorumCertificate,
	kvStoreFactory func(epochStateID flow.Identifier) (protocol_state.KVStoreAPI, error),
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
		ChainID:              root.ChainID, // chain ID must match the root block
		SporkID:              root.ID(),    // use root block ID as the unique spork identifier
		SporkRootBlockHeight: root.Height,  // use root block height as the spork root block height
		SporkRootBlockView:   root.View,    // use root block view as the spork root block view
	}

	rootMinEpochState, err := EpochProtocolStateFromServiceEvents(setup, commit)
	if err != nil {
		return nil, fmt.Errorf("could not construct epoch protocol state: %w", err)
	}
	rootEpochStateID := rootMinEpochState.ID()
	rootKvStore, err := kvStoreFactory(rootEpochStateID)
	if err != nil {
		return nil, fmt.Errorf("could not construct root kvstore: %w", err)
	}
	if rootKvStore.ID() != root.Payload.ProtocolStateID {
		return nil, fmt.Errorf("incorrect protocol state ID in root block, expected (%x) but got (%x)",
			root.Payload.ProtocolStateID, rootKvStore.ID())
	}
	kvStoreVersion, kvStoreData, err := rootKvStore.VersionedEncode()
	if err != nil {
		return nil, fmt.Errorf("could not encode kvstore: %w", err)
	}

	rootEpochState, err := flow.NewEpochStateEntry(
		flow.UntrustedEpochStateEntry{
			MinEpochStateEntry:  rootMinEpochState,
			PreviousEpochSetup:  nil,
			PreviousEpochCommit: nil,
			CurrentEpochSetup:   setup,
			CurrentEpochCommit:  commit,
			NextEpochSetup:      nil,
			NextEpochCommit:     nil,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("could not construct root epoch state entry: %w", err)
	}
	richRootEpochState, err := flow.NewRichEpochStateEntry(rootEpochState)
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

	proposal, err := flow.NewRootProposal(
		flow.UntrustedProposal{
			Block:           *root,
			ProposerSigData: nil,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("could not construct root proposal: %w", err)
	}

	snap := SnapshotFromEncodable(EncodableSnapshot{
		SealingSegment: &flow.SealingSegment{
			Blocks: []*flow.Proposal{
				proposal,
			},
			ExecutionResults: flow.ExecutionResultList{result},
			LatestSeals:      map[flow.Identifier]flow.Identifier{root.ID(): seal.ID()},
			ProtocolStateEntries: map[flow.Identifier]*flow.ProtocolStateEntryWrapper{
				rootKvStore.ID(): rootProtocolStateEntryWrapper,
			},
			FirstSeal:      seal,
			ExtraBlocks:    make([]*flow.Proposal, 0),
			SporkRootBlock: root,
		},
		QuorumCertificate:   qc,
		Params:              params,
		SealedVersionBeacon: nil,
	})

	return snap, nil
}

// EpochProtocolStateFromServiceEvents generates a protocol.MinEpochStateEntry for a root protocol state which is used for bootstrapping.
//
// CONTEXT: The EpochSetup event contains the IdentitySkeletons for each participant, thereby specifying active epoch members.
// While ejection status is not part of the EpochSetup event, we can supplement this information as follows:
//   - Per convention, service events are delivered (asynchronously) in an *order-preserving* manner. Furthermore,
//     node ejection is also mediated by system smart contracts and delivered via service events.
//   - Therefore, the EpochSetup event contains the up-to-date snapshot of the epoch participants. Any node ejection
//     that happened before should be reflected in the EpochSetup event. Specifically, ejected
//     nodes should be no longer listed in the EpochSetup event.
//     Hence, when the EpochSetup event is emitted / processed, the ejected flag is false for all epoch participants.
func EpochProtocolStateFromServiceEvents(setup *flow.EpochSetup, commit *flow.EpochCommit) (*flow.MinEpochStateEntry, error) {
	identities := make(flow.DynamicIdentityEntryList, 0, len(setup.Participants))
	for _, identity := range setup.Participants {
		identities = append(identities, &flow.DynamicIdentityEntry{
			NodeID:  identity.NodeID,
			Ejected: false,
		})
	}
	currentEpoch, err := flow.NewEpochStateContainer(
		flow.UntrustedEpochStateContainer{
			SetupID:          setup.ID(),
			CommitID:         commit.ID(),
			ActiveIdentities: identities,
			EpochExtensions:  nil,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("could not construct current epoch state: %w", err)
	}

	return flow.NewMinEpochStateEntry(
		flow.UntrustedMinEpochStateEntry{
			PreviousEpoch:          nil,
			CurrentEpoch:           *currentEpoch,
			NextEpoch:              nil,
			EpochFallbackTriggered: false,
		},
	)
}
