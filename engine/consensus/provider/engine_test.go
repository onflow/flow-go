// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package provider

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestOnBlockProposalValid(t *testing.T) {

	me := &module.Local{}
	state := &protocol.State{}
	final := &protocol.Snapshot{}
	con := &network.Conduit{}
	metrics := newMetrics()

	e := &Engine{
		me:      me,
		state:   state,
		con:     con,
		metrics: metrics,
	}

	proposal := unittest.ProposalFixture()
	identities := unittest.IdentityListFixture(100)
	localID := identities[0].NodeID

	params := []interface{}{proposal}
	for _, id := range identities[1:] {
		params = append(params, id.NodeID)
	}

	me.On("NodeID").Return(localID)
	state.On("Final").Return(final).Once()
	final.On("Identities", mock.Anything).Return(identities[1:], nil).Once()
	con.On("Submit", params...).Return(nil).Once()

	err := e.onBlockProposal(localID, proposal)
	require.NoError(t, err)

	me.AssertExpectations(t)
	state.AssertExpectations(t)
	final.AssertExpectations(t)
	con.AssertExpectations(t)
}

func TestOnBlockProposalRemoteOrigin(t *testing.T) {

	me := &module.Local{}
	state := &protocol.State{}
	final := &protocol.Snapshot{}
	con := &network.Conduit{}
	metrics := newMetrics()

	e := &Engine{
		me:      me,
		state:   state,
		con:     con,
		metrics: metrics,
	}

	proposal := unittest.ProposalFixture()
	identities := unittest.IdentityListFixture(100)
	localID := identities[0].NodeID
	remoteID := identities[1].NodeID

	me.On("NodeID").Return(localID)

	err := e.onBlockProposal(remoteID, proposal)
	require.Error(t, err)

	me.AssertExpectations(t)
	state.AssertExpectations(t)
	final.AssertExpectations(t)
	con.AssertExpectations(t)
}

func TestOnBlockProposalIdentitiesError(t *testing.T) {

	me := &module.Local{}
	state := &protocol.State{}
	final := &protocol.Snapshot{}
	con := &network.Conduit{}
	metrics := newMetrics()

	e := &Engine{
		me:      me,
		state:   state,
		con:     con,
		metrics: metrics,
	}

	proposal := unittest.ProposalFixture()
	identities := unittest.IdentityListFixture(100)
	localID := identities[0].NodeID

	me.On("NodeID").Return(localID)
	state.On("Final").Return(final).Once()
	final.On("Identities", mock.Anything).Return(nil, errors.New("no identities")).Once()

	err := e.onBlockProposal(localID, proposal)
	require.Error(t, err)

	me.AssertExpectations(t)
	state.AssertExpectations(t)
	final.AssertExpectations(t)
	con.AssertExpectations(t)
}

func TestOnBlockProposalSubmitFail(t *testing.T) {

	me := &module.Local{}
	state := &protocol.State{}
	final := &protocol.Snapshot{}
	con := &network.Conduit{}
	metrics := newMetrics()

	e := &Engine{
		me:      me,
		state:   state,
		con:     con,
		metrics: metrics,
	}

	proposal := unittest.ProposalFixture()
	identities := unittest.IdentityListFixture(100)
	localID := identities[0].NodeID

	params := []interface{}{proposal}
	for _, id := range identities[1:] {
		params = append(params, id.NodeID)
	}

	me.On("NodeID").Return(localID)
	state.On("Final").Return(final).Once()
	final.On("Identities", mock.Anything).Return(identities[1:], nil).Once()
	con.On("Submit", params...).Return(errors.New("submit failed")).Once()

	err := e.onBlockProposal(localID, proposal)
	require.Error(t, err)

	me.AssertExpectations(t)
	state.AssertExpectations(t)
	final.AssertExpectations(t)
	con.AssertExpectations(t)
}

func newMetrics() *module.Metrics {
	metrics := &module.Metrics{}
	metrics.On("StartBlockToSeal", mock.Anything).Return()
	return metrics
}
