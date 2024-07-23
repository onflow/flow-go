package epochs

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

type EpochLookupSuite struct {
	suite.Suite

	// mocks
	epochQuery *mocks.EpochQuery
	state      *mockprotocol.State
	snapshot   *mockprotocol.Snapshot
	params     *mockprotocol.Params

	// backend for mocked functions
	mu    sync.Mutex // protects access to phase and used to invoke funcs with a lock
	phase flow.EpochPhase

	// config for each epoch
	currentEpochCounter uint64
	prevEpoch           epochRange
	currEpoch           epochRange
	nextEpoch           epochRange

	lookup *EpochLookup
	cancel context.CancelFunc
}

func TestEpochLookup(t *testing.T) {
	suite.Run(t, new(EpochLookupSuite))
}

func (suite *EpochLookupSuite) SetupTest() {
	suite.currentEpochCounter = uint64(1)
	suite.phase = flow.EpochPhaseStaking

	suite.prevEpoch = epochRange{counter: suite.currentEpochCounter - 1, firstView: 100, finalView: 199}
	suite.currEpoch = epochRange{counter: suite.currentEpochCounter, firstView: 200, finalView: 299}
	suite.nextEpoch = epochRange{counter: suite.currentEpochCounter + 1, firstView: 300, finalView: 399}

	suite.state = new(mockprotocol.State)
	suite.snapshot = new(mockprotocol.Snapshot)
	suite.params = new(mockprotocol.Params)
	suite.epochQuery = mocks.NewEpochQuery(suite.T(), suite.currentEpochCounter)

	suite.snapshot.On("Epochs").Return(suite.epochQuery)
	suite.snapshot.On("EpochPhase").Return(
		func() flow.EpochPhase { return suite.Phase() },
		func() error { return nil })

	epochProtocolState := mockprotocol.NewEpochProtocolState(suite.T())

	suite.snapshot.On("EpochProtocolState").Return(epochProtocolState, nil)

	suite.state.On("Final").Return(suite.snapshot)
	suite.state.On("Params").Return(suite.params)
}

func (suite *EpochLookupSuite) TearDownTest() {
	if suite.cancel != nil {
		suite.cancel()
	}
}

// WithLock runs the given function while holding the suite lock. Must be used
// while updating fields used as backends for mocked functions.
func (suite *EpochLookupSuite) WithLock(f func()) {
	suite.mu.Lock()
	f()
	suite.mu.Unlock()
}

func (suite *EpochLookupSuite) Phase() flow.EpochPhase {
	suite.mu.Lock()
	defer suite.mu.Unlock()
	return suite.phase
}

// CommitEpochs adds the new epochs to the state.
func (suite *EpochLookupSuite) CommitEpochs(epochs ...epochRange) {
	for _, epoch := range epochs {
		mockEpoch := newMockEpoch(epoch.counter, epoch.firstView, epoch.finalView)
		suite.epochQuery.Add(mockEpoch)
		// if we add a next epoch (counter 1 greater than current), then set phase to committed
		if epoch.counter == suite.currentEpochCounter+1 {
			suite.WithLock(func() {
				suite.phase = flow.EpochPhaseCommitted
			})
		}
	}
}

// CreateAndStartEpochLookup instantiates and starts the lookup.
// Should be called only once per test, after initial epoch mocks are created.
// It spawns a goroutine to detect fatal errors from the committee's error channel.
func (suite *EpochLookupSuite) CreateAndStartEpochLookup() {
	lookup, err := NewEpochLookup(suite.state)
	suite.Require().NoError(err)
	ctx, cancel, errCh := irrecoverable.WithSignallerAndCancel(context.Background())
	lookup.Start(ctx)
	go unittest.FailOnIrrecoverableError(suite.T(), ctx.Done(), errCh)

	suite.lookup = lookup
	suite.cancel = cancel
}

// TestEpochForViewWithFallback_Curr tests constructing and subsequently querying
// EpochLookup with an initial state of a current epoch.
func (suite *EpochLookupSuite) TestEpochForViewWithFallback_Curr() {
	epochs := []epochRange{suite.currEpoch}
	suite.CommitEpochs(epochs...)
	suite.CreateAndStartEpochLookup()
	testEpochForViewWithFallback(suite.T(), suite.lookup, suite.state, epochs...)
}

// TestEpochForViewWithFallback_PrevCurr tests constructing and subsequently querying
// EpochLookup with an initial state of a previous and current epoch.
func (suite *EpochLookupSuite) TestEpochForViewWithFallback_PrevCurr() {
	epochs := []epochRange{suite.prevEpoch, suite.currEpoch}
	suite.CommitEpochs(epochs...)
	suite.CreateAndStartEpochLookup()
	testEpochForViewWithFallback(suite.T(), suite.lookup, suite.state, epochs...)
}

// TestEpochForViewWithFallback_CurrNext tests constructing and subsequently querying
// EpochLookup with an initial state of a current and next epoch.
func (suite *EpochLookupSuite) TestEpochForViewWithFallback_CurrNext() {
	epochs := []epochRange{suite.currEpoch, suite.nextEpoch}
	suite.CommitEpochs(epochs...)
	suite.CreateAndStartEpochLookup()
	testEpochForViewWithFallback(suite.T(), suite.lookup, suite.state, epochs...)
}

// TestEpochForViewWithFallback_CurrNextPrev tests constructing and subsequently querying
// EpochLookup with an initial state of a previous, current, and next epoch.
func (suite *EpochLookupSuite) TestEpochForViewWithFallback_CurrNextPrev() {
	epochs := []epochRange{suite.prevEpoch, suite.currEpoch, suite.nextEpoch}
	suite.CommitEpochs(epochs...)
	suite.CreateAndStartEpochLookup()
	testEpochForViewWithFallback(suite.T(), suite.lookup, suite.state, epochs...)
}

// TestEpochForViewWithFallback_EpochFallbackTriggered tests constructing and subsequently querying
// EpochLookup with an initial state of epoch fallback triggered.
func (suite *EpochLookupSuite) TestEpochForViewWithFallback_EpochFallbackTriggered() {
	epochs := []epochRange{suite.prevEpoch, suite.currEpoch, suite.nextEpoch}
	suite.CommitEpochs(epochs...)
	suite.CreateAndStartEpochLookup()
	testEpochForViewWithFallback(suite.T(), suite.lookup, suite.state, epochs...)
}

// TestProtocolEvents_EpochExtended tests constructing and subsequently querying
// EpochLookup, where we process an EpochExtended event and expect the latest
// epoch final view to be updated with the updated final view of the current epoch
// in the protocol state.
func (suite *EpochLookupSuite) TestProtocolEvents_EpochExtended() {
	// previous and current epochs will be committed
	suite.CommitEpochs(suite.prevEpoch)
	epoch, extension := suite.epochFixtureWithExtension(suite.currEpoch.counter, suite.currEpoch.firstView, suite.currEpoch.finalView+1000)
	suite.epochQuery.Add(epoch)

	epochs := []epochRange{
		suite.prevEpoch,
		{counter: suite.currEpoch.counter, firstView: suite.currEpoch.firstView, finalView: extension.FinalView},
	}

	header := unittest.BlockHeaderFixture()
	suite.state.On("AtHeight", header.Height).Return(suite.snapshot).Times(4)

	suite.CreateAndStartEpochLookup()

	suite.lookup.EpochExtended(suite.currEpoch.counter, header, extension)

	// wait for the protocol event to be processed (async)
	assert.Eventually(suite.T(), func() bool {
		_, err := suite.lookup.EpochForViewWithFallback(extension.FinalView)
		return err == nil
	}, 5*time.Second, 50*time.Millisecond)

	// validate queries are answered correctly
	testEpochForViewWithFallback(suite.T(), suite.lookup, suite.state, epochs...)

	// should handle multiple deliveries of the protocol event
	suite.lookup.EpochExtended(suite.currEpoch.counter, header, extension)
	suite.lookup.EpochExtended(suite.currEpoch.counter, header, extension)
	suite.lookup.EpochExtended(suite.currEpoch.counter, header, extension)

	// validate queries are answered correctly
	testEpochForViewWithFallback(suite.T(), suite.lookup, suite.state, epochs...)
}

// TestProtocolEvents_CommittedEpoch tests correct processing of an `EpochCommittedPhaseStarted` event
func (suite *EpochLookupSuite) TestProtocolEvents_CommittedEpoch() {
	// initially, only current epoch is committed
	suite.CommitEpochs(suite.currEpoch)
	suite.CreateAndStartEpochLookup()

	// commit the next epoch, and emit a protocol event
	firstBlockOfCommittedPhase := unittest.BlockHeaderFixture()
	suite.state.On("AtBlockID", firstBlockOfCommittedPhase.ID()).Return(suite.snapshot)
	suite.CommitEpochs(suite.nextEpoch)
	suite.lookup.EpochCommittedPhaseStarted(suite.currentEpochCounter, firstBlockOfCommittedPhase)

	// wait for the protocol event to be processed (async)
	assert.Eventually(suite.T(), func() bool {
		_, err := suite.lookup.EpochForViewWithFallback(suite.currEpoch.finalView + 1)
		return err == nil
	}, 5*time.Second, 50*time.Millisecond)

	// validate queries are answered correctly
	testEpochForViewWithFallback(suite.T(), suite.lookup, suite.state, suite.currEpoch, suite.nextEpoch)

	// should handle multiple deliveries of the protocol event
	suite.lookup.EpochCommittedPhaseStarted(suite.currentEpochCounter, firstBlockOfCommittedPhase)
	suite.lookup.EpochCommittedPhaseStarted(suite.currentEpochCounter, firstBlockOfCommittedPhase)
	suite.lookup.EpochCommittedPhaseStarted(suite.currentEpochCounter, firstBlockOfCommittedPhase)

	// validate queries are answered correctly
	testEpochForViewWithFallback(suite.T(), suite.lookup, suite.state, suite.currEpoch, suite.nextEpoch)
}

// epochFixtureWithExtension creates a setup epoch with an extension.
func (suite *EpochLookupSuite) epochFixtureWithExtension(counter, firstView, extensionFinalView uint64) (protocol.Epoch, flow.EpochExtension) {
	setupFixture := unittest.EpochSetupFixture()
	setupFixture.Counter = counter
	setupFixture.FirstView = firstView

	extension := flow.EpochExtension{
		FirstView: setupFixture.FirstView,
		FinalView: extensionFinalView,
	}

	epoch := inmem.NewSetupEpoch(setupFixture, []flow.EpochExtension{extension})
	return epoch, extension
}

// testEpochForViewWithFallback accepts a constructed EpochLookup and state, and
// validates correctness by issuing various queries, using the input state and
// epochs as source of truth.
func testEpochForViewWithFallback(t *testing.T, lookup *EpochLookup, state protocol.State, epochs ...epochRange) {
	t.Run("should be able to query within any committed epoch", func(t *testing.T) {
		for _, epoch := range epochs {
			t.Run("first view", func(t *testing.T) {
				counter, err := lookup.EpochForViewWithFallback(epoch.firstView)
				assert.NoError(t, err)
				assert.Equal(t, epoch.counter, counter)
			})
			t.Run("final view", func(t *testing.T) {
				counter, err := lookup.EpochForViewWithFallback(epoch.finalView)
				assert.NoError(t, err)
				assert.Equal(t, epoch.counter, counter)
			})
			t.Run("random view in range", func(t *testing.T) {
				counter, err := lookup.EpochForViewWithFallback(unittest.Uint64InRange(epoch.firstView, epoch.finalView))
				assert.NoError(t, err)
				assert.Equal(t, epoch.counter, counter)
			})
		}
	})

	t.Run("should return ErrViewForUnknownEpoch below earliest epoch", func(t *testing.T) {
		t.Run("view 0", func(t *testing.T) {
			_, err := lookup.EpochForViewWithFallback(0)
			assert.ErrorIs(t, err, model.ErrViewForUnknownEpoch)
		})
		t.Run("boundary of earliest epoch", func(t *testing.T) {
			_, err := lookup.EpochForViewWithFallback(epochs[0].firstView - 1)
			assert.ErrorIs(t, err, model.ErrViewForUnknownEpoch)
		})
		t.Run("random view below earliest epoch", func(t *testing.T) {
			_, err := lookup.EpochForViewWithFallback(unittest.Uint64InRange(0, epochs[0].firstView-1))
			assert.ErrorIs(t, err, model.ErrViewForUnknownEpoch)
		})
	})

	t.Run("should return ErrViewForUnknownEpoch for queries above latest epoch final view", func(t *testing.T) {
		_, err := lookup.EpochForViewWithFallback(epochs[len(epochs)-1].finalView + 1)
		assert.ErrorIs(t, err, model.ErrViewForUnknownEpoch)
	})
}

// newMockEpoch returns a mock epoch with the given fields set.
func newMockEpoch(counter, firstView, finalView uint64) *mockprotocol.Epoch {
	epoch := new(mockprotocol.Epoch)
	epoch.On("FirstView").Return(firstView, nil)
	epoch.On("FinalView").Return(finalView, nil)
	epoch.On("Counter").Return(counter, nil)
	return epoch
}
