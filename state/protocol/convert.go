package protocol

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

// ToEpochSetup converts an Epoch interface instance to the underlying
// concrete epoch setup service event.
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
		Counter:      counter,
		FirstView:    firstView,
		FinalView:    finalView,
		Participants: participants,
		Assignments:  assignments,
		RandomSource: randomSource,
	}
	return setup, nil
}

// ToEpochCommit converts an Epoch interface instance to the underlying
// concrete epoch commit service event. The epoch must have been committed.
func ToEpochCommit(epoch Epoch) (*flow.EpochCommit, error) {
	counter, err := epoch.Counter()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch counter: %w", err)
	}
	clustering, err := epoch.Clustering()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch clustering: %w", err)
	}
	qcs := make([]*flow.QuorumCertificate, 0, len(clustering))
	for i := range clustering {
		cluster, err := epoch.Cluster(uint(i))
		if err != nil {
			return nil, fmt.Errorf("could not get epoch cluster (index=%d): %w", i, err)
		}
		qcs = append(qcs, cluster.RootQC())
	}
	participants, err := epoch.InitialIdentities()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch participants: %w", err)
	}
	dkg, err := epoch.DKG()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch dkg: %w", err)
	}
	dkgParticipants, err := ToDKGParticipantLookup(dkg, participants.Filter(filter.HasRole(flow.RoleConsensus)))
	if err != nil {
		return nil, fmt.Errorf("could not compute dkg participant lookup: %w", err)
	}

	commit := &flow.EpochCommit{
		Counter:         counter,
		ClusterQCs:      qcs,
		DKGGroupKey:     dkg.GroupKey(),
		DKGParticipants: dkgParticipants,
	}
	return commit, nil
}

// ToDKGParticipantLookup computes the nodeID -> DKGParticipant lookup for a
// DKG instance. The participants must exactly match the DKG instance configuration.
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
