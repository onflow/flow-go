// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package provider

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	network "github.com/onflow/flow-go/network/mock"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	me         *module.Local
	conduit    *network.Conduit
	state      *protocol.State
	final      *protocol.Snapshot
	epochQuery *protocol.EpochQuery
	nextEpoch  *protocol.Epoch

	currentEpochIdentities flow.IdentityList
	nextEpochIdentities    flow.IdentityList

	engine *Engine
}

func TestProviderEngine(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {

	suite.me = new(module.Local)
	suite.conduit = new(network.Conduit)
	suite.state = new(protocol.State)
	suite.final = new(protocol.Snapshot)
	suite.epochQuery = new(protocol.EpochQuery)
	suite.nextEpoch = new(protocol.Epoch)

	suite.engine = &Engine{
		me:      suite.me,
		state:   suite.state,
		con:     suite.conduit,
		message: metrics.NewNoopCollector(),
		tracer:  trace.NewNoopTracer(),
	}

	suite.currentEpochIdentities = unittest.CompleteIdentitySet()
	suite.nextEpochIdentities = unittest.CompleteIdentitySet()
	localID := suite.currentEpochIdentities[0].NodeID

	suite.me.On("NodeID").Return(localID)
	suite.state.On("Final").Return(suite.final)
	suite.final.On("Identities", mock.Anything).Return(
		func(f flow.IdentityFilter) flow.IdentityList { return suite.currentEpochIdentities[1:].Filter(f) },
		func(flow.IdentityFilter) error { return nil },
	)
	suite.final.On("Epochs").Return(suite.epochQuery)
	suite.epochQuery.On("Next").Return(suite.nextEpoch)
	suite.nextEpoch.On("InitialIdentities").Return(
		func() flow.IdentityList { return suite.nextEpochIdentities },
		func() error { return nil },
	)

}

// proposals submitted by remote nodes should not be accepted.
func (suite Suite) TestOnBlockProposal_RemoteOrigin() {

	proposal := unittest.ProposalFixture()
	// message submitted by remote node
	err := suite.engine.onBlockProposal(suite.currentEpochIdentities[1].NodeID, proposal)
	suite.Assert().Error(err)
}

// for a regular valid block proposal during epoch staking phase, we should send
// only to current epoch participants (we don't know about next epoch yet)
func (suite *Suite) TestOnBlockProposal_EpochStakingPhase() {

	proposal := unittest.ProposalFixture()

	// we're in staking phase
	suite.final.On("Phase").Return(flow.EpochPhaseStaking, nil)

	params := []interface{}{proposal}
	for _, id := range suite.currentEpochIdentities[1:] {
		// skip consensus nodes
		if id.Role == flow.RoleConsensus {
			continue
		}
		params = append(params, id.NodeID)
	}
	suite.conduit.On("Publish", params...).Return(nil).Once()

	err := suite.engine.onBlockProposal(suite.me.NodeID(), proposal)
	suite.Require().Nil(err)
	suite.conduit.AssertExpectations(suite.T())
}

// when we're in setup phase, we should include current epoch participants and
// next epoch participants
func (suite *Suite) TestOnBlockProposal_EpochSetupPhase() {

	proposal := unittest.ProposalFixture()

	// we're in setup phase
	suite.final.On("Phase").Return(flow.EpochPhaseSetup, nil)

	params := []interface{}{proposal}
	lookup := make(map[flow.Identifier]struct{})
	for _, id := range append(suite.currentEpochIdentities[1:], suite.nextEpochIdentities...) {
		// skip consensus nodes
		if id.Role == flow.RoleConsensus {
			continue
		}
		// don't duplicate nodes
		if _, exists := lookup[id.NodeID]; !exists {
			params = append(params, id.NodeID)
		}
	}
	suite.conduit.On("Publish", params...).Return(nil).Once()

	err := suite.engine.onBlockProposal(suite.me.NodeID(), proposal)
	suite.Require().Nil(err)
	suite.conduit.AssertExpectations(suite.T())
}

// when we're in committed phase, we should include current epoch participants and
// next epoch participants
func (suite *Suite) TestOnBlockProposal_EpochCommittedPhase() {

	proposal := unittest.ProposalFixture()

	// we're in setup phase
	suite.final.On("Phase").Return(flow.EpochPhaseCommitted, nil)

	params := []interface{}{proposal}
	lookup := make(map[flow.Identifier]struct{})
	for _, id := range append(suite.currentEpochIdentities[1:], suite.nextEpochIdentities...) {
		// skip consensus nodes
		if id.Role == flow.RoleConsensus {
			continue
		}
		// don't duplicate nodes
		if _, exists := lookup[id.NodeID]; !exists {
			params = append(params, id.NodeID)
		}
	}
	suite.conduit.On("Publish", params...).Return(nil).Once()

	err := suite.engine.onBlockProposal(suite.me.NodeID(), proposal)
	suite.Require().Nil(err)
	suite.conduit.AssertExpectations(suite.T())
}
