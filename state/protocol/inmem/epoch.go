package inmem

import (
	"fmt"

	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/invalid"
)

// Epoch is a memory-backed implementation of protocol.Epoch.
type Epoch struct {
	enc EncodableEpoch
}

func (e Epoch) Encodable() EncodableEpoch {
	return e.enc
}

func (e Epoch) Counter() (uint64, error)            { return e.enc.Counter, nil }
func (e Epoch) FirstView() (uint64, error)          { return e.enc.FirstView, nil }
func (e Epoch) DKGPhase1FinalView() (uint64, error) { return e.enc.DKGPhase1FinalView, nil }
func (e Epoch) DKGPhase2FinalView() (uint64, error) { return e.enc.DKGPhase2FinalView, nil }
func (e Epoch) DKGPhase3FinalView() (uint64, error) { return e.enc.DKGPhase3FinalView, nil }
func (e Epoch) FinalView() (uint64, error)          { return e.enc.FinalView, nil }
func (e Epoch) InitialIdentities() (flow.IdentityList, error) {
	return e.enc.InitialIdentities, nil
}
func (e Epoch) RandomSource() ([]byte, error) {
	return e.enc.RandomSource, nil
}

func (e Epoch) Clustering() (flow.ClusterList, error) {
	return e.enc.Clustering, nil
}

func (e Epoch) DKG() (protocol.DKG, error) {
	if e.enc.DKG != nil {
		return DKG{*e.enc.DKG}, nil
	}
	return nil, protocol.ErrEpochNotCommitted
}

func (e Epoch) Cluster(i uint) (protocol.Cluster, error) {
	if e.enc.Clusters != nil {
		if i >= uint(len(e.enc.Clusters)) {
			return nil, fmt.Errorf("no cluster with index %d", i)
		}
		return Cluster{e.enc.Clusters[i]}, nil
	}
	return nil, protocol.ErrEpochNotCommitted
}

type Epochs struct {
	enc EncodableEpochs
}

func (eq Epochs) Previous() protocol.Epoch {
	if eq.enc.Previous != nil {
		return Epoch{*eq.enc.Previous}
	}
	return invalid.NewEpoch(protocol.ErrNoPreviousEpoch)
}
func (eq Epochs) Current() protocol.Epoch {
	return Epoch{eq.enc.Current}
}
func (eq Epochs) Next() protocol.Epoch {
	if eq.enc.Next != nil {
		return Epoch{*eq.enc.Next}
	}
	return invalid.NewEpoch(protocol.ErrNextEpochNotSetup)
}

// setupEpoch is an implementation of protocol.Epoch backed by an EpochSetup
// service event. This is used for converting service events to inmem.Epoch.
type setupEpoch struct {
	// EpochSetup service event
	setupEvent *flow.EpochSetup
}

func (es *setupEpoch) Counter() (uint64, error) {
	return es.setupEvent.Counter, nil
}

func (es *setupEpoch) FirstView() (uint64, error) {
	return es.setupEvent.FirstView, nil
}

func (es *setupEpoch) DKGPhase1FinalView() (uint64, error) {
	return es.setupEvent.DKGPhase1FinalView, nil
}

func (es *setupEpoch) DKGPhase2FinalView() (uint64, error) {
	return es.setupEvent.DKGPhase2FinalView, nil
}

func (es *setupEpoch) DKGPhase3FinalView() (uint64, error) {
	return es.setupEvent.DKGPhase3FinalView, nil
}

func (es *setupEpoch) FinalView() (uint64, error) {
	return es.setupEvent.FinalView, nil
}

func (es *setupEpoch) InitialIdentities() (flow.IdentityList, error) {
	identities := es.setupEvent.Participants.Filter(filter.Any)
	return identities, nil
}

func (es *setupEpoch) Clustering() (flow.ClusterList, error) {
	collectorFilter := filter.HasRole(flow.RoleCollection)
	clustering, err := flow.NewClusterList(es.setupEvent.Assignments, es.setupEvent.Participants.Filter(collectorFilter))
	if err != nil {
		return nil, fmt.Errorf("failed to generate ClusterList from collector identities: %w", err)
	}
	return clustering, nil
}

func (es *setupEpoch) Cluster(_ uint) (protocol.Cluster, error) {
	return nil, protocol.ErrEpochNotCommitted
}

func (es *setupEpoch) DKG() (protocol.DKG, error) {
	return nil, protocol.ErrEpochNotCommitted
}

func (es *setupEpoch) RandomSource() ([]byte, error) {
	return es.setupEvent.RandomSource, nil
}

// committedEpoch is an implementation of protocol.Epoch backed by an EpochSetup
// and EpochCommit service event. This is used for converting service events to
// inmem.Epoch.
type committedEpoch struct {
	setupEpoch
	commitEvent *flow.EpochCommit
}

func (es *committedEpoch) Cluster(index uint) (protocol.Cluster, error) {

	epochCounter := es.setupEvent.Counter

	clustering, err := es.Clustering()
	if err != nil {
		return nil, fmt.Errorf("failed to generate clustering: %w", err)
	}

	members, ok := clustering.ByIndex(index)
	if !ok {
		return nil, fmt.Errorf("failed to get members of cluster %d: %w", index, err)
	}

	qcs := es.commitEvent.ClusterQCs
	if uint(len(qcs)) <= index {
		return nil, fmt.Errorf("no cluster with index %d", index)
	}
	rootQCVoteData := qcs[index]

	rootBlock := cluster.CanonicalRootBlock(epochCounter, members)
	rootQC := &flow.QuorumCertificate{
		View:      rootBlock.Header.View,
		BlockID:   rootBlock.ID(),
		SignerIDs: rootQCVoteData.VoterIDs,
		SigData:   rootQCVoteData.SigData,
	}

	cluster, err := ClusterFromEncodable(EncodableCluster{
		Index:     index,
		Counter:   epochCounter,
		Members:   members,
		RootBlock: rootBlock,
		RootQC:    rootQC,
	})
	return cluster, err
}

func (es *committedEpoch) DKG() (protocol.DKG, error) {
	// filter initial participants to valid DKG participants
	participants := es.setupEvent.Participants.Filter(filter.IsValidDKGParticipant)
	lookup, err := flow.ToDKGParticipantLookup(participants, es.commitEvent.DKGParticipantKeys)
	if err != nil {
		return nil, fmt.Errorf("could not construct dkg lookup: %w", err)
	}

	dkg, err := DKGFromEncodable(EncodableDKG{
		GroupKey: encodable.RandomBeaconPubKey{
			PublicKey: es.commitEvent.DKGGroupKey,
		},
		Participants: lookup,
	})
	return dkg, err
}

// NewSetupEpoch returns a memory-backed epoch implementation based on an
// EpochSetup event. Epoch information available after the setup phase will
// not be accessible in the resulting epoch instance.
func NewSetupEpoch(setupEvent *flow.EpochSetup) (*Epoch, error) {
	convertible := &setupEpoch{
		setupEvent: setupEvent,
	}
	return FromEpoch(convertible)
}

// NewCommittedEpoch returns a memory-backed epoch implementation based on an
// EpochSetup and EpochCommit event.
func NewCommittedEpoch(setupEvent *flow.EpochSetup, commitEvent *flow.EpochCommit) (*Epoch, error) {
	convertible := &committedEpoch{
		setupEpoch: setupEpoch{
			setupEvent: setupEvent,
		},
		commitEvent: commitEvent,
	}
	return FromEpoch(convertible)
}
