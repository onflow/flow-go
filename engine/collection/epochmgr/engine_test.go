package epochmgr

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff"
	mockhotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	epochmgr "github.com/onflow/flow-go/engine/collection/epochmgr/mock"
	mockcollection "github.com/onflow/flow-go/engine/collection/mock"
	"github.com/onflow/flow-go/model/flow"
	realmodule "github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	mockcomponent "github.com/onflow/flow-go/module/component/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/mempool/epochs"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	realcluster "github.com/onflow/flow-go/state/cluster"
	cluster "github.com/onflow/flow-go/state/cluster/mock"
	realprotocol "github.com/onflow/flow-go/state/protocol"
	events "github.com/onflow/flow-go/state/protocol/events/mock"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

// mockComponents is a container for the mocked version of epoch components.
type mockComponents struct {
	state             *cluster.State
	prop              *mockcomponent.Component
	sync              *mockmodule.ReadyDoneAware
	hotstuff          *mockmodule.HotStuff
	voteAggregator    *mockhotstuff.VoteAggregator
	timeoutAggregator *mockhotstuff.TimeoutAggregator
	messageHub        *mockcomponent.Component
}

func newMockComponents(t *testing.T) *mockComponents {
	components := &mockComponents{
		state:             cluster.NewState(t),
		prop:              mockcomponent.NewComponent(t),
		sync:              mockmodule.NewReadyDoneAware(t),
		hotstuff:          mockmodule.NewHotStuff(t),
		voteAggregator:    mockhotstuff.NewVoteAggregator(t),
		timeoutAggregator: mockhotstuff.NewTimeoutAggregator(t),
		messageHub:        mockcomponent.NewComponent(t),
	}
	unittest.ReadyDoneify(components.prop)
	unittest.ReadyDoneify(components.sync)
	unittest.ReadyDoneify(components.hotstuff)
	unittest.ReadyDoneify(components.voteAggregator)
	unittest.ReadyDoneify(components.timeoutAggregator)
	unittest.ReadyDoneify(components.messageHub)

	components.prop.On("Start", mock.Anything)
	components.hotstuff.On("Start", mock.Anything)
	components.voteAggregator.On("Start", mock.Anything)
	components.timeoutAggregator.On("Start", mock.Anything)
	components.messageHub.On("Start", mock.Anything)
	params := cluster.NewParams(t)
	params.On("ChainID").Return(flow.ChainID("chain-id"), nil).Maybe()
	components.state.On("Params").Return(params).Maybe()
	return components
}

type Suite struct {
	suite.Suite

	// engine dependencies
	log   zerolog.Logger
	me    *mockmodule.Local
	state *protocol.State
	snap  *protocol.Snapshot
	pools *epochs.TransactionPools

	// qc voter dependencies
	signer  *mockhotstuff.Signer
	client  *mockmodule.QCContractClient
	voter   *mockmodule.ClusterRootQCVoter
	factory *epochmgr.EpochComponentsFactory
	heights *events.Heights

	epochQuery *mocks.EpochQuery
	counter    uint64                              // reflects the counter of the current epoch
	phase      flow.EpochPhase                     // phase at mocked snapshot
	header     *flow.Header                        // header at mocked snapshot
	epochs     map[uint64]*protocol.CommittedEpoch // track all epochs
	components map[uint64]*mockComponents          // track all epoch components

	ctx    irrecoverable.SignalerContext
	cancel context.CancelFunc
	errs   <-chan error

	engine *Engine

	engineEventsDistributor *mockcollection.EngineEvents
}

// MockFactoryCreate mocks the epoch factory to create epoch components for the given epoch.
func (suite *Suite) MockFactoryCreate(arg any) {
	suite.factory.On("Create", arg).
		Run(func(args mock.Arguments) {
			epoch, ok := args.Get(0).(realprotocol.CommittedEpoch)
			suite.Require().Truef(ok, "invalid type %T", args.Get(0))
			suite.components[epoch.Counter()] = newMockComponents(suite.T())
		}).
		Return(
			func(epoch realprotocol.CommittedEpoch) realcluster.State {
				return suite.ComponentsForEpoch(epoch).state
			},
			func(epoch realprotocol.CommittedEpoch) component.Component {
				return suite.ComponentsForEpoch(epoch).prop
			},
			func(epoch realprotocol.CommittedEpoch) realmodule.ReadyDoneAware {
				return suite.ComponentsForEpoch(epoch).sync
			},
			func(epoch realprotocol.CommittedEpoch) realmodule.HotStuff {
				return suite.ComponentsForEpoch(epoch).hotstuff
			},
			func(epoch realprotocol.CommittedEpoch) hotstuff.VoteAggregator {
				return suite.ComponentsForEpoch(epoch).voteAggregator
			},
			func(epoch realprotocol.CommittedEpoch) hotstuff.TimeoutAggregator {
				return suite.ComponentsForEpoch(epoch).timeoutAggregator
			},
			func(epoch realprotocol.CommittedEpoch) component.Component {
				return suite.ComponentsForEpoch(epoch).messageHub
			},
			func(epoch realprotocol.CommittedEpoch) error { return nil },
		).Maybe()
}

func (suite *Suite) SetupTest() {
	suite.log = unittest.Logger()
	suite.me = mockmodule.NewLocal(suite.T())
	suite.state = protocol.NewState(suite.T())
	suite.snap = protocol.NewSnapshot(suite.T())

	suite.epochs = make(map[uint64]*protocol.CommittedEpoch)
	suite.components = make(map[uint64]*mockComponents)

	suite.signer = mockhotstuff.NewSigner(suite.T())
	suite.client = mockmodule.NewQCContractClient(suite.T())
	suite.voter = mockmodule.NewClusterRootQCVoter(suite.T())
	suite.factory = epochmgr.NewEpochComponentsFactory(suite.T())
	suite.heights = events.NewHeights(suite.T())

	// mock out Create so that it instantiates the appropriate mocks
	suite.MockFactoryCreate(mock.Anything)

	suite.phase = flow.EpochPhaseSetup
	suite.header = unittest.BlockHeaderFixture()
	suite.epochQuery = mocks.NewEpochQuery(suite.T(), suite.counter)

	suite.state.On("Final").Return(suite.snap)
	suite.state.On("AtBlockID", suite.header.ID()).Return(suite.snap).Maybe()
	suite.snap.On("Epochs").Return(suite.epochQuery)
	suite.snap.On("Head").Return(
		func() *flow.Header { return suite.header },
		func() error { return nil })
	suite.snap.On("EpochPhase").Return(
		func() flow.EpochPhase { return suite.phase },
		func() error { return nil })

	// add current epoch
	suite.AddCommittedEpoch(suite.counter)
	// next epoch (with counter+1) is added later, as either setup/tentative (if we need to start QC)
	// or committed (if we need to transition to it) depending on the test

	suite.pools = epochs.NewTransactionPools(func(_ uint64) mempool.Transactions {
		return herocache.NewTransactions(1000, suite.log, metrics.NewNoopCollector())
	})

	suite.engineEventsDistributor = mockcollection.NewEngineEvents(suite.T())

	var err error
	suite.engine, err = New(suite.log, suite.me, suite.state, suite.pools, suite.voter, suite.factory, suite.heights, suite.engineEventsDistributor)
	suite.Require().Nil(err)

}

// StartEngine starts the engine under test, and spawns a routine to check for irrecoverable errors.
func (suite *Suite) StartEngine() {
	suite.ctx, suite.cancel, suite.errs = irrecoverable.WithSignallerAndCancel(context.Background())
	go unittest.FailOnIrrecoverableError(suite.T(), suite.ctx.Done(), suite.errs)
	suite.engine.Start(suite.ctx)
	unittest.AssertClosesBefore(suite.T(), suite.engine.Ready(), time.Second)
}

// TearDownTest stops the engine and checks for any irrecoverable errors.
func (suite *Suite) TearDownTest() {
	if suite.cancel == nil {
		return
	}
	suite.cancel()
	unittest.RequireCloseBefore(suite.T(), suite.engine.Done(), time.Second, "engine failed to stop")
	select {
	case err := <-suite.errs:
		assert.NoError(suite.T(), err)
	default:
	}
}

// TransitionEpoch triggers an epoch transition in the suite's mocks.
func (suite *Suite) TransitionEpoch() {
	suite.counter++
	require.Contains(suite.T(), suite.epochs, suite.counter)
	suite.epochQuery.Transition()
}

// AddCommittedEpoch adds a Committed Epoch with the given counter to the test suite,
// so the epoch information can be retrieved by the business logic.
func (suite *Suite) AddCommittedEpoch(counter uint64) *protocol.CommittedEpoch {
	epoch := new(protocol.CommittedEpoch)
	epoch.On("Counter").Return(counter, nil)
	suite.epochs[counter] = epoch
	suite.epochQuery.AddCommitted(epoch)
	return epoch
}

// AddTentativeEpoch adds a Tentative Epoch with the given counter to the test suite,
// so the epoch information can be retrieved by the business logic.
func (suite *Suite) AddTentativeEpoch(counter uint64) *protocol.TentativeEpoch {
	epoch := new(protocol.TentativeEpoch)
	epoch.On("Counter").Return(counter, nil)
	suite.epochQuery.AddTentative(epoch)
	return epoch
}

// AssertEpochStarted asserts that the components for the given epoch have been started.
func (suite *Suite) AssertEpochStarted(counter uint64) {
	components, ok := suite.components[counter]
	suite.Assert().True(ok, "asserting nonexistent epoch %d started", counter)
	components.prop.AssertCalled(suite.T(), "Ready")
	components.sync.AssertCalled(suite.T(), "Ready")
	components.voteAggregator.AssertCalled(suite.T(), "Ready")
	components.voteAggregator.AssertCalled(suite.T(), "Start", mock.Anything)
}

// AssertEpochStopped asserts that the components for the given epoch have been stopped.
func (suite *Suite) AssertEpochStopped(counter uint64) {
	components, ok := suite.components[counter]
	suite.Assert().True(ok, "asserting nonexistent epoch stopped", counter)
	components.prop.AssertCalled(suite.T(), "Done")
	components.sync.AssertCalled(suite.T(), "Done")
}

func (suite *Suite) ComponentsForEpoch(epoch realprotocol.CommittedEpoch) *mockComponents {
	counter := epoch.Counter()
	components, ok := suite.components[counter]
	suite.Require().True(ok, "missing component for counter", counter)
	return components
}

// MockAsUnauthorizedNode mocks the factory to return a sentinel indicating
// we are not authorized in the epoch
func (suite *Suite) MockAsUnauthorizedNode(forEpoch uint64) {

	// mock as unauthorized for given epoch only
	unauthorizedMatcher := func(epoch realprotocol.CommittedEpoch) bool {
		return epoch.Counter() == forEpoch
	}
	authorizedMatcher := func(epoch realprotocol.CommittedEpoch) bool { return !unauthorizedMatcher(epoch) }

	suite.factory = epochmgr.NewEpochComponentsFactory(suite.T())
	suite.factory.
		On("Create", mock.MatchedBy(unauthorizedMatcher)).
		Return(nil, nil, nil, nil, nil, nil, nil, ErrNotAuthorizedForEpoch)
	suite.MockFactoryCreate(mock.MatchedBy(authorizedMatcher))

	var err error
	suite.engine, err = New(suite.log, suite.me, suite.state, suite.pools, suite.voter, suite.factory, suite.heights, suite.engineEventsDistributor)
	suite.Require().Nil(err)
}

func TestEpochManager(t *testing.T) {
	suite.Run(t, new(Suite))
}

// TestRestartInSetupPhase tests that, if we start up during the setup phase,
// we should kick off the root QC voter
func (suite *Suite) TestRestartInSetupPhase() {
	// we expect 1 ActiveClustersChanged events when the engine first starts and the first set of epoch components are started
	suite.engineEventsDistributor.On("ActiveClustersChanged", mock.AnythingOfType("flow.ChainIDList")).Once()
	defer suite.engineEventsDistributor.AssertExpectations(suite.T())
	// we are in setup phase
	suite.AddTentativeEpoch(suite.counter + 1)
	suite.phase = flow.EpochPhaseSetup
	// should call voter with next epoch
	var called = make(chan struct{})
	nextEpochTentative, err := suite.epochQuery.NextUnsafe()
	require.NoError(suite.T(), err, "cannot get next tentative epoch")
	suite.voter.On("Vote", mock.Anything, nextEpochTentative).
		Return(nil).
		Run(func(args mock.Arguments) {
			close(called)
		}).Once()

	// start up the engine
	suite.StartEngine()

	unittest.AssertClosesBefore(suite.T(), called, time.Second)
}

// TestStartAfterEpochBoundary_WithinTxExpiry tests starting the engine shortly after an epoch transition.
// When the finalized height is within the first tx_expiry blocks of the new epoch
// the engine should restart the previous epoch cluster consensus.
func (suite *Suite) TestStartAfterEpochBoundary_WithinTxExpiry() {
	// we expect 2 ActiveClustersChanged events once when the engine first starts and the first set of epoch components are started and on restart
	suite.engineEventsDistributor.On("ActiveClustersChanged", mock.AnythingOfType("flow.ChainIDList")).Twice()
	defer suite.engineEventsDistributor.AssertExpectations(suite.T())
	suite.phase = flow.EpochPhaseStaking
	// transition epochs, so that a Previous epoch is queryable
	suite.AddCommittedEpoch(suite.counter + 1)
	suite.TransitionEpoch()
	prevEpoch := suite.epochs[suite.counter-1]
	// the finalized height is within [1,tx_expiry] heights of previous epoch final height
	prevEpochFinalHeight := uint64(100)
	prevEpoch.On("FinalHeight").Return(prevEpochFinalHeight, nil)
	suite.header.Height = prevEpochFinalHeight + 1
	suite.heights.On("OnHeight", prevEpochFinalHeight+flow.DefaultTransactionExpiry+1, mock.Anything)

	suite.StartEngine()
	// previous epoch components should have been started
	suite.AssertEpochStarted(suite.counter - 1)
	suite.AssertEpochStarted(suite.counter)
}

// TestStartAfterEpochBoundary_BeyondTxExpiry tests starting the engine shortly after an epoch transition.
// When the finalized height is beyond the first tx_expiry blocks of the new epoch
// the engine should NOT restart the previous epoch cluster consensus.
func (suite *Suite) TestStartAfterEpochBoundary_BeyondTxExpiry() {
	// we expect 1 ActiveClustersChanged events when the engine first starts and the first set of epoch components are started
	suite.engineEventsDistributor.On("ActiveClustersChanged", mock.AnythingOfType("flow.ChainIDList")).Once()
	defer suite.engineEventsDistributor.AssertExpectations(suite.T())
	suite.phase = flow.EpochPhaseStaking
	// transition epochs, so that a Previous epoch is queryable
	suite.AddCommittedEpoch(suite.counter + 1)
	suite.TransitionEpoch()
	prevEpoch := suite.epochs[suite.counter-1]
	// the finalized height is more than tx_expiry above previous epoch final height
	prevEpochFinalHeight := uint64(100)
	prevEpoch.On("FinalHeight").Return(prevEpochFinalHeight, nil)
	suite.header.Height = prevEpochFinalHeight + flow.DefaultTransactionExpiry + 100

	suite.StartEngine()
	// previous epoch components should not have been started
	suite.AssertEpochStarted(suite.counter)
	suite.Assert().Len(suite.components, 1)
}

// TestStartAfterEpochBoundary_NotApprovedForPreviousEpoch tests starting the engine
// shortly after an epoch transition. The finalized boundary is near enough the epoch
// boundary that we could start the previous epoch cluster consensus - however,
// since we are not approved for the epoch, we should only start current epoch components.
func (suite *Suite) TestStartAfterEpochBoundary_NotApprovedForPreviousEpoch() {
	// we expect 1 ActiveClustersChanged events when the current epoch components are started
	suite.engineEventsDistributor.On("ActiveClustersChanged", mock.AnythingOfType("flow.ChainIDList")).Once()
	defer suite.engineEventsDistributor.AssertExpectations(suite.T())
	suite.phase = flow.EpochPhaseStaking
	// transition epochs, so that a Previous epoch is queryable
	suite.AddCommittedEpoch(suite.counter + 1)
	suite.TransitionEpoch()
	prevEpoch := suite.epochs[suite.counter-1]
	// the finalized height is within [1,tx_expiry] heights of previous epoch final height
	prevEpochFinalHeight := uint64(100)
	prevEpoch.On("FinalHeight").Return(prevEpochFinalHeight, nil)
	suite.header.Height = 101
	suite.MockAsUnauthorizedNode(suite.counter - 1)

	suite.StartEngine()
	// previous epoch components should not have been started
	suite.AssertEpochStarted(suite.counter)
	suite.Assert().Len(suite.components, 1)
}

// TestStartAfterEpochBoundary_NotApprovedForCurrentEpoch tests starting the engine
// shortly after an epoch transition. The finalized boundary is near enough the epoch
// boundary that we should start the previous epoch cluster consensus. However, we are
// not approved for the current epoch -> we should only start *previous* epoch components.
func (suite *Suite) TestStartAfterEpochBoundary_NotApprovedForCurrentEpoch() {
	// we expect 1 ActiveClustersChanged events when the current epoch components are started
	suite.engineEventsDistributor.On("ActiveClustersChanged", mock.AnythingOfType("flow.ChainIDList")).Once()
	defer suite.engineEventsDistributor.AssertExpectations(suite.T())
	suite.phase = flow.EpochPhaseStaking
	// transition epochs, so that a Previous epoch is queryable
	suite.AddCommittedEpoch(suite.counter + 1)
	suite.TransitionEpoch()
	prevEpoch := suite.epochs[suite.counter-1]
	// the finalized height is within [1,tx_expiry] heights of previous epoch final height
	prevEpochFinalHeight := uint64(100)
	prevEpoch.On("FinalHeight").Return(prevEpochFinalHeight, nil)
	suite.header.Height = 101
	suite.heights.On("OnHeight", prevEpochFinalHeight+flow.DefaultTransactionExpiry+1, mock.Anything)
	suite.MockAsUnauthorizedNode(suite.counter)

	suite.StartEngine()
	// only previous epoch components should have been started
	suite.AssertEpochStarted(suite.counter - 1)
	suite.Assert().Len(suite.components, 1)
}

// TestStartAfterEpochBoundary_PreviousEpochTransitionBeforeRoot tests starting the engine
// with a root snapshot whose sealing segment excludes the last epoch boundary.
// In this case we should only start up current-epoch components.
func (suite *Suite) TestStartAfterEpochBoundary_PreviousEpochTransitionBeforeRoot() {
	// we expect 1 ActiveClustersChanged events when the current epoch components are started
	suite.engineEventsDistributor.On("ActiveClustersChanged", mock.AnythingOfType("flow.ChainIDList")).Once()
	defer suite.engineEventsDistributor.AssertExpectations(suite.T())
	suite.phase = flow.EpochPhaseStaking
	// transition epochs, so that a Previous epoch is queryable
	suite.AddCommittedEpoch(suite.counter + 1)
	suite.TransitionEpoch()
	prevEpoch := suite.epochs[suite.counter-1]
	// Previous epoch end boundary is unknown because it is before our root snapshot
	prevEpoch.On("FinalHeight").Return(uint64(0), realprotocol.ErrUnknownEpochBoundary)

	suite.StartEngine()
	// only current epoch components should have been started
	suite.AssertEpochStarted(suite.counter)
	suite.Assert().Len(suite.components, 1)
}

// TestStartAsUnauthorizedNode test that when a collection node joins the network
// at an epoch boundary, they must start running during the EpochSetup phase in the
// epoch before they become an authorized member so they submit their cluster QC vote.
//
// These nodes must kick off the root QC voter but should not attempt to participate
// in cluster consensus in the current epoch.
func (suite *Suite) TestStartAsUnauthorizedNode() {
	suite.MockAsUnauthorizedNode(suite.counter)
	// we are in setup phase
	suite.AddTentativeEpoch(suite.counter + 1)
	suite.phase = flow.EpochPhaseSetup
	// should call voter with next epoch
	var called = make(chan struct{})
	nextEpochTentative, err := suite.epochQuery.NextUnsafe()
	require.NoError(suite.T(), err, "cannot get next tentative epoch")
	suite.voter.On("Vote", mock.Anything, nextEpochTentative).
		Return(nil).
		Run(func(args mock.Arguments) {
			close(called)
		}).Once()

	// start the engine
	suite.StartEngine()

	// should have submitted vote
	unittest.AssertClosesBefore(suite.T(), called, time.Second)
	// should have no epoch components
	assert.Empty(suite.T(), suite.engine.epochs, "should have 0 epoch components")
}

// TestRespondToPhaseChange should kick off root QC voter when we receive an event
// indicating the EpochSetup phase has started.
func (suite *Suite) TestRespondToPhaseChange() {
	// we expect 1 ActiveClustersChanged events when the engine first starts and the first set of epoch components are started
	suite.engineEventsDistributor.On("ActiveClustersChanged", mock.AnythingOfType("flow.ChainIDList")).Once()
	defer suite.engineEventsDistributor.AssertExpectations(suite.T())

	// start in staking phase
	suite.phase = flow.EpochPhaseStaking
	suite.AddTentativeEpoch(suite.counter + 1)
	// should call voter with next epoch
	var called = make(chan struct{})
	nextEpochTentative, err := suite.epochQuery.NextUnsafe()
	require.NoError(suite.T(), err, "cannot get next tentative epoch")
	suite.voter.On("Vote", mock.Anything, nextEpochTentative).
		Return(nil).
		Run(func(args mock.Arguments) {
			close(called)
		}).Once()

	firstBlockOfEpochSetupPhase := unittest.BlockHeaderFixture()
	suite.state.On("AtBlockID", firstBlockOfEpochSetupPhase.ID()).Return(suite.snap)
	suite.StartEngine()

	// after receiving the protocol event, we should submit our root QC vote
	suite.engine.EpochSetupPhaseStarted(0, firstBlockOfEpochSetupPhase)
	unittest.AssertClosesBefore(suite.T(), called, time.Second)
}

// TestRespondToEpochTransition tests the engine's behaviour during epoch transition.
// It should:
//   - instantiate cluster consensus for the new epoch
//   - register callback to stop the previous epoch's cluster consensus
//   - stop the previous epoch's cluster consensus when the callback is invoked
func (suite *Suite) TestRespondToEpochTransition() {
	// we expect 3 ActiveClustersChanged events
	// - once when the engine first starts and the first set of epoch components are started
	// - once when the epoch transitions and the new set of epoch components are started
	// - once when the epoch transitions and the old set of epoch components are stopped
	expectedNumOfEvents := 3
	suite.engineEventsDistributor.On("ActiveClustersChanged", mock.AnythingOfType("flow.ChainIDList")).Times(expectedNumOfEvents)
	defer suite.engineEventsDistributor.AssertExpectations(suite.T())

	// we are in committed phase
	suite.AddCommittedEpoch(suite.counter + 1)
	suite.phase = flow.EpochPhaseCommitted
	suite.StartEngine()

	firstBlockOfEpoch := unittest.BlockHeaderFixture()
	suite.state.On("AtBlockID", firstBlockOfEpoch.ID()).Return(suite.snap)

	// should set up callback for height at which previous epoch expires
	var expiryCallback func()
	heightRegistered := make(chan struct{})
	suite.heights.On("OnHeight", firstBlockOfEpoch.Height+flow.DefaultTransactionExpiry, mock.Anything).
		Run(func(args mock.Arguments) {
			expiryCallback = args.Get(1).(func())
			close(heightRegistered)
		}).
		Once()

	// mock the epoch transition
	suite.TransitionEpoch()
	// notify the engine of the epoch transition
	suite.engine.EpochTransition(suite.counter, firstBlockOfEpoch)
	// ensure we registered a height callback
	unittest.AssertClosesBefore(suite.T(), heightRegistered, time.Second)
	suite.Assert().NotNil(expiryCallback)

	// the engine should have two epochs under management, the just ended epoch
	// and the newly started epoch
	suite.Eventually(func() bool {
		suite.engine.mu.Lock()
		defer suite.engine.mu.Unlock()
		return len(suite.engine.epochs) == 2
	}, time.Second, 10*time.Millisecond)
	_, exists := suite.engine.epochs[suite.counter-1]
	suite.Assert().True(exists, "should have previous epoch components")
	_, exists = suite.engine.epochs[suite.counter]
	suite.Assert().True(exists, "should have current epoch components")

	// the newly started (current) epoch should have been started
	suite.AssertEpochStarted(suite.counter)

	// when we invoke the callback registered to handle the previous epoch's
	// expiry, the previous epoch components should be cleaned up
	expiryCallback()

	suite.Assert().Eventually(func() bool {
		suite.engine.mu.Lock()
		defer suite.engine.mu.Unlock()
		return len(suite.engine.epochs) == 1
	}, time.Second, 10*time.Millisecond)

	// after the previous epoch expires, we should only have current epoch
	_, exists = suite.engine.epochs[suite.counter]
	suite.Assert().True(exists, "should have current epoch components")
	_, exists = suite.engine.epochs[suite.counter-1]
	suite.Assert().False(exists, "should not have previous epoch components")

	// the expired epoch should have been stopped
	suite.AssertEpochStopped(suite.counter - 1)
}

// TestStopQcVoting tests that, if we encounter an EpochEmergencyFallbackTriggered event
// the engine will stop in progress QC voting. The engine keeps track of the current in progress
// qc vote by keeping a pointer to the cancel func for the context of that process.
// When the EFM event is encountered and voting is in progress the cancel func will be invoked
// and the voting process will be stopped.
func (suite *Suite) TestStopQcVoting() {
	// we expect 1 ActiveClustersChanged events when the engine first starts and the first set of epoch components are started
	suite.engineEventsDistributor.On("ActiveClustersChanged", mock.AnythingOfType("flow.ChainIDList")).Once()

	// we are in setup phase, forces engine to start voting on startup
	suite.AddTentativeEpoch(suite.counter + 1)
	suite.phase = flow.EpochPhaseSetup

	receivedCancelSignal := make(chan struct{})
	nextEpochTentative, err := suite.epochQuery.NextUnsafe()
	require.NoError(suite.T(), err, "cannot get next tentative epoch")
	suite.voter.On("Vote", mock.Anything, nextEpochTentative).
		Return(nil).
		Run(func(args mock.Arguments) {
			ctx := args.Get(0).(context.Context)
			<-ctx.Done()
			close(receivedCancelSignal)
		}).Once()

	// start up the engine
	suite.StartEngine()

	require.NotNil(suite.T(), suite.engine.inProgressQCVote.Load(), "expected qc vote to be in progress")

	// simulate processing efm triggered event, this should cancel all in progress voting
	suite.engine.EpochEmergencyFallbackTriggered()

	unittest.AssertClosesBefore(suite.T(), receivedCancelSignal, time.Second)
}
