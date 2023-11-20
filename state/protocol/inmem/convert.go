package inmem

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/state/protocol"
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
	snap.Head, err = from.Head()
	if err != nil {
		return nil, fmt.Errorf("could not get head: %w", err)
	}
	snap.Identities, err = from.Identities(filter.Any)
	if err != nil {
		return nil, fmt.Errorf("could not get identities: %w", err)
	}
	snap.LatestResult, snap.LatestSeal, err = from.SealedResult()
	if err != nil {
		return nil, fmt.Errorf("could not get seal: %w", err)
	}

	snap.SealingSegment, err = from.SealingSegment()
	if err != nil {
		return nil, fmt.Errorf("could not get sealing segment: %w", err)
	}
	snap.QuorumCertificate, err = from.QuorumCertificate()
	if err != nil {
		return nil, fmt.Errorf("could not get qc: %w", err)
	}
	snap.Phase, err = from.Phase()
	if err != nil {
		return nil, fmt.Errorf("could not get phase: %w", err)
	}

	// convert epochs
	previous, err := FromEpoch(from.Epochs().Previous())
	// it is possible for valid snapshots to have no previous epoch
	if errors.Is(err, protocol.ErrNoPreviousEpoch) {
		snap.Epochs.Previous = nil
	} else if err != nil {
		return nil, fmt.Errorf("could not get previous epoch: %w", err)
	} else {
		snap.Epochs.Previous = &previous.enc
	}

	current, err := FromEpoch(from.Epochs().Current())
	if err != nil {
		return nil, fmt.Errorf("could not get current epoch: %w", err)
	}
	snap.Epochs.Current = current.enc

	next, err := FromEpoch(from.Epochs().Next())
	// it is possible for valid snapshots to have no next epoch
	if errors.Is(err, protocol.ErrNextEpochNotSetup) {
		snap.Epochs.Next = nil
	} else if err != nil {
		return nil, fmt.Errorf("could not get next epoch: %w", err)
	} else {
		snap.Epochs.Next = &next.enc
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
	var (
		params EncodableParams
		err    error
	)

	params.ChainID, err = from.ChainID()
	if err != nil {
		return nil, fmt.Errorf("could not get chain id: %w", err)
	}
	params.SporkID, err = from.SporkID()
	if err != nil {
		return nil, fmt.Errorf("could not get spork id: %w", err)
	}
	params.SporkRootBlockHeight, err = from.SporkRootBlockHeight()
	if err != nil {
		return nil, fmt.Errorf("could not get spork root block height: %w", err)
	}
	params.ProtocolVersion, err = from.ProtocolVersion()
	if err != nil {
		return nil, fmt.Errorf("could not get protocol version: %w", err)
	}
	params.EpochCommitSafetyThreshold, err = from.EpochCommitSafetyThreshold()
	if err != nil {
		return nil, fmt.Errorf("could not get protocol version: %w", err)
	}

	return &Params{params}, nil
}

// FromEpoch converts any protocol.Epoch to a memory-backed Epoch.
// Error returns:
// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
func FromEpoch(from protocol.Epoch) (*Epoch, error) {
	var (
		epoch EncodableEpoch
		err   error
	)

	// convert top-level fields
	epoch.Counter, err = from.Counter()
	if err != nil {
		return nil, fmt.Errorf("could not get counter: %w", err)
	}
	epoch.InitialIdentities, err = from.InitialIdentities()
	if err != nil {
		return nil, fmt.Errorf("could not get initial identities: %w", err)
	}
	epoch.FirstView, err = from.FirstView()
	if err != nil {
		return nil, fmt.Errorf("could not get first view: %w", err)
	}
	epoch.FinalView, err = from.FinalView()
	if err != nil {
		return nil, fmt.Errorf("could not get final view: %w", err)
	}
	epoch.RandomSource, err = from.RandomSource()
	if err != nil {
		return nil, fmt.Errorf("could not get random source: %w", err)
	}
	epoch.TargetDuration, err = from.TargetDuration()
	if err != nil {
		return nil, fmt.Errorf("could not get target duration: %w", err)
	}
	epoch.TargetEndTime, err = from.TargetEndTime()
	if err != nil {
		return nil, fmt.Errorf("could not get target end time: %w", err)
	}
	epoch.DKGPhase1FinalView, epoch.DKGPhase2FinalView, epoch.DKGPhase3FinalView, err = protocol.DKGPhaseViews(from)
	if err != nil {
		return nil, fmt.Errorf("could not get dkg final views")
	}
	clustering, err := from.Clustering()
	if err != nil {
		return nil, fmt.Errorf("could not get clustering: %w", err)
	}
	epoch.Clustering = clustering

	// convert dkg
	dkg, err := from.DKG()
	// if this epoch hasn't been committed yet, return the epoch as-is
	if errors.Is(err, protocol.ErrNextEpochNotCommitted) {
		return &Epoch{epoch}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not get dkg: %w", err)
	}
	convertedDKG, err := FromDKG(dkg, epoch.InitialIdentities.Filter(filter.HasRole(flow.RoleConsensus)))
	if err != nil {
		return nil, err
	}
	epoch.DKG = &convertedDKG.enc

	// convert clusters
	for index := range clustering {
		cluster, err := from.Cluster(uint(index))
		if err != nil {
			return nil, fmt.Errorf("could not get cluster %d: %w", index, err)
		}
		convertedCluster, err := FromCluster(cluster)
		if err != nil {
			return nil, fmt.Errorf("could not convert cluster %d: %w", index, err)
		}
		epoch.Clusters = append(epoch.Clusters, convertedCluster.enc)
	}

	// convert height bounds
	firstHeight, err := from.FirstHeight()
	if errors.Is(err, protocol.ErrEpochTransitionNotFinalized) {
		// if this epoch hasn't been started yet, return the epoch as-is
		return &Epoch{epoch}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not get first height: %w", err)
	}
	epoch.FirstHeight = &firstHeight
	finalHeight, err := from.FinalHeight()
	if errors.Is(err, protocol.ErrEpochTransitionNotFinalized) {
		// if this epoch hasn't ended yet, return the epoch as-is
		return &Epoch{epoch}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not get final height: %w", err)
	}
	epoch.FinalHeight = &finalHeight

	return &Epoch{epoch}, nil
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
func FromDKG(from protocol.DKG, participants flow.IdentityList) (*DKG, error) {
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
	return SnapshotFromBootstrapStateWithParams(root, result, seal, qc, version, threshold)
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
	encodable, err := FromEpoch(NewStartedEpoch(setup, commit, root.Header.Height))
	if err != nil {
		return nil, fmt.Errorf("could not convert epoch: %w", err)
	}
	epochs := EncodableEpochs{
		Current: encodable.enc,
	}

	params := EncodableParams{
		ChainID:                    root.Header.ChainID,        // chain ID must match the root block
		SporkID:                    root.ID(),                  // use root block ID as the unique spork identifier
		SporkRootBlockHeight:       root.Header.Height,         // use root block height as the spork root block height
		ProtocolVersion:            protocolVersion,            // major software version for this spork
		EpochCommitSafetyThreshold: epochCommitSafetyThreshold, // see protocol.Params for details
	}

	snap := SnapshotFromEncodable(EncodableSnapshot{
		Head:         root.Header,
		Identities:   setup.Participants,
		LatestSeal:   seal,
		LatestResult: result,
		SealingSegment: &flow.SealingSegment{
			Blocks:           []*flow.Block{root},
			ExecutionResults: flow.ExecutionResultList{result},
			LatestSeals:      map[flow.Identifier]flow.Identifier{root.ID(): seal.ID()},
			FirstSeal:        seal,
			ExtraBlocks:      make([]*flow.Block, 0),
		},
		QuorumCertificate:   qc,
		Phase:               flow.EpochPhaseStaking,
		Epochs:              epochs,
		Params:              params,
		SealedVersionBeacon: nil,
	})
	return snap, nil
}
