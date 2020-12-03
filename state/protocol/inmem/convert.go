package inmem

import (
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
		snap encodable.Snapshot
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
	if err != nil {
		return nil, err
	}
	snap.Epochs.Previous = previous.Epoch
	current, err := FromEpoch(from.Epochs().Current())
	if err != nil {
		return nil, err
	}
	snap.Epochs.Current = current.Epoch
	next, err := FromEpoch(from.Epochs().Next())
	if err != nil {
		return nil, err
	}
	snap.Epochs.Next = next.Epoch

	return &Snapshot{snap}, nil
}

// FromEpoch converts any protocol.Protocol to a memory-backed Epoch.
func FromEpoch(from protocol.Epoch) (*Epoch, error) {

	var (
		epoch encodable.Epoch
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
	if err != nil {
		return nil, err
	}
	convertedDKG, err := FromDKG(dkg, epoch.InitialIdentities.Filter(filter.HasRole(flow.RoleConsensus)))
	if err != nil {
		return nil, err
	}
	epoch.DKG = convertedDKG.DKG

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
		epoch.Clusters = append(epoch.Clusters, convertedCluster.Cluster)
	}

	return &Epoch{epoch}, nil
}

// FromCluster converts any protocol.Cluster to a memory-backed Cluster
func FromCluster(from protocol.Cluster) (*Cluster, error) {
	cluster := encodable.Cluster{
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

	var dkg encodable.DKG
	dkg.Size = from.Size()
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
