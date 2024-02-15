package integration

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// pacemaker timeout
// if your laptop is fast enough, 10 ms is enough
const pmTimeout = 100 * time.Millisecond

// maxTimeoutRebroadcast specifies how often the PaceMaker rebroadcasts
// its timeout object in case there is no progress. We keep the value
// small so we have smaller latency
const maxTimeoutRebroadcast = 1 * time.Second

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
	timeouts, err := timeout.NewConfig(pmTimeout, pmTimeout, 1.5, happyPathMaxRoundFailures, maxTimeoutRebroadcast)
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
	unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second, "expect to finish before timeout")

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
	replicasDroppingHappyPathMsgs := 2
	finalView := uint64(30)

	// generate the 4 hotstuff participants
	participants := unittest.IdentityListFixture(healthyReplicas + replicasDroppingHappyPathMsgs)
	instances := make([]*Instance, 0, healthyReplicas+replicasDroppingHappyPathMsgs)
	root := DefaultRoot()
	timeouts, err := timeout.NewConfig(10*time.Millisecond, 50*time.Millisecond, 1.5, happyPathMaxRoundFailures, maxTimeoutRebroadcast)
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

	// set up instances which can't vote, nor propose
	for n := healthyReplicas; n < healthyReplicas+replicasDroppingHappyPathMsgs; n++ {
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
	unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second, "expect to finish before timeout")

	// check that all instances have the same finalized block
	ref := instances[0]
	finalizedViews := FinalizedViews(ref)
	assert.Equal(t, []uint64{0}, finalizedViews, "no view was finalized, because finalization requires 2 direct chain plus a QC which never happen in this case")
	assert.Equal(t, finalView, ref.pacemaker.CurView(), "expect instance 0 should made enough progress, but didn't")
	for i := 1; i < healthyReplicas; i++ {
		assert.Equal(t, ref.forks.FinalizedBlock(), instances[i].forks.FinalizedBlock(), "instance %d should have same finalized block as first instance", i)
		assert.Equal(t, finalizedViews, FinalizedViews(instances[i]), "instance %d should have same finalized view as first instance", i)
		assert.Equal(t, finalView, instances[i].pacemaker.CurView(), "instance %d should have same active view as first instance", i)
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
	timeouts, err := timeout.NewConfig(pmTimeout, pmTimeout, 1.5, happyPathMaxRoundFailures, maxTimeoutRebroadcast)
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
	success := unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second, "expect to finish before timeout")
	if !success {
		t.Logf("dumping state of system:")
		for i, inst := range instances {
			t.Logf(
				"instance %d: %d %d %d",
				i,
				inst.pacemaker.CurView(),
				inst.pacemaker.NewestQC().View,
				inst.forks.FinalizedBlock().View,
			)
		}
	}

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
	timeouts, err := timeout.NewConfig(pmTimeout, pmTimeout, 1.5, happyPathMaxRoundFailures, maxTimeoutRebroadcast)
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
	unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second, "expect to finish before timeout")

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

// TestAsyncClusterStartup tests a realistic scenario where nodes are started asynchronously:
//   - Replicas are started in sequential order
//   - Each replica skips voting for first block(emulating message omission).
//   - Each replica skips first Timeout Object [TO] (emulating message omission).
//   - At this point protocol loses liveness unless a timeout rebroadcast happens from super-majority of replicas.
//
// This test verifies that nodes still make progress, despite first TO messages being lost.
// Implementation:
//   - We have 4 replicas in total, each of them skips voting for first view to force a timeout
//   - Block TOs for whole committee until each replica has generated its first TO.
//   - After each replica has generated a timeout allow subsequent timeout rebroadcasts to make progress.
func TestAsyncClusterStartup(t *testing.T) {
	replicas := 4
	finalView := uint64(20)

	// generate the seven hotstuff participants
	participants := unittest.IdentityListFixture(replicas)
	instances := make([]*Instance, 0, replicas)
	root := DefaultRoot()
	// set block rate delay to be bigger than minimal timeout
	timeouts, err := timeout.NewConfig(pmTimeout, pmTimeout, 1.5, 6, maxTimeoutRebroadcast)
	require.NoError(t, err)

	// set up instances that work fully
	var lock sync.Mutex
	timeoutObjectGenerated := make(map[flow.Identifier]struct{}, 0)
	for n := 0; n < replicas; n++ {
		in := NewInstance(t,
			WithRoot(root),
			WithParticipants(participants),
			WithLocalID(participants[n].NodeID),
			WithTimeouts(timeouts),
			WithStopCondition(ViewFinalized(finalView)),
			WithOutgoingVotes(func(vote *model.Vote) bool {
				return vote.View == 1
			}),
			WithOutgoingTimeoutObjects(func(object *model.TimeoutObject) bool {
				lock.Lock()
				defer lock.Unlock()
				timeoutObjectGenerated[object.SignerID] = struct{}{}
				// start allowing timeouts when every node has generated one
				// when nodes will broadcast again, it will go through
				return len(timeoutObjectGenerated) != replicas
			}),
		)
		instances = append(instances, in)
	}

	// connect the communicators of the instances together
	Connect(t, instances)

	// start each node only after previous one has started
	var wg sync.WaitGroup
	for _, in := range instances {
		wg.Add(1)
		go func(in *Instance) {
			err := in.Run()
			require.True(t, errors.Is(err, errStopCondition))
			wg.Done()
		}(in)
	}
	unittest.AssertReturnsBefore(t, wg.Wait, 10*time.Second, "expect to finish before timeout")

	// check that all instances have the same finalized block
	ref := instances[0]
	assert.Equal(t, finalView, ref.forks.FinalizedBlock().View, "expect instance 0 should made enough progress, but didn't")
	finalizedViews := FinalizedViews(ref)
	for i := 1; i < replicas; i++ {
		assert.Equal(t, ref.forks.FinalizedBlock(), instances[i].forks.FinalizedBlock(), "instance %d should have same finalized block as first instance")
		assert.Equal(t, finalizedViews, FinalizedViews(instances[i]), "instance %d should have same finalized view as first instance")
	}
}
