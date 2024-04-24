package inmem

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/factory"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/invalid"
)

// Epochs provides access to epoch data, backed by a rich protocol state entry.
type Epochs struct {
	entry *flow.RichProtocolStateEntry
}

var _ protocol.EpochQuery = (*Epochs)(nil)

func (eq Epochs) Previous() protocol.Epoch {
	if eq.entry.PreviousEpoch == nil {
		return invalid.NewEpoch(protocol.ErrNoPreviousEpoch)
	}
	return NewCommittedEpoch(eq.entry.PreviousEpochSetup, eq.entry.PreviousEpochCommit)
}

func (eq Epochs) Current() protocol.Epoch {
	return NewCommittedEpoch(eq.entry.CurrentEpochSetup, eq.entry.CurrentEpochCommit)
}

func (eq Epochs) Next() protocol.Epoch {
	switch eq.entry.EpochPhase() {
	case flow.EpochPhaseStaking:
		return invalid.NewEpoch(protocol.ErrNextEpochNotSetup)
	case flow.EpochPhaseSetup:
		return NewSetupEpoch(eq.entry.NextEpochSetup)
	case flow.EpochPhaseCommitted:
		return NewCommittedEpoch(eq.entry.NextEpochSetup, eq.entry.NextEpochCommit)
	}
	return invalid.NewEpochf("unexpected unknown phase in protocol state entry")
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

// TargetDuration returns the desired real-world duration for this epoch, in seconds.
// This target is specified by the FlowEpoch smart contract in the EpochSetup event
// and used by the Cruise Control system to moderate the block rate.
func (es *setupEpoch) TargetDuration() (uint64, error) {
	return es.setupEvent.TargetDuration, nil
}

// TargetEndTime returns the desired real-world end time for this epoch, represented as
// Unix Time (in units of seconds). This target is specified by the FlowEpoch smart contract in
// the EpochSetup event and used by the Cruise Control system to moderate the block rate.
func (es *setupEpoch) TargetEndTime() (uint64, error) {
	return es.setupEvent.TargetEndTime, nil
}

func (es *setupEpoch) RandomSource() ([]byte, error) {
	return es.setupEvent.RandomSource, nil
}

func (es *setupEpoch) InitialIdentities() (flow.IdentitySkeletonList, error) {
	return es.setupEvent.Participants, nil
}

func (es *setupEpoch) Clustering() (flow.ClusterList, error) {
	return ClusteringFromSetupEvent(es.setupEvent)
}

func ClusteringFromSetupEvent(setupEvent *flow.EpochSetup) (flow.ClusterList, error) {
	collectorFilter := filter.HasRole[flow.IdentitySkeleton](flow.RoleCollection)
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
	return 0, protocol.ErrEpochTransitionNotFinalized
}

func (es *setupEpoch) FinalHeight() (uint64, error) {
	return 0, protocol.ErrEpochTransitionNotFinalized
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
		return nil, fmt.Errorf("no cluster with index %d: %w", index, protocol.ErrClusterNotFound)
	}

	qcs := es.commitEvent.ClusterQCs
	if uint(len(qcs)) <= index {
		return nil, fmt.Errorf("internal data inconsistency: cannot get qc at index %d - epoch has %d clusters and %d cluster QCs",
			index, len(clustering), len(qcs))
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

func (es *committedEpoch) ClusterByChainID(chainID flow.ChainID) (protocol.Cluster, error) {
	clustering, err := es.Clustering()
	if err != nil {
		return nil, fmt.Errorf("failed to generate clustering: %w", err)
	}

	for i, cl := range clustering {
		if cluster.CanonicalClusterID(es.setupEvent.Counter, cl.NodeIDs()) == chainID {
			cl, err := es.Cluster(uint(i))
			if err != nil {
				return nil, fmt.Errorf("could not retrieve known existing cluster (idx=%d, id=%s): %v", i, chainID, err)
			}
			return cl, nil
		}
	}
	return nil, protocol.ErrClusterNotFound
}

func (es *committedEpoch) DKG() (protocol.DKG, error) {
	encodable, err := EncodableDKGFromEvents(es.setupEvent, es.commitEvent)
	if err != nil {
		return nil, fmt.Errorf("could not build encodable DKG from epoch events")
	}
	return DKGFromEncodable(encodable)
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
func NewSetupEpoch(setupEvent *flow.EpochSetup) protocol.Epoch {
	return &setupEpoch{
		setupEvent: setupEvent,
	}
}

// NewCommittedEpoch returns a memory-backed epoch implementation based on an
// EpochSetup and EpochCommit events.
// No errors are expected during normal operations.
func NewCommittedEpoch(setupEvent *flow.EpochSetup, commitEvent *flow.EpochCommit) protocol.Epoch {
	return &committedEpoch{
		setupEpoch: setupEpoch{
			setupEvent: setupEvent,
		},
		commitEvent: commitEvent,
	}
}

// NewStartedEpoch returns a memory-backed epoch implementation based on an
// EpochSetup and EpochCommit events, and the epoch's first block height.
// No errors are expected during normal operations.
func NewStartedEpoch(setupEvent *flow.EpochSetup, commitEvent *flow.EpochCommit, firstHeight uint64) protocol.Epoch {
	return &startedEpoch{
		committedEpoch: committedEpoch{
			setupEpoch: setupEpoch{
				setupEvent: setupEvent,
			},
			commitEvent: commitEvent,
		},
		firstHeight: firstHeight,
	}
}

// NewEndedEpoch returns a memory-backed epoch implementation based on an
// EpochSetup and EpochCommit events, and the epoch's final block height.
// No errors are expected during normal operations.
func NewEndedEpoch(setupEvent *flow.EpochSetup, commitEvent *flow.EpochCommit, firstHeight, finalHeight uint64) protocol.Epoch {
	return &endedEpoch{
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
}
