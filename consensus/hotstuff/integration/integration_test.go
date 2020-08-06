package integration

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// a pacemaker timeout to wait for proposals. Usually 10 ms is enough,
// but for slow environment like CI, a longer one is needed.
const safeTimeout = 2 * time.Second
const safeDecrease = 200 * time.Millisecond
const safeDecreaseFactor = 0.85

// waitTimeout waits for the waitgroup for the specified max timeout.
// Returns true if waiting timed out.
func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}

func TestSingleInstance(t *testing.T) {

	// set up a single instance to run
	// NOTE: currently, the HotStuff logic will infinitely call back on itself
	// with a single instance, leading to a boundlessly growing call stack,
	// which slows down the mocks significantly due to splitting the callstack
	// to find the calling function name; we thus keep it low for now
	finalView := uint64(100)
	in := NewInstance(t,
		WithStopCondition(ViewFinalized(finalView)),
	)

	// run the event handler until we reach a stop condition
	err := in.Run()
	require.True(t, errors.Is(err, errStopCondition), "should run until stop condition")

	// check if forks and pacemaker are in expected view state
	assert.Equal(t, finalView, in.forks.FinalizedView(), "finalized view should be three lower than current view")
}

// Run 3 instances to build blocks until there are a certain number of blocks are finalized
func TestInstancesThree(t *testing.T) {

	// test parameters
	num := 3
	finalizedCount := 10

	// generate three hotstuff participants
	participants := unittest.IdentityListFixture(num)
	root := DefaultRoot()
	timeouts, err := timeout.NewConfig(safeTimeout, safeTimeout, 0.5, 1.5, safeDecreaseFactor, 0)
	require.NoError(t, err)

	// set up three instances that are exactly the same
	instances := make([]*Instance, num)
	for n := 0; n < num; n++ {
		in := NewInstance(t,
			WithRoot(root),
			WithParticipants(participants),
			WithLocalID(participants[n].NodeID),
			WithTimeouts(timeouts),
			// when should we stop the instance?
			// should we stop it as soon as the finalized count reaches a certain number? no.
			// because if one node has finalized x blocks, other nodes might be behind, stopping this node
			// would cause other nodes unable to reach the targeted finalized count.
			// therefore, we should wait until all nodes have passed a certain finalized count.
			WithStopCondition(FinalizedCountsAllReached(instances, finalizedCount)),
		)
		instances[n] = in
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

	timeoutTriggered := waitTimeout(&wg, 30*time.Second)
	require.False(t, timeoutTriggered)

	allViews := allFinalizedViews(t, instances)
	assertSafety(t, allViews)
}

func TestInstancesSeven(t *testing.T) {

	// test parameters
	numPass := 5
	numFail := 2

	finalizedCount := 1

	// generate the seven hotstuff participants
	participants := unittest.IdentityListFixture(numPass + numFail)
	instances := make([]*Instance, numPass+numFail)
	root := DefaultRoot()
	timeouts, err := timeout.NewConfig(safeTimeout, safeTimeout, 0.5, 1.5, safeDecreaseFactor, 0)
	require.NoError(t, err)

	// set up five instances that work fully
	for n := 0; n < numPass; n++ {
		in := NewInstance(t,
			WithRoot(root),
			WithParticipants(participants),
			WithLocalID(participants[n].NodeID),
			WithTimeouts(timeouts),
			// stop when all honest nodes have finalized a certain number of blocks
			WithStopCondition(FinalizedCountsAllReached(instances[:numPass], finalizedCount)),
		)
		instances[n] = in
	}

	// set up two instances which can't vote
	for n := numPass; n < numPass+numFail; n++ {
		in := NewInstance(t,
			WithRoot(root),
			WithParticipants(participants),
			WithLocalID(participants[n].NodeID),
			WithTimeouts(timeouts),
			// stop when all honest nodes have finalized a certain number of blocks
			WithStopCondition(FinalizedCountsAllReached(instances[:numPass], finalizedCount)),
			WithOutgoingVotes(BlockAllVotes),
		)
		instances[n] = in
	}

	// connect the communicators of the instances together
	Connect(instances)

	// start all seven instances and wait for them to wrap up
	var wg sync.WaitGroup
	for _, in := range instances {
		wg.Add(1)
		go func(in *Instance) {
			err := in.Run()
			require.True(t, errors.Is(err, errStopCondition), fmt.Sprintf("should run until stop condition, but got error: %v", err))
			wg.Done()
		}(in)
	}
	timeoutTriggered := waitTimeout(&wg, 30*time.Second)
	require.False(t, timeoutTriggered)

	allViews := allFinalizedViews(t, instances)
	assertSafety(t, allViews)
}
