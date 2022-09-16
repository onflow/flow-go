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
	"github.com/onflow/flow-go/network/mocknetwork"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	me      *module.Local
	conduit *mocknetwork.Conduit
	state   *protocol.State
	final   *protocol.Snapshot

	identities flow.IdentityList

	engine *Engine
}

func TestProviderEngine(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {

	suite.me = module.NewLocal(suite.T())
	suite.conduit = mocknetwork.NewConduit(suite.T())
	suite.state = protocol.NewState(suite.T())
	suite.final = protocol.NewSnapshot(suite.T())

	suite.engine = &Engine{
		me:      suite.me,
		state:   suite.state,
		con:     suite.conduit,
		message: metrics.NewNoopCollector(),
		tracer:  trace.NewNoopTracer(),
	}

	suite.identities = unittest.CompleteIdentitySet()
	localID := suite.identities[0].NodeID

	suite.me.On("NodeID").Return(localID)
	suite.state.On("AtBlockID", mock.Anything).Return(suite.final).Maybe()
	suite.final.On("Identities", mock.Anything).Return(
		func(f flow.IdentityFilter) flow.IdentityList { return suite.identities.Filter(f) },
		func(flow.IdentityFilter) error { return nil },
	).Maybe()
}

// TestBroadcastProposal_Success should broadcast a valid block.
func (suite *Suite) TestBroadcastProposal_Success() {
	proposal := unittest.ProposalFixture()
	proposal.Header.ProposerID = suite.me.NodeID()

	params := []interface{}{proposal}
	for _, identity := range suite.identities {
		// skip consensus nodes
		if identity.Role == flow.RoleConsensus {
			continue
		}
		params = append(params, identity.NodeID)
	}

	suite.conduit.On("Publish", params...).Return(nil).Once()

	err := suite.engine.broadcastProposal(proposal)
	suite.Assert().NoError(err)
}

func (suite *Suite) TestBroadcastProposal_Invalid() {

	proposal := unittest.ProposalFixture()
	proposal.Header.ProposerID = unittest.IdentifierFixture() // set a random proposer

	params := []interface{}{proposal}
	for _, identity := range suite.identities {
		// skip consensus nodes
		if identity.Role == flow.RoleConsensus {
			continue
		}
		params = append(params, identity.NodeID)
	}

	err := suite.engine.broadcastProposal(proposal)
	suite.Assert().Error(err)
	suite.conduit.AssertNotCalled(suite.T(), "Publish", params...)
}
