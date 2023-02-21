package inmem

import (
	"fmt"

	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/factory"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/signature"
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
	return nil, protocol.ErrNextEpochNotCommitted
}

func (e Epoch) Cluster(i uint) (protocol.Cluster, error) {
	if e.enc.Clusters == nil {
		return nil, protocol.ErrNextEpochNotCommitted
	}

	if i >= uint(len(e.enc.Clusters)) {
		return nil, fmt.Errorf("no cluster with index %d: %w", i, protocol.ErrClusterNotFound)
	}
	return Cluster{e.enc.Clusters[i]}, nil
}

func (e Epoch) ClusterByChainID(chainID flow.ChainID) (protocol.Cluster, error) {
	if e.enc.Clusters == nil {
		return nil, protocol.ErrNextEpochNotCommitted
	}

	for _, cluster := range e.enc.Clusters {
		if cluster.RootBlock.Header.ChainID == chainID {
			return Cluster{cluster}, nil
		}
	}
	chainIDs := make([]string, 0, len(e.enc.Clusters))
	for _, cluster := range e.enc.Clusters {
		chainIDs = append(chainIDs, string(cluster.RootBlock.Header.ChainID))
	}
	return nil, fmt.Errorf("no cluster with the given chain ID %v, available chainIDs %v: %w", chainID, chainIDs, protocol.ErrClusterNotFound)
}

func (e Epoch) FinalHeight() (uint64, error) {
	if e.enc.FinalHeight != nil {
		return *e.enc.FinalHeight, nil
	}
	return 0, protocol.ErrEpochNotEnded
}

func (e Epoch) FirstHeight() (uint64, error) {
	if e.enc.FirstHeight != nil {
		return *e.enc.FirstHeight, nil
	}
	return 0, protocol.ErrEpochNotStarted
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

func (es *setupEpoch) RandomSource() ([]byte, error) {
	return es.setupEvent.RandomSource, nil
}

func (es *setupEpoch) InitialIdentities() (flow.IdentityList, error) {
	identities := es.setupEvent.Participants.Filter(filter.Any)
	return identities, nil
}

func (es *setupEpoch) Clustering() (flow.ClusterList, error) {
	return ClusteringFromSetupEvent(es.setupEvent)
}

func ClusteringFromSetupEvent(setupEvent *flow.EpochSetup) (flow.ClusterList, error) {
	collectorFilter := filter.HasRole(flow.RoleCollection)
	clustering, err := factory.NewClusterList(setupEvent.Assignments, setupEvent.Participants.Filter(collectorFilter))
	if err != nil {
		return nil, fmt.Errorf("failed to generate ClusterList from collector identities: %w", err)
	}
	return clustering, nil
}

func (es *setupEpoch) Cluster(_ uint) (protocol.Cluster, error) {
	return nil, protocol.ErrNextEpochNotCommitted
}

func (es *setupEpoch) ClusterByChainID(_ flow.ChainID) (protocol.Cluster, error) {
	return nil, protocol.ErrNextEpochNotCommitted
}

func (es *setupEpoch) DKG() (protocol.DKG, error) {
	return nil, protocol.ErrNextEpochNotCommitted
}

func (es *setupEpoch) FirstHeight() (uint64, error) {
	return 0, protocol.ErrEpochNotStarted
}

func (es *setupEpoch) FinalHeight() (uint64, error) {
	return 0, protocol.ErrEpochNotEnded
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

	// TODO: double check ByIndex returns canonical order
	members, ok := clustering.ByIndex(index)
	if !ok {
		return nil, fmt.Errorf("failed to get members of cluster %d: %w", index, err)
	}

	qcs := es.commitEvent.ClusterQCs
	if uint(len(qcs)) <= index {
		return nil, fmt.Errorf("no cluster with index %d", index)
	}
	rootQCVoteData := qcs[index]

	signerIndices, err := signature.EncodeSignersToIndices(members.NodeIDs(), rootQCVoteData.VoterIDs)
	if err != nil {
		return nil, fmt.Errorf("could not encode signer indices for rootQCVoteData.VoterIDs: %w", err)
	}

	rootBlock := cluster.CanonicalRootBlock(epochCounter, members)
	rootQC := &flow.QuorumCertificate{
		View:          rootBlock.Header.View,
		BlockID:       rootBlock.ID(),
		SignerIndices: signerIndices,
		SigData:       rootQCVoteData.SigData,
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

// startedEpoch represents an epoch (with counter N) that has started, but there is no _finalized_ transition
// to the next epoch yet. Note that nodes can already be in views belonging to the _next_ Epoch, and it is
// possible that there are already unfinalized blocks in that next epoch. However, without finalized blocks
// in Epoch N+1, there is no definition of "last block" for Epoch N. 
// 
// startedEpoch has all the information of a committedEpoch, plus the epoch's first block height.
type startedEpoch struct {
	committedEpoch
	firstHeight uint64
}

func (e *startedEpoch) FirstHeight() (uint64, error) {
	return e.firstHeight, nil
}

// endedEpoch is an epoch which has ended (ie. the previous epoch). It has all the
// information of a startedEpoch, plus the epoch's final block height.
type endedEpoch struct {
	startedEpoch
	finalHeight uint64
}

func (e *endedEpoch) FinalHeight() (uint64, error) {
	return e.finalHeight, nil
}

// NewSetupEpoch returns a memory-backed epoch implementation based on an
// EpochSetup event. Epoch information available after the setup phase will
// not be accessible in the resulting epoch instance.
// No errors are expected during normal operations.
func NewSetupEpoch(setupEvent *flow.EpochSetup) (*Epoch, error) {
	convertible := &setupEpoch{
		setupEvent: setupEvent,
	}
	epoch, err := FromEpoch(convertible)
	// since we are passing in a concrete service event, no errors are expected
	if err != nil {
		return nil, fmt.Errorf("unexpected error constructing setup epoch from service event: %s", err.Error())
	}
	return epoch, nil
}

// NewCommittedEpoch returns a memory-backed epoch implementation based on an
// EpochSetup and EpochCommit events.
// No errors are expected during normal operations.
func NewCommittedEpoch(setupEvent *flow.EpochSetup, commitEvent *flow.EpochCommit) (*Epoch, error) {
	convertible := &committedEpoch{
		setupEpoch: setupEpoch{
			setupEvent: setupEvent,
		},
		commitEvent: commitEvent,
	}
	epoch, err := FromEpoch(convertible)
	// since we are passing in a concrete service event, no errors are expected
	if err != nil {
		return nil, fmt.Errorf("unexpected error constructing committed epoch from service events: %s", err.Error())
	}
	return epoch, nil
}

// NewStartedEpoch returns a memory-backed epoch implementation based on an
// EpochSetup and EpochCommit events, and the epoch's first block height.
// No errors are expected during normal operations.
func NewStartedEpoch(setupEvent *flow.EpochSetup, commitEvent *flow.EpochCommit, firstHeight uint64) (*Epoch, error) {
	convertible := &startedEpoch{
		committedEpoch: committedEpoch{
			setupEpoch: setupEpoch{
				setupEvent: setupEvent,
			},
			commitEvent: commitEvent,
		},
		firstHeight: firstHeight,
	}
	epoch, err := FromEpoch(convertible)
	// since we are passing in a concrete service event, no errors are expected
	if err != nil {
		return nil, fmt.Errorf("unexpected error constructing started epoch from service events: %s", err.Error())
	}
	return epoch, nil
}

// NewEndedEpoch returns a memory-backed epoch implementation based on an
// EpochSetup and EpochCommit events, and the epoch's final block height.
// No errors are expected during normal operations.
func NewEndedEpoch(setupEvent *flow.EpochSetup, commitEvent *flow.EpochCommit, firstHeight, finalHeight uint64) (*Epoch, error) {
	convertible := &endedEpoch{
		startedEpoch: startedEpoch{
			committedEpoch: committedEpoch{
				setupEpoch: setupEpoch{
					setupEvent: setupEvent,
				},
				commitEvent: commitEvent,
			},
			firstHeight: firstHeight,
		},
		finalHeight: finalHeight,
	}
	epoch, err := FromEpoch(convertible)
	// since we are passing in a concrete service event, no errors are expected
	if err != nil {
		return nil, fmt.Errorf("unexpected error constructing ended epoch from service events: %s", err.Error())
	}
	return epoch, nil
}
