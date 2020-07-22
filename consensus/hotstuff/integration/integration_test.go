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

// a pacemaker timeout to wait for proposals. Usually 10 ms is enough,
// but for slow environment like CI, a longer one is needed.
const safeTimeout = 2 * time.Second
const safeDecrease = 200 * time.Millisecond
const safeDecreaseFactor = 0.85
const livenessViewFactor = 0.9

func TestSingleInstance(t *testing.T) {

	// set up a single instance to run
	// NOTE: currently, the HotStuff logic will infinitely call back on itself
	// with a single instance, leading to a boundlessly growing call stack,
	// which slows down the mocks significantly due to splitting the callstack
	// to find the calling function name; we thus keep it low for now
	finalView := uint64(10)
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
	finalView := uint64(30)

	// generate three hotstuff participants
	participants := unittest.IdentityListFixture(num)
	root := DefaultRoot()
	timeouts, err := timeout.NewConfig(safeTimeout, safeTimeout, 0.5, 1.5, safeDecreaseFactor, 0)
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

	allViews := allFinalizedViews(t, instances)
	assertSafety(t, allViews)
	assertLiveness(t, allViews, uint64(float64(finalView)*livenessViewFactor))
}

func TestSevenInstances(t *testing.T) {

	// test parameters
	// NOTE: block finalization seems to be rather slow on CI at the moment,
	// needing around 1 minute on Travis for 1000 blocks and 10 minutes on
	// TeamCity for 1000 blocks; in order to avoid test timeouts, we keep the
	// number low here
	numPass := 5
	numFail := 2

	// When using 100 as finalView, I often saw this tests fail on CI, because it only made to around 64-86
	// so using 10 will still check that it's making progress and give enough buffer.
	finalView := uint64(30)

	// generate the seven hotstuff participants
	participants := unittest.IdentityListFixture(numPass + numFail)
	instances := make([]*Instance, 0, numPass+numFail)
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

	allViews := allFinalizedViews(t, instances)
	assertSafety(t, allViews)
	assertLiveness(t, allViews, uint64(float64(finalView)*livenessViewFactor))
}
