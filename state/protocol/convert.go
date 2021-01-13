package protocol

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

// TODO error handling
func ToEpochSetup(epoch Epoch) (*flow.EpochSetup, error) {
	counter, err := epoch.Counter()
	if err != nil {
		return nil, err
	}
	firstView, err := epoch.FirstView()
	finalView, err := epoch.FinalView()
	participants, err := epoch.InitialIdentities()
	clustering, err := epoch.Clustering()
	assignments := clustering.Assignments()
	src, err := epoch.RandomSource()

	setup := &flow.EpochSetup{
		Counter:      counter,
		FirstView:    firstView,
		FinalView:    finalView,
		Participants: participants,
		Assignments:  assignments,
		RandomSource: src,
	}
	return setup, nil
}

// TODO error handling
func ToEpochCommit(epoch Epoch) (*flow.EpochCommit, error) {
	counter, err := epoch.Counter()
	if err != nil {
		return nil, err
	}
	clustering, err := epoch.Clustering()
	qcs := make([]*flow.QuorumCertificate, 0, len(clustering))
	for i := range clustering {
		cluster, err := epoch.Cluster(uint(i))
		if err != nil {
			return nil, err
		}
		qcs = append(qcs, cluster.RootQC())
	}
	participants, err := epoch.InitialIdentities()
	dkg, err := epoch.DKG()
	dkgParticipants, err := ToDKGParticipantLookup(dkg, participants.Filter(filter.HasRole(flow.RoleConsensus)))

	commit := &flow.EpochCommit{
		Counter:         counter,
		ClusterQCs:      qcs,
		DKGGroupKey:     dkg.GroupKey(),
		DKGParticipants: dkgParticipants,
	}
	return commit, nil
}

// TODO doc
func ToDKGParticipantLookup(dkg DKG, participants flow.IdentityList) (map[flow.Identifier]flow.DKGParticipant, error) {

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
