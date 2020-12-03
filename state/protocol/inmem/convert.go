package inmem

import (
	"errors"

	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
)

// FromSnapshot generates a memory-backed snapshot from the input snapshot.
// Typically, this would be used to convert a database-backed snapshot to
// one that can easily be serialized to disk or to network.
func FromSnapshot(from protocol.Snapshot) (*Snapshot, error) {

	var (
		snap EncodableSnapshot
		err  error
	)

	// convert top-level fields
	snap.Head, err = from.Head()
	if err != nil {
		return nil, err
	}
	snap.Identities, err = from.Identities(filter.Any)
	if err != nil {
		return nil, err
	}
	snap.Commit, err = from.Commit()
	if err != nil {
		return nil, err
	}
	snap.QuorumCertificate, err = from.QuorumCertificate()
	if err != nil {
		return nil, err
	}
	snap.Phase, err = from.Phase()
	if err != nil {
		return nil, err
	}

	// convert epochs
	previous, err := FromEpoch(from.Epochs().Previous())
	// it is possible for valid snapshots to have no previous epoch
	if errors.Is(err, protocol.ErrNoPreviousEpoch) {
		snap.Epochs.Previous = nil
	} else if err != nil {
		return nil, err
	} else {
		snap.Epochs.Previous = &previous.enc
	}

	current, err := FromEpoch(from.Epochs().Current())
	if err != nil {
		return nil, err
	}
	snap.Epochs.Current = &current.enc

	next, err := FromEpoch(from.Epochs().Next())
	// it is possible for valid snapshots to have no next epoch
	if errors.Is(err, protocol.ErrNextEpochNotSetup) {
		snap.Epochs.Next = nil
	} else if err != nil {
		return nil, err
	} else {
		snap.Epochs.Next = &next.enc
	}

	return &Snapshot{snap}, nil
}

// FromEpoch converts any protocol.Protocol to a memory-backed Epoch.
func FromEpoch(from protocol.Epoch) (*Epoch, error) {

	var (
		epoch EncodableEpoch
		err   error
	)

	// convert top-level fields
	epoch.Counter, err = from.Counter()
	if err != nil {
		return nil, err
	}
	epoch.InitialIdentities, err = from.InitialIdentities()
	if err != nil {
		return nil, err
	}
	epoch.FirstView, err = from.FirstView()
	if err != nil {
		return nil, err
	}
	epoch.FinalView, err = from.FinalView()
	if err != nil {
		return nil, err
	}
	epoch.RandomSource, err = from.RandomSource()
	if err != nil {
		return nil, err
	}

	// convert dkg
	dkg, err := from.DKG()
	// if this epoch hasn't been committed yet, return the epoch as-is
	if errors.Is(err, protocol.ErrEpochNotCommitted) {
		return &Epoch{epoch}, nil
	}
	if err != nil {
		return nil, err
	}
	convertedDKG, err := FromDKG(dkg, epoch.InitialIdentities.Filter(filter.HasRole(flow.RoleConsensus)))
	if err != nil {
		return nil, err
	}
	epoch.DKG = &convertedDKG.enc

	// convert clusters
	clustering, err := from.Clustering()
	if err != nil {
		return nil, err
	}
	for index := range clustering {
		cluster, err := from.Cluster(uint(index))
		if err != nil {
			return nil, err
		}
		convertedCluster, err := FromCluster(cluster)
		if err != nil {
			return nil, err
		}
		epoch.Clusters = append(epoch.Clusters, convertedCluster.enc)
	}

	return &Epoch{epoch}, nil
}

// FromCluster converts any protocol.Cluster to a memory-backed Cluster
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
func FromDKG(from protocol.DKG, participants flow.IdentityList) (*DKG, error) {

	var dkg EncodableDKG

	dkg.GroupKey = encodable.RandomBeaconPubKey{PublicKey: from.GroupKey()}
	dkg.Participants = make(map[flow.Identifier]flow.DKGParticipant)
	for _, identity := range participants {

		index, err := from.Index(identity.NodeID)
		if err != nil {
			return nil, err
		}
		key, err := from.KeyShare(identity.NodeID)
		if err != nil {
			return nil, err
		}

		dkg.Participants[identity.NodeID] = flow.DKGParticipant{
			Index:    index,
			KeyShare: key,
		}
	}

	return &DKG{dkg}, nil
}
