package protocol

import (
	"fmt"

	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/signature"
)

// ToEpochSetup converts a CommittedEpoch interface instance to the underlying concrete
// epoch setup service event. The input must be a valid, set up epoch.
// CAUTION: this conversion only works for Epochs that have NO epoch-fallback EXTENSIONS
// Error returns:
// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
func ToEpochSetup(epoch CommittedEpoch) (*flow.EpochSetup, error) {
	clustering, err := epoch.Clustering()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch clustering: %w", err)
	}
	assignments := clustering.Assignments()

	setup := &flow.EpochSetup{
		Counter:            epoch.Counter(),
		FirstView:          epoch.FirstView(),
		DKGPhase1FinalView: epoch.DKGPhase1FinalView(),
		DKGPhase2FinalView: epoch.DKGPhase2FinalView(),
		DKGPhase3FinalView: epoch.DKGPhase3FinalView(),
		FinalView:          epoch.FinalView(),
		Participants:       epoch.InitialIdentities(),
		Assignments:        assignments,
		RandomSource:       epoch.RandomSource(),
		TargetDuration:     epoch.TargetDuration(),
		TargetEndTime:      epoch.TargetEndTime(),
	}
	return setup, nil
}

// ToEpochCommit converts a CommittedEpoch interface instance to the underlying
// concrete epoch commit service event. The epoch must have been committed.
// Error returns:
// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
// * protocol.ErrNextEpochNotCommitted - if the epoch has not been committed.
// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
func ToEpochCommit(epoch CommittedEpoch) (*flow.EpochCommit, error) {
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

	participants := epoch.InitialIdentities()
	dkg, err := epoch.DKG()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch dkg: %w", err)
	}
	dkgParticipantKeys, err := GetDKGParticipantKeys(dkg, participants.Filter(filter.IsValidDKGParticipant))
	if err != nil {
		return nil, fmt.Errorf("could not get dkg participant keys: %w", err)
	}

	commit := &flow.EpochCommit{
		Counter:            epoch.Counter(),
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
