package epochs

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
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

	// protects access to phase and used to invoke funcs with a lock
	mu    sync.Mutex
	phase flow.EpochPhase

	// config for each epoch
	currentEpochCounter uint64
	prevEpoch           viewRange
	currEpoch           viewRange
	nextEpoch           viewRange

	lookup *EpochLookup
	cancel context.CancelFunc
}

func TestEpochLookup(t *testing.T) {
	suite.Run(t, new(EpochLookupSuite))
}

func (suite *EpochLookupSuite) SetupTest() {
	suite.currentEpochCounter = uint64(1)
	suite.phase = flow.EpochPhaseStaking

	suite.prevEpoch = viewRange{epochCounter: suite.currentEpochCounter - 1, firstView: 100, finalView: 199}
	suite.currEpoch = viewRange{epochCounter: suite.currentEpochCounter, firstView: 200, finalView: 299}
	suite.nextEpoch = viewRange{epochCounter: suite.currentEpochCounter + 1, firstView: 300, finalView: 399}

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
func (suite *EpochLookupSuite) CommitEpochs(epochs ...viewRange) {
	for _, epoch := range epochs {
		mockEpoch := newMockCommittedEpoch(epoch.epochCounter, epoch.firstView, epoch.finalView)
		suite.epochQuery.AddCommitted(mockEpoch)
		// if we add a next epoch (counter 1 greater than current), then set phase to committed
		if epoch.epochCounter == suite.currentEpochCounter+1 {
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

// TestEpochForView_Curr tests constructing and subsequently querying
// EpochLookup with an initial state of a current epoch.
func (suite *EpochLookupSuite) TestEpochForView_Curr() {
	epochs := []viewRange{suite.currEpoch}
	suite.CommitEpochs(epochs...)
	suite.CreateAndStartEpochLookup()
	testEpochForView(suite.T(), suite.lookup, epochs...)
}

// TestEpochForView_PrevCurr tests constructing and subsequently querying
// EpochLookup with an initial state of a previous and current epoch.
func (suite *EpochLookupSuite) TestEpochForView_PrevCurr() {
	epochs := []viewRange{suite.prevEpoch, suite.currEpoch}
	suite.CommitEpochs(epochs...)
	suite.CreateAndStartEpochLookup()
	testEpochForView(suite.T(), suite.lookup, epochs...)
}

// TestEpochForView_CurrNext tests constructing and subsequently querying
// EpochLookup with an initial state of a current and next epoch.
func (suite *EpochLookupSuite) TestEpochForView_CurrNext() {
	epochs := []viewRange{suite.currEpoch, suite.nextEpoch}
	suite.CommitEpochs(epochs...)
	suite.CreateAndStartEpochLookup()
	testEpochForView(suite.T(), suite.lookup, epochs...)
}

// TestEpochForView_CurrNextPrev tests constructing and subsequently querying
// EpochLookup with an initial state of a previous, current, and next epoch.
func (suite *EpochLookupSuite) TestEpochForView_CurrNextPrev() {
	epochs := []viewRange{suite.prevEpoch, suite.currEpoch, suite.nextEpoch}
	suite.CommitEpochs(epochs...)
	suite.CreateAndStartEpochLookup()
	testEpochForView(suite.T(), suite.lookup, epochs...)
}

// TestProtocolEvents_EpochExtended tests constructing and subsequently querying
// EpochLookup, where we process an EpochExtended event and expect the latest
// epoch final view to be updated with the updated final view of the current epoch
// in the protocol state.
func (suite *EpochLookupSuite) TestProtocolEvents_EpochExtended() {
	// previous and current epochs will be committed
	epochs := []viewRange{suite.prevEpoch, suite.currEpoch}
	suite.CommitEpochs(suite.prevEpoch, suite.currEpoch)

	suite.CreateAndStartEpochLookup()

	extension := flow.EpochExtension{
		FirstView: suite.currEpoch.finalView + 1,
		FinalView: suite.currEpoch.finalView + 100,
	}
	suite.lookup.EpochExtended(suite.currEpoch.epochCounter, nil, extension)

	// wait for the protocol event to be processed (async)
	assert.Eventually(suite.T(), func() bool {
		_, err := suite.lookup.EpochForView(extension.FinalView)
		return err == nil
	}, 5*time.Second, 50*time.Millisecond)

	// validate queries are answered correctly
	suite.currEpoch.finalView = extension.FinalView // expect final view to have been updated from extension
	testEpochForView(suite.T(), suite.lookup, epochs...)

	// should handle multiple deliveries of the protocol event
	suite.lookup.EpochExtended(suite.currEpoch.epochCounter, nil, extension)
	suite.lookup.EpochExtended(suite.currEpoch.epochCounter, nil, extension)
	suite.lookup.EpochExtended(suite.currEpoch.epochCounter, nil, extension)

	assert.Eventually(suite.T(), func() bool {
		return len(suite.lookup.epochEvents) == 0
	}, time.Second, time.Millisecond)

	// validate queries are answered correctly
	testEpochForView(suite.T(), suite.lookup, epochs...)
}

// TestProtocolEvents_EpochExtended_SanityChecks ensures all expected sanity checks are checked when processing
// EpochExtended events.
func (suite *EpochLookupSuite) TestProtocolEvents_EpochExtended_SanityChecks() {
	initAndStartLookup := func() *irrecoverable.MockSignalerContext {
		lookup, err := NewEpochLookup(suite.state)
		suite.Require().NoError(err)
		ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(suite.T(), context.Background())
		lookup.Start(ctx)

		suite.lookup = lookup
		suite.cancel = cancel

		return ctx
	}

	suite.T().Run("sanity check: `extension.FinalView` should be greater than final view of latest epoch", func(t *testing.T) {
		// initially, only current epoch is committed
		suite.CommitEpochs(suite.prevEpoch, suite.currEpoch)
		ctx := initAndStartLookup()

		// create invalid extension with final view in the past
		extension := flow.EpochExtension{
			FirstView: suite.currEpoch.finalView + 1,
			FinalView: suite.currEpoch.finalView - 100,
		}

		ctx.On("Throw", mock.AnythingOfType("*errors.errorString")).Run(func(args mock.Arguments) {
			err, ok := args.Get(0).(error)
			assert.True(suite.T(), ok)
			assert.Contains(suite.T(), err.Error(), fmt.Sprintf(invalidExtensionFinalView, suite.currEpoch.finalView, extension.FinalView))
		})

		suite.lookup.EpochExtended(suite.currEpoch.epochCounter, nil, extension)

		// wait for the protocol event to be processed (async)
		assert.Eventually(suite.T(), func() bool {
			return len(suite.lookup.epochEvents) == 0
		}, 2*time.Second, 50*time.Millisecond)
	})
	suite.T().Run("sanity check: epoch extension should have the same epoch counter as the latest epoch", func(t *testing.T) {
		// initially, only current epoch is committed
		suite.CommitEpochs(suite.prevEpoch, suite.currEpoch)
		ctx := initAndStartLookup()

		unknownCounter := uint64(100)
		ctx.On("Throw", mock.AnythingOfType("*errors.errorString")).Run(func(args mock.Arguments) {
			err, ok := args.Get(0).(error)
			assert.True(suite.T(), ok)
			assert.Contains(suite.T(), err.Error(), fmt.Sprintf(mismatchEpochCounter, suite.currEpoch.epochCounter, unknownCounter))
		})

		suite.lookup.EpochExtended(unknownCounter, nil, flow.EpochExtension{
			FirstView: suite.currEpoch.finalView + 1,
			FinalView: suite.currEpoch.finalView + 100,
		})

		// wait for the protocol event to be processed (async)
		assert.Eventually(suite.T(), func() bool {
			return len(suite.lookup.epochEvents) == 0
		}, 2*time.Second, 50*time.Millisecond)
	})
	suite.T().Run("sanity check: first view of the epoch extension should immediately start after the final view of the latest epoch", func(t *testing.T) {
		// initially, only current epoch is committed
		suite.CommitEpochs(suite.prevEpoch, suite.currEpoch)
		ctx := initAndStartLookup()

		// create invalid extension with final view in the past
		extension := flow.EpochExtension{
			FirstView: suite.currEpoch.finalView - 100,
			FinalView: suite.currEpoch.finalView + 100,
		}

		throwCalled := make(chan struct{})
		ctx.On("Throw", mock.AnythingOfType("*errors.errorString")).Run(func(args mock.Arguments) {
			err, ok := args.Get(0).(error)
			assert.True(suite.T(), ok)
			assert.Contains(suite.T(), err.Error(), fmt.Sprintf(invalidEpochViewSequence, extension.FirstView, suite.currEpoch.finalView))
			close(throwCalled)
		})

		suite.lookup.EpochExtended(suite.currEpoch.epochCounter, nil, extension)

		// wait for the protocol event to be processed (async)
		assert.Eventually(suite.T(), func() bool {
			select {
			case <-throwCalled:
				return len(suite.lookup.epochEvents) == 0
			default:
				return false
			}
		}, 2*time.Second, 50*time.Millisecond)
	})
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
		_, err := suite.lookup.EpochForView(suite.currEpoch.finalView + 1)
		return err == nil
	}, 5*time.Second, 50*time.Millisecond)

	// validate queries are answered correctly
	testEpochForView(suite.T(), suite.lookup, suite.currEpoch, suite.nextEpoch)

	// should handle multiple deliveries of the protocol event
	suite.lookup.EpochCommittedPhaseStarted(suite.currentEpochCounter, firstBlockOfCommittedPhase)
	suite.lookup.EpochCommittedPhaseStarted(suite.currentEpochCounter, firstBlockOfCommittedPhase)
	suite.lookup.EpochCommittedPhaseStarted(suite.currentEpochCounter, firstBlockOfCommittedPhase)

	// validate queries are answered correctly
	testEpochForView(suite.T(), suite.lookup, suite.currEpoch, suite.nextEpoch)
}

// testEpochForView accepts a constructed EpochLookup and state, and
// validates correctness by issuing various queries, using the input state and
// epochs as source of truth.
func testEpochForView(t *testing.T, lookup *EpochLookup, epochs ...viewRange) {
	t.Run("should be able to query within any committed epoch", func(t *testing.T) {
		for _, epoch := range epochs {
			t.Run("first view", func(t *testing.T) {
				counter, err := lookup.EpochForView(epoch.firstView)
				assert.NoError(t, err)
				assert.Equal(t, epoch.epochCounter, counter)
			})
			t.Run("final view", func(t *testing.T) {
				counter, err := lookup.EpochForView(epoch.finalView)
				assert.NoError(t, err)
				assert.Equal(t, epoch.epochCounter, counter)
			})
			t.Run("random view in range", func(t *testing.T) {
				counter, err := lookup.EpochForView(unittest.Uint64InRange(epoch.firstView, epoch.finalView))
				assert.NoError(t, err)
				assert.Equal(t, epoch.epochCounter, counter)
			})
		}
	})

	t.Run("should return ErrViewForUnknownEpoch below earliest epoch", func(t *testing.T) {
		t.Run("view 0", func(t *testing.T) {
			_, err := lookup.EpochForView(0)
			assert.ErrorIs(t, err, model.ErrViewForUnknownEpoch)
		})
		t.Run("boundary of earliest epoch", func(t *testing.T) {
			_, err := lookup.EpochForView(epochs[0].firstView - 1)
			assert.ErrorIs(t, err, model.ErrViewForUnknownEpoch)
		})
		t.Run("random view below earliest epoch", func(t *testing.T) {
			_, err := lookup.EpochForView(unittest.Uint64InRange(0, epochs[0].firstView-1))
			assert.ErrorIs(t, err, model.ErrViewForUnknownEpoch)
		})
	})

	t.Run("should return ErrViewForUnknownEpoch for queries above latest epoch final view", func(t *testing.T) {
		latest, exists := lookup.epochs.latest()
		assert.True(t, exists)
		_, err := lookup.EpochForView(latest.finalView + 1)
		assert.ErrorIs(t, err, model.ErrViewForUnknownEpoch)
	})
}

// newMockCommittedEpoch returns a mock epoch with the given properties
func newMockCommittedEpoch(counter, firstView, finalView uint64) *mockprotocol.CommittedEpoch {
	epoch := new(mockprotocol.CommittedEpoch)
	epoch.On("FirstView").Return(firstView)
	epoch.On("FinalView").Return(finalView)
	epoch.On("Counter").Return(counter)
	return epoch
}
