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

	numPass := 5
	numFail := 2
	finalView := uint64(30)

	// generate the seven hotstuff participants
	participants := unittest.IdentityListFixture(numPass + numFail)
	instances := make([]*Instance, 0, numPass+numFail)
	root := DefaultRoot()
	timeouts, err := timeout.NewConfig(pmTimeout, pmTimeout, pmTimeout, 1.5, 0.85, 0)
	require.NoError(t, err)

	// set up five instances that work fully
	for n := 0; n < numPass; n++ {
		in := NewInstance(t,
			WithRoot(root),
			WithParticipants(participants),
			WithLocalID(participants[n].NodeID),
			WithTimeouts(timeouts),
			WithStopCondition(ViewReached(finalView)),
		)
		instances = append(instances, in)
	}

	// set up two instances which can't vote, nor propose
	for n := numPass; n < numPass+numFail; n++ {
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
	Connect(instances)

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
	assert.Less(t, finalView-uint64(2*numPass+numFail), ref.forks.FinalizedBlock().View, "expect instance 0 should made enough progress, but didn't")
	finalizedViews := FinalizedViews(ref)
	for i := 1; i < numPass; i++ {
		assert.Equal(t, ref.forks.FinalizedBlock(), instances[i].forks.FinalizedBlock(), "instance %d should have same finalized block as first instance", i)
		assert.Equal(t, finalizedViews, FinalizedViews(instances[i]), "instance %d should have same finalized view as first instance", i)
	}
}

// 2 nodes in a 4-node cluster are configured to be able only to send timeout messages (no voting or proposing).
// The other 2 unconstrained nodes should be able to make progress through the recovery path by creating TCs
// for every round, but no block will be finalized, because finalization requires direct 1-chain and QC.
func Test2TimeoutOutof4Instances(t *testing.T) {

	numPass := 2
	numFail := 2
	finalView := uint64(30)

	// generate the 4 hotstuff participants
	participants := unittest.IdentityListFixture(numPass + numFail)
	instances := make([]*Instance, 0, numPass+numFail)
	root := DefaultRoot()
	timeouts, err := timeout.NewConfig(pmTimeout, pmTimeout, pmTimeout, 1.5, 0.85, 0)
	require.NoError(t, err)

	// set up two instances that work fully
	for n := 0; n < numPass; n++ {
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
	for n := numPass; n < numPass+numFail; n++ {
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
	Connect(instances)

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
	for i := 1; i < numPass; i++ {
		assert.Equal(t, ref.forks.FinalizedBlock(), instances[i].forks.FinalizedBlock(), "instance %d should have same finalized block as first instance")
		assert.Equal(t, finalizedViews, FinalizedViews(instances[i]), "instance %d should have same finalized view as first instance")
	}
}

// If 1 node is down in a 5 nodes cluster, the rest of 4 nodes can
// make progress and reach consensus
func Test1TimeoutOutof5Instances(t *testing.T) {

	numPass := 4
	numFail := 1
	finalView := uint64(30)

	// generate the seven hotstuff participants
	participants := unittest.IdentityListFixture(numPass + numFail)
	instances := make([]*Instance, 0, numPass+numFail)
	root := DefaultRoot()
	timeouts, err := timeout.NewConfig(pmTimeout, pmTimeout, pmTimeout, 1.5, 0.85, 0)
	require.NoError(t, err)

	// set up instances that work fully
	for n := 0; n < numPass; n++ {
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
	for n := numPass; n < numPass+numFail; n++ {
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
	Connect(instances)

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
	assert.Less(t, finalView-uint64(2*numPass+numFail), ref.forks.FinalizedBlock().View, "expect instance 0 should made enough progress, but didn't")
	for i := 1; i < numPass; i++ {
		assert.Equal(t, ref.forks.FinalizedBlock(), instances[i].forks.FinalizedBlock(), "instance %d should have same finalized block as first instance")
		assert.Equal(t, finalizedViews, FinalizedViews(instances[i]), "instance %d should have same finalized view as first instance")
	}
}
