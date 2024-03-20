package protocol

import (
	"fmt"

	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/signature"
)

// ToEpochSetup converts an Epoch interface instance to the underlying concrete
// epoch setup service event. The input must be a valid, set up epoch.
// Error returns:
// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
func ToEpochSetup(epoch Epoch) (*flow.EpochSetup, error) {
	counter, err := epoch.Counter()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch counter: %w", err)
	}
	firstView, err := epoch.FirstView()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch first view: %w", err)
	}
	finalView, err := epoch.FinalView()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch final view: %w", err)
	}
	dkgPhase1FinalView, dkgPhase2FinalView, dkgPhase3FinalView, err := DKGPhaseViews(epoch)
	if err != nil {
		return nil, fmt.Errorf("could not get epoch dkg final views: %w", err)
	}
	targetDuration, err := epoch.TargetDuration()
	if err != nil {
		return nil, fmt.Errorf("could not get target duration: %w", err)
	}
	targetEndTime, err := epoch.TargetEndTime()
	if err != nil {
		return nil, fmt.Errorf("could not get target end time: %w", err)
	}
	participants, err := epoch.InitialIdentities()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch participants: %w", err)
	}
	clustering, err := epoch.Clustering()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch clustering: %w", err)
	}
	assignments := clustering.Assignments()
	randomSource, err := epoch.RandomSource()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch random source: %w", err)
	}

	setup := &flow.EpochSetup{
		Counter:            counter,
		FirstView:          firstView,
		DKGPhase1FinalView: dkgPhase1FinalView,
		DKGPhase2FinalView: dkgPhase2FinalView,
		DKGPhase3FinalView: dkgPhase3FinalView,
		FinalView:          finalView,
		Participants:       participants,
		Assignments:        assignments,
		RandomSource:       randomSource,
		TargetDuration:     targetDuration,
		TargetEndTime:      targetEndTime,
	}
	return setup, nil
}

// ToEpochCommit converts an Epoch interface instance to the underlying
// concrete epoch commit service event. The epoch must have been committed.
// Error returns:
// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
// * protocol.ErrNextEpochNotCommitted - if the epoch has not been committed.
// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
func ToEpochCommit(epoch Epoch) (*flow.EpochCommit, error) {
	counter, err := epoch.Counter()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch counter: %w", err)
	}
	clustering, err := epoch.Clustering()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch clustering: %w", err)
	}
	qcs := make([]*flow.QuorumCertificateWithSignerIDs, 0, len(clustering))
	for i := range clustering {
		cluster, err := epoch.Cluster(uint(i))
		if err != nil {
			return nil, fmt.Errorf("could not get epoch cluster (index=%d): %w", i, err)
		}
		qc := cluster.RootQC()
		// TODO: double check cluster.Members returns canonical order
		signerIDs, err := signature.DecodeSignerIndicesToIdentifiers(cluster.Members().NodeIDs(), qc.SignerIndices)
		if err != nil {
			return nil, fmt.Errorf("could not encode signer indices: %w", err)
		}
		qcs = append(qcs, &flow.QuorumCertificateWithSignerIDs{
			View:      qc.View,
			BlockID:   qc.BlockID,
			SignerIDs: signerIDs,
			SigData:   qc.SigData,
		})
	}

	participants, err := epoch.InitialIdentities()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch participants: %w", err)
	}
	dkg, err := epoch.DKG()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch dkg: %w", err)
	}
	dkgParticipantKeys, err := GetDKGParticipantKeys(dkg, participants.Filter(filter.IsValidDKGParticipant))
	if err != nil {
		return nil, fmt.Errorf("could not get dkg participant keys: %w", err)
	}

	commit := &flow.EpochCommit{
		Counter:            counter,
		ClusterQCs:         flow.ClusterQCVoteDatasFromQCs(qcs),
		DKGGroupKey:        dkg.GroupKey(),
		DKGParticipantKeys: dkgParticipantKeys,
	}
	return commit, nil
}

// GetDKGParticipantKeys retrieves the canonically ordered list of DKG
// participant keys from the DKG.
// All errors indicate inconsistent or invalid inputs.
// No errors are expected during normal operation.
func GetDKGParticipantKeys(dkg DKG, participants flow.IdentitySkeletonList) ([]crypto.PublicKey, error) {

	keys := make([]crypto.PublicKey, 0, len(participants))
	for i, identity := range participants {

		index, err := dkg.Index(identity.NodeID)
		if err != nil {
			return nil, fmt.Errorf("could not get index (node=%x): %w", identity.NodeID, err)
		}
		key, err := dkg.KeyShare(identity.NodeID)
		if err != nil {
			return nil, fmt.Errorf("could not get key share (node=%x): %w", identity.NodeID, err)
		}
		if uint(i) != index {
			return nil, fmt.Errorf("participant list index (%d) does not match dkg index (%d)", i, index)
		}

		keys = append(keys, key)
	}

	return keys, nil
}

// ToDKGParticipantLookup computes the nodeID -> DKGParticipant lookup for a
// DKG instance. The participants must exactly match the DKG instance configuration.
// All errors indicate inconsistent or invalid inputs.
// No errors are expected during normal operation.
func ToDKGParticipantLookup(dkg DKG, participants flow.IdentitySkeletonList) (map[flow.Identifier]flow.DKGParticipant, error) {

	lookup := make(map[flow.Identifier]flow.DKGParticipant)
	for _, identity := range participants {

		index, err := dkg.Index(identity.NodeID)
		if err != nil {
			return nil, fmt.Errorf("could not get index (node=%x): %w", identity.NodeID, err)
		}
		key, err := dkg.KeyShare(identity.NodeID)
		if err != nil {
			return nil, fmt.Errorf("could not get key share (node=%x): %w", identity.NodeID, err)
		}

		lookup[identity.NodeID] = flow.DKGParticipant{
			Index:    index,
			KeyShare: key,
		}
	}

	return lookup, nil
}

// DKGPhaseViews returns the DKG final phase views for an epoch.
// Error returns:
// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
// * protocol.ErrNextEpochNotCommitted - if the epoch has not been committed.
// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
func DKGPhaseViews(epoch Epoch) (phase1FinalView uint64, phase2FinalView uint64, phase3FinalView uint64, err error) {
	phase1FinalView, err = epoch.DKGPhase1FinalView()
	if err != nil {
		return
	}
	phase2FinalView, err = epoch.DKGPhase2FinalView()
	if err != nil {
		return
	}
	phase3FinalView, err = epoch.DKGPhase3FinalView()
	if err != nil {
		return
	}
	return
}
