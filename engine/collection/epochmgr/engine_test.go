package epochmgr

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	epochmgr "github.com/onflow/flow-go/engine/collection/epochmgr/mock"
	"github.com/onflow/flow-go/model/flow"
	realmodule "github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/mempool/epochs"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	module "github.com/onflow/flow-go/module/mock"
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
	state    *cluster.State
	prop     *module.Engine
	sync     *module.Engine
	hotstuff *module.HotStuff
}

func newMockComponents() *mockComponents {

	components := &mockComponents{
		state:    new(cluster.State),
		prop:     new(module.Engine),
		sync:     new(module.Engine),
		hotstuff: new(module.HotStuff),
	}
	unittest.ReadyDoneify(components.prop)
	unittest.ReadyDoneify(components.sync)
	unittest.ReadyDoneify(components.hotstuff)

	return components
}

type Suite struct {
	suite.Suite

	// engine dependencies
	me    *module.Local
	state *protocol.State
	snap  *protocol.Snapshot

	// qc voter dependencies
	signer  *hotstuff.Signer
	client  *module.QCContractClient
	voter   *module.ClusterRootQCVoter
	factory *epochmgr.EpochComponentsFactory
	heights *events.Heights

	epochQuery *mocks.EpochQuery
	counter    uint64                     // reflects the counter of the current epoch
	epochs     map[uint64]*protocol.Epoch // track all epochs
	components map[uint64]*mockComponents // track all epoch components

	engine *Engine
}

func (suite *Suite) SetupTest() {

	log := zerolog.New(ioutil.Discard)
	suite.me = new(module.Local)
	suite.state = new(protocol.State)
	suite.snap = new(protocol.Snapshot)

	suite.epochs = make(map[uint64]*protocol.Epoch)
	suite.components = make(map[uint64]*mockComponents)

	suite.signer = new(hotstuff.Signer)
	suite.client = new(module.QCContractClient)
	suite.voter = new(module.ClusterRootQCVoter)
	suite.factory = new(epochmgr.EpochComponentsFactory)
	suite.heights = new(events.Heights)

	// mock out Create so that it instantiates the appropriate mocks
	suite.factory.On("Create", mock.Anything).
		Run(func(args mock.Arguments) {
			epoch, ok := args.Get(0).(realprotocol.Epoch)
			suite.Require().Truef(ok, "invalid type %T", args.Get(0))
			counter, err := epoch.Counter()
			suite.Require().Nil(err)
			suite.components[counter] = newMockComponents()
		}).
		Return(
			func(epoch realprotocol.Epoch) realcluster.State { return suite.ComponentsForEpoch(epoch).state },
			func(epoch realprotocol.Epoch) realmodule.Engine { return suite.ComponentsForEpoch(epoch).prop },
			func(epoch realprotocol.Epoch) realmodule.Engine { return suite.ComponentsForEpoch(epoch).sync },
			func(epoch realprotocol.Epoch) realmodule.HotStuff { return suite.ComponentsForEpoch(epoch).hotstuff },
			func(epoch realprotocol.Epoch) error { return nil },
		)

	suite.epochQuery = mocks.NewEpochQuery(suite.T(), suite.counter)
	suite.state.On("Final").Return(suite.snap)
	suite.snap.On("Epochs").Return(suite.epochQuery)

	// add current and next epochs
	suite.AddEpoch(suite.counter)
	suite.AddEpoch(suite.counter + 1)

	pools := epochs.NewTransactionPools(func() mempool.Transactions { return stdmap.NewTransactions(1000) })

	var err error
	suite.engine, err = New(log, suite.me, suite.state, pools, suite.voter, suite.factory, suite.heights)
	suite.Require().Nil(err)
}

func TestEpochManager(t *testing.T) {
	suite.Run(t, new(Suite))
}

// TransitionEpoch triggers an epoch transition in the suite's mocks.
func (suite *Suite) TransitionEpoch() {
	suite.counter++
	suite.epochQuery.Transition()
}

// AddEpoch adds an epoch with the given counter.
func (suite *Suite) AddEpoch(counter uint64) *protocol.Epoch {
	epoch := new(protocol.Epoch)
	epoch.On("Counter").Return(counter, nil)
	suite.epochs[counter] = epoch
	suite.epochQuery.Add(epoch)
	return epoch
}

// AssertEpochStarted asserts that the components for the given epoch have been started.
func (suite *Suite) AssertEpochStarted(counter uint64) {
	components, ok := suite.components[counter]
	suite.Assert().True(ok, "asserting nonexistent epoch started", counter)
	components.hotstuff.AssertCalled(suite.T(), "Ready")
	components.prop.AssertCalled(suite.T(), "Ready")
	components.sync.AssertCalled(suite.T(), "Ready")
}

// AssertEpochStopped asserts that the components for the given epoch have been stopped.
func (suite *Suite) AssertEpochStopped(counter uint64) {
	components, ok := suite.components[counter]
	suite.Assert().True(ok, "asserting nonexistent epoch stopped", counter)
	components.hotstuff.AssertCalled(suite.T(), "Done")
	components.prop.AssertCalled(suite.T(), "Done")
	components.sync.AssertCalled(suite.T(), "Done")
}

func (suite *Suite) ComponentsForEpoch(epoch realprotocol.Epoch) *mockComponents {
	counter, err := epoch.Counter()
	suite.Require().Nil(err, "cannot get counter")
	components, ok := suite.components[counter]
	suite.Require().True(ok, "missing component for counter", counter)
	return components
}

// if we start up during the setup phase, we should kick off the root QC voter
func (suite *Suite) TestRestartInSetupPhase() {

	suite.snap.On("Phase").Return(flow.EpochPhaseSetup, nil)
	// should call voter with next epoch
	var called bool
	suite.voter.On("Vote", mock.Anything, suite.epochQuery.Next()).
		Return(nil).
		Run(func(args mock.Arguments) {
			called = true
		}).Once()

	// start up the engine
	<-suite.engine.Ready()
	suite.Assert().Eventually(func() bool {
		return called
	}, time.Second, time.Millisecond)

	suite.voter.AssertExpectations(suite.T())
}

// should kick off root QC voter on setup phase start event
func (suite *Suite) TestRespondToPhaseChange() {

	// should call voter with next epoch
	var called bool
	suite.voter.On("Vote", mock.Anything, suite.epochQuery.Next()).
		Return(nil).
		Run(func(args mock.Arguments) {
			called = true
		}).Once()

	suite.engine.EpochSetupPhaseStarted(0, nil)
	suite.Assert().Eventually(func() bool {
		return called
	}, time.Second, time.Millisecond)

	suite.voter.AssertExpectations(suite.T())
}

func (suite *Suite) TestRespondToEpochTransition() {

	first := unittest.BlockHeaderFixture()

	// should set up callback for height at which previous epoch expires
	var expiryCallback func()
	suite.heights.On("OnHeight", first.Height+flow.DefaultTransactionExpiry, mock.Anything).
		Run(func(args mock.Arguments) {
			expiryCallback = args.Get(1).(func())
		}).
		Once()

	// mock the epoch transition
	suite.TransitionEpoch()
	// notify the engine of the epoch transition
	suite.engine.EpochTransition(suite.counter, &first)

	suite.Assert().Eventually(func() bool {
		return expiryCallback != nil
	}, time.Second, time.Millisecond)

	// the engine should have two epochs under management, the just ended epoch
	// and the newly started epoch
	suite.Assert().Len(suite.engine.epochs, 2)
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
		return len(suite.engine.epochs) == 1
	}, time.Second, time.Millisecond)

	// after the previous epoch expires, we should only have current epoch
	_, exists = suite.engine.epochs[suite.counter]
	suite.Assert().True(exists, "should have current epoch components")
	_, exists = suite.engine.epochs[suite.counter-1]
	suite.Assert().False(exists, "should not have previous epoch components")

	// the expired epoch should have been stopped
	suite.AssertEpochStopped(suite.counter - 1)
}
