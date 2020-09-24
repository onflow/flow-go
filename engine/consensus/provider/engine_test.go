// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package provider

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	network "github.com/onflow/flow-go/network/mock"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestOnBlockProposalValid(t *testing.T) {

	me := &module.Local{}
	state := &protocol.State{}
	final := &protocol.Snapshot{}
	con := &network.Conduit{}
	metrics := metrics.NewNoopCollector()

	e := &Engine{
		me:      me,
		state:   state,
		con:     con,
		message: metrics,
		tracer:  trace.NewNoopTracer(),
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
	con.On("Publish", params...).Return(nil).Once()

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
	metrics := metrics.NewNoopCollector()

	e := &Engine{
		me:      me,
		state:   state,
		con:     con,
		message: metrics,
		tracer:  trace.NewNoopTracer(),
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
	metrics := metrics.NewNoopCollector()

	e := &Engine{
		me:      me,
		state:   state,
		con:     con,
		message: metrics,
		tracer:  trace.NewNoopTracer(),
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
	metrics := metrics.NewNoopCollector()

	e := &Engine{
		me:      me,
		state:   state,
		con:     con,
		message: metrics,
		tracer:  trace.NewNoopTracer(),
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
	con.On("Publish", params...).Return(errors.New("submit failed")).Once()

	err := e.onBlockProposal(localID, proposal)
	require.Error(t, err)

	me.AssertExpectations(t)
	state.AssertExpectations(t)
	final.AssertExpectations(t)
	con.AssertExpectations(t)
}
