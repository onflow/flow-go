package epochmgr

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	hotstuff "github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	epochmgr "github.com/dapperlabs/flow-go/engine/collection/epochmgr/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	module "github.com/dapperlabs/flow-go/module/mock"
	cluster "github.com/dapperlabs/flow-go/state/cluster/mock"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
)

type Suite struct {
	suite.Suite

	// engine dependencies
	me    *module.Local
	state *protocol.State

	// qc voter dependencies
	signer  *hotstuff.Signer
	client  *module.QCContractClient
	voter   *module.ClusterRootQCVoter
	factory *epochmgr.EpochComponentsFactory

	snap       *protocol.Snapshot
	epochQuery *protocol.EpochQuery
	currEpoch  *protocol.Epoch
	nextEpoch  *protocol.Epoch

	hotstuff *module.HotStuff
	sync     *module.Engine
	prop     *module.Engine

	engine *Engine
}

func (suite *Suite) SetupTest() {

	log := zerolog.New(ioutil.Discard)
	suite.me = new(module.Local)
	suite.state = new(protocol.State)

	suite.signer = new(hotstuff.Signer)
	suite.client = new(module.QCContractClient)
	suite.voter = new(module.ClusterRootQCVoter)
	suite.factory = new(epochmgr.EpochComponentsFactory)

	clusterState := new(cluster.State)
	suite.hotstuff = new(module.HotStuff)
	suite.sync = new(module.Engine)
	suite.prop = new(module.Engine)
	suite.factory.On("Create", mock.Anything).Return(
		clusterState, suite.prop, suite.sync, suite.hotstuff, nil,
	)

	rwReady := make(chan struct{})
	close(rwReady)
	var ready <-chan struct{} = rwReady
	suite.hotstuff.On("Ready").Return(ready)
	suite.prop.On("Ready").Return(ready)
	suite.sync.On("Ready").Return(ready)

	suite.snap = new(protocol.Snapshot)
	suite.epochQuery = new(protocol.EpochQuery)
	suite.currEpoch = new(protocol.Epoch)
	suite.nextEpoch = new(protocol.Epoch)

	suite.state.On("Final").Return(suite.snap)
	suite.snap.On("Epochs").Return(suite.epochQuery)
	suite.epochQuery.On("Current").Return(suite.currEpoch)
	suite.epochQuery.On("Next").Return(suite.nextEpoch)

	var err error
	suite.engine, err = New(log, suite.me, suite.state, suite.voter, suite.factory)
	suite.Require().Nil(err)
}

func TestEpochManager(t *testing.T) {
	suite.Run(t, new(Suite))
}

// if we start up during the setup phase, we should kick off the root QC voter
func (suite *Suite) TestRestartInSetupPhase() {

	suite.snap.On("Phase").Return(flow.EpochPhaseSetup, nil)
	// should call voter with next epoch
	var called bool
	suite.voter.On("Vote", mock.Anything, suite.nextEpoch).
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
	suite.voter.On("Vote", mock.Anything, suite.nextEpoch).
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
