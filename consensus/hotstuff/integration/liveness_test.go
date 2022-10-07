package integration

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/onflow/flow-go/utils/unittest"
)

// pacemaker timeout
// if your laptop is fast enough, 10 ms is enough
const pmTimeout = 60 * time.Millisecond

// If 2 nodes are down in a 7 nodes cluster, the rest of 5 nodes can
// still make progress and reach consensus
func Test2TimeoutOutof7Instances(t *testing.T) {

	healthyReplicas := 5
	notVotingReplicas := 2
	finalView := uint64(30)

	// generate the seven hotstuff participants
	participants := unittest.IdentityListFixture(healthyReplicas + notVotingReplicas)
	instances := make([]*Instance, 0, healthyReplicas+notVotingReplicas)
	root := DefaultRoot()
	timeouts, err := timeout.NewConfig(pmTimeout, pmTimeout, 1.5, happyPathMaxRoundFailures, 0)
	require.NoError(t, err)

	// set up five instances that work fully
	for n := 0; n < healthyReplicas; n++ {
		in := NewInstance(t,
			WithRoot(root),
			WithParticipants(participants),
			WithLocalID(participants[n].NodeID),
			WithTimeouts(timeouts),
			WithStopCondition(ViewFinalized(finalView)),
		)
		instances = append(instances, in)
	}

	// set up two instances which can't vote, nor propose
	for n := healthyReplicas; n < healthyReplicas+notVotingReplicas; n++ {
		in := NewInstance(t,
			WithRoot(root),
			WithParticipants(participants),
			WithLocalID(participants[n].NodeID),
			WithTimeouts(timeouts),
			WithStopCondition(ViewFinalized(finalView)),
			WithOutgoingVotes(BlockAllVotes),
			WithOutgoingProposals(BlockAllProposals),
		)
		instances = append(instances, in)
	}

	// connect the communicators of the instances together
	Connect(t, instances)

	// start all seven instances and wait for them to wrap up
	var wg sync.WaitGroup
	for _, in := range instances {
		wg.Add(1)
		go func(in *Instance) {
			err := in.Run()
			require.True(t, errors.Is(err, errStopCondition))
			wg.Done()
		}(in)
	}
	wg.Wait()

	// check that all instances have the same finalized block
	ref := instances[0]
	assert.Equal(t, finalView, ref.forks.FinalizedBlock().View, "expect instance 0 should made enough progress, but didn't")
	finalizedViews := FinalizedViews(ref)
	for i := 1; i < healthyReplicas; i++ {
		assert.Equal(t, ref.forks.FinalizedBlock(), instances[i].forks.FinalizedBlock(), "instance %d should have same finalized block as first instance")
		assert.Equal(t, finalizedViews, FinalizedViews(instances[i]), "instance %d should have same finalized view as first instance")
	}
}

// 2 nodes in a 4-node cluster are configured to be able only to send timeout messages (no voting or proposing).
// The other 2 unconstrained nodes should be able to make progress through the recovery path by creating TCs
// for every round, but no block will be finalized, because finalization requires direct 1-chain and QC.
func Test2TimeoutOutof4Instances(t *testing.T) {

	healthyReplicas := 2
	replicasDroppingTimeouts := 2
	finalView := uint64(30)

	// generate the 4 hotstuff participants
	participants := unittest.IdentityListFixture(healthyReplicas + replicasDroppingTimeouts)
	instances := make([]*Instance, 0, healthyReplicas+replicasDroppingTimeouts)
	root := DefaultRoot()
	timeouts, err := timeout.NewConfig(
		pmTimeout, pmTimeout, 1.5, happyPathMaxRoundFailures, 0)
	require.NoError(t, err)

	// set up two instances that work fully
	for n := 0; n < healthyReplicas; n++ {
		in := NewInstance(t,
			WithRoot(root),
			WithParticipants(participants),
			WithLocalID(participants[n].NodeID),
			WithTimeouts(timeouts),
			WithStopCondition(ViewReached(finalView)),
		)
		instances = append(instances, in)
	}

	// set up one instance which can't vote, nor propose
	for n := healthyReplicas; n < healthyReplicas+replicasDroppingTimeouts; n++ {
		in := NewInstance(t,
			WithRoot(root),
			WithParticipants(participants),
			WithLocalID(participants[n].NodeID),
			WithTimeouts(timeouts),
			WithStopCondition(ViewReached(finalView)),
			WithOutgoingVotes(BlockAllVotes),
			WithIncomingVotes(BlockAllVotes),
			WithOutgoingProposals(BlockAllProposals),
		)
		instances = append(instances, in)
	}

	// connect the communicators of the instances together
	Connect(t, instances)

	// start the instances and wait for them to finish
	var wg sync.WaitGroup
	for _, in := range instances {
		wg.Add(1)
		go func(in *Instance) {
			err := in.Run()
			require.True(t, errors.Is(err, errStopCondition), "should run until stop condition")
			wg.Done()
		}(in)
	}
	wg.Wait()

	// check that all instances have the same finalized block
	ref := instances[0]
	finalizedViews := FinalizedViews(ref)
	assert.Equal(t, []uint64{0}, finalizedViews, "no view was finalized, because finalization requires 2 direct chain plus a QC which never happen in this case")
	assert.LessOrEqual(t, finalView, ref.pacemaker.CurView(), "expect instance 0 should made enough progress, but didn't")
	for i := 1; i < healthyReplicas; i++ {
		assert.Equal(t, ref.forks.FinalizedBlock(), instances[i].forks.FinalizedBlock(), "instance %d should have same finalized block as first instance")
		assert.Equal(t, finalizedViews, FinalizedViews(instances[i]), "instance %d should have same finalized view as first instance")
	}
}

// If 1 node is down in a 5 nodes cluster, the rest of 4 nodes can
// make progress and reach consensus
func Test1TimeoutOutof5Instances(t *testing.T) {

	healthyReplicas := 4
	blockedReplicas := 1
	finalView := uint64(30)

	// generate the seven hotstuff participants
	participants := unittest.IdentityListFixture(healthyReplicas + blockedReplicas)
	instances := make([]*Instance, 0, healthyReplicas+blockedReplicas)
	root := DefaultRoot()
	timeouts, err := timeout.NewConfig(pmTimeout, pmTimeout, 1.5, happyPathMaxRoundFailures, 0)
	require.NoError(t, err)

	// set up instances that work fully
	for n := 0; n < healthyReplicas; n++ {
		in := NewInstance(t,
			WithRoot(root),
			WithParticipants(participants),
			WithLocalID(participants[n].NodeID),
			WithTimeouts(timeouts),
			WithStopCondition(ViewFinalized(finalView)),
		)
		instances = append(instances, in)
	}

	// set up one instance which can't vote, nor propose
	for n := healthyReplicas; n < healthyReplicas+blockedReplicas; n++ {
		in := NewInstance(t,
			WithRoot(root),
			WithParticipants(participants),
			WithLocalID(participants[n].NodeID),
			WithTimeouts(timeouts),
			WithStopCondition(ViewReached(finalView)),
			WithOutgoingVotes(BlockAllVotes),
			WithOutgoingProposals(BlockAllProposals),
			WithIncomingProposals(BlockAllProposals),
		)
		instances = append(instances, in)
	}

	// connect the communicators of the instances together
	Connect(t, instances)

	// start all seven instances and wait for them to wrap up
	var wg sync.WaitGroup
	for _, in := range instances {
		wg.Add(1)
		go func(in *Instance) {
			err := in.Run()
			require.True(t, errors.Is(err, errStopCondition))
			wg.Done()
		}(in)
	}
	wg.Wait()

	// check that all instances have the same finalized block
	ref := instances[0]
	finalizedViews := FinalizedViews(ref)
	assert.Equal(t, finalView, ref.forks.FinalizedBlock().View, "expect instance 0 should made enough progress, but didn't")
	for i := 1; i < healthyReplicas; i++ {
		assert.Equal(t, ref.forks.FinalizedBlock(), instances[i].forks.FinalizedBlock(), "instance %d should have same finalized block as first instance")
		assert.Equal(t, finalizedViews, FinalizedViews(instances[i]), "instance %d should have same finalized view as first instance")
	}
}

// TestBlockDelayIsHigherThanTimeout tests an edge case protocol edge case, where
//   - The block arrives in time for replicas to vote.
//   - The next primary does not respond in time with a follow-up proposal,
//     so nodes start sending TimeoutObjects.
//   - However, eventually, the next primary successfully constructs a QC and a new
//     block before a TC leads to the round timing out.
//
// This test verifies that nodes still make progress on the happy path (QC constructed),
// despite already having initiated the timeout.
// Example scenarios, how this timing edge case could manifest:
//   - block delay is very close (or larger) than round duration
//   - delayed message transmission (specifically votes) within network
//   - overwhelmed / slowed-down primary
//   - byzantine primary
//
// Implementation:
//   - We have 4 nodes in total where the TimeoutObjects from two of them are always
//     discarded. Therefore, no TC can be constructed.
//   - To force nodes to initiate the timeout (i.e. send TimeoutObjects), we set
//     the `blockRateDelay` to _twice_ the PaceMaker Timeout. Furthermore, we configure
//     the PaceMaker to only increase timeout duration after 6 successive round failures.
func TestBlockDelayIsHigherThanTimeout(t *testing.T) {
	healthyReplicas := 2
	replicasNotGeneratingTimeouts := 2
	finalView := uint64(20)

	// generate the 4 hotstuff participants
	participants := unittest.IdentityListFixture(healthyReplicas + replicasNotGeneratingTimeouts)
	instances := make([]*Instance, 0, healthyReplicas+replicasNotGeneratingTimeouts)
	root := DefaultRoot()
	// set block rate delay to be bigger than minimal timeout
	timeouts, err := timeout.NewConfig(pmTimeout, pmTimeout, 1.5, happyPathMaxRoundFailures, pmTimeout*2)
	require.NoError(t, err)

	// set up 2 instances that fully work (incl. sending TimeoutObjects)
	for n := 0; n < healthyReplicas; n++ {
		in := NewInstance(t,
			WithRoot(root),
			WithParticipants(participants),
			WithLocalID(participants[n].NodeID),
			WithTimeouts(timeouts),
			WithStopCondition(ViewFinalized(finalView)),
		)
		instances = append(instances, in)
	}

	// set up two instances which don't generate and receive timeout objects
	for n := healthyReplicas; n < healthyReplicas+replicasNotGeneratingTimeouts; n++ {
		in := NewInstance(t,
			WithRoot(root),
			WithParticipants(participants),
			WithLocalID(participants[n].NodeID),
			WithTimeouts(timeouts),
			WithStopCondition(ViewFinalized(finalView)),
			WithIncomingTimeoutObjects(BlockAllTimeoutObjects),
			WithOutgoingTimeoutObjects(BlockAllTimeoutObjects),
		)
		instances = append(instances, in)
	}

	// connect the communicators of the instances together
	Connect(t, instances)

	// start all 4 instances and wait for them to wrap up
	var wg sync.WaitGroup
	for _, in := range instances {
		wg.Add(1)
		go func(in *Instance) {
			err := in.Run()
			require.True(t, errors.Is(err, errStopCondition))
			wg.Done()
		}(in)
	}
	wg.Wait()

	// check that all instances have the same finalized block
	ref := instances[0]
	assert.Equal(t, finalView, ref.forks.FinalizedBlock().View, "expect instance 0 should made enough progress, but didn't")
	finalizedViews := FinalizedViews(ref)
	// in this test we rely on QC being produced in each view
	// make sure that all views are strictly in increasing order with no gaps
	for i := 1; i < len(finalizedViews); i++ {
		// finalized views are sorted in descending order
		if finalizedViews[i-1] != finalizedViews[i]+1 {
			t.Fatalf("finalized views series has gap, this is not expected: %v", finalizedViews)
			return
		}
	}
	for i := 1; i < healthyReplicas; i++ {
		assert.Equal(t, ref.forks.FinalizedBlock(), instances[i].forks.FinalizedBlock(), "instance %d should have same finalized block as first instance")
		assert.Equal(t, finalizedViews, FinalizedViews(instances[i]), "instance %d should have same finalized view as first instance")
	}
}
