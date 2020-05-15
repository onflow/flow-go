package integration

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

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

func TestThreeInstances(t *testing.T) {

	// test parameters
	// NOTE: block finalization seems to be rather slow on CI at the moment,
	// needing around 1 minute on Travis for 1000 blocks and 10 minutes on
	// TeamCity for 1000 blocks; in order to avoid test timeouts, we keep the
	// number low here
	num := 3
	finalView := uint64(100)

	// generate three hotstuff participants
	participants := unittest.IdentityListFixture(num)
	root := DefaultRoot()

	// all of the nodes are up and should start around the same time, so we
	// can use a normal timeout with no need to go up or down
	timeouts, err := timeout.NewConfig(time.Second, time.Second, 0.5, 1.001, 1)
	require.NoError(t, err)

	// set up three instances that are exactly the same
	instances := make([]*Instance, 0, num)
	for n := 0; n < num; n++ {
		in := NewInstance(t,
			WithRoot(root),
			WithParticipants(participants),
			WithLocalID(participants[n].NodeID),
			WithTimeouts(timeouts),
			WithStopCondition(ViewFinalized(finalView)),
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
	in1 := instances[0]
	in2 := instances[1]
	in3 := instances[2]
	// verify progress has been made
	assert.GreaterOrEqual(t, in1.forks.FinalizedBlock().View, finalView, "the first instance 's finalized view should be four lower than current view")
	// verify same progresses have been made
	assert.Equal(t, in1.forks.FinalizedBlock(), in2.forks.FinalizedBlock(), "second instance should have same finalized block as first instance")
	assert.Equal(t, in1.forks.FinalizedBlock(), in3.forks.FinalizedBlock(), "third instance should have same finalized block as first instance")
	assert.Equal(t, FinalizedViews(in1), FinalizedViews(in2))
	assert.Equal(t, FinalizedViews(in1), FinalizedViews(in3))
}

func TestSevenInstances(t *testing.T) {

	t.Skip("this test is too heavy for CI")

	// test parameters
	// NOTE: block finalization seems to be rather slow on CI at the moment,
	// needing around 1 minute on Travis for 1000 blocks and 10 minutes on
	// TeamCity for 1000 blocks; in order to avoid test timeouts, we keep the
	// number low here
	numPass := 5
	numFail := 2
	finalView := uint64(100)

	// generate the seven hotstuff participants
	participants := unittest.IdentityListFixture(numPass + numFail)
	instances := make([]*Instance, 0, numPass+numFail)
	root := DefaultRoot()

	// we have two nodes that are offline; we should thus allow aggressive
	// reduction of timeout to avoid having them slow it down too much
	timeouts, err := timeout.NewConfig(time.Second, time.Second/2, 0.5, 1.5, 5*time.Second)
	require.NoError(t, err)

	// set up five instances that work fully
	for n := 0; n < numPass; n++ {
		in := NewInstance(t,
			WithRoot(root),
			WithParticipants(participants),
			WithLocalID(participants[n].NodeID),
			WithTimeouts(timeouts),
			WithStopCondition(ViewFinalized(finalView)),
		)
		instances = append(instances, in)
	}

	// set up two instances which can't vote
	for n := numPass; n < numPass+numFail; n++ {
		in := NewInstance(t,
			WithRoot(root),
			WithParticipants(participants),
			WithLocalID(participants[n].NodeID),
			WithTimeouts(timeouts),
			WithStopCondition(ViewFinalized(finalView)),
			WithOutgoingVotes(BlockAllVotes),
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
			require.True(t, errors.Is(err, errStopCondition), "should run until stop condition")
			wg.Done()
		}(in)
	}
	wg.Wait()

	// check that all instances have the same finalized block
	ref := instances[0]
	assert.Less(t, finalView-uint64(2*numPass+numFail), ref.forks.FinalizedBlock().View, "expect instance 0 should made enough progress, but didn't")
	finalizedViews := FinalizedViews(ref)
	for i := 1; i < numPass; i++ {
		assert.Equal(t, ref.forks.FinalizedBlock(), instances[i].forks.FinalizedBlock(), "instance %d should have same finalized block as first instance")
		assert.Equal(t, finalizedViews, FinalizedViews(instances[i]), "instance %d should have same finalized view as first instance")
	}
}
