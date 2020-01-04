// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package ingestion

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestOnCollectionGuaranteeValid(t *testing.T) {

	prop := &network.Engine{}
	state := &protocol.State{}
	final := &protocol.Snapshot{}

	e := &Engine{
		prop:  prop,
		state: state,
	}

	originID := unittest.IdentifierFixture()
	coll := &flow.CollectionGuarantee{Hash: unittest.HashFixture(32)}
	id := unittest.IdentityFixture()
	id.Role = flow.RoleCollection

	state.On("Final").Return(final).Once()
	final.On("Identity", originID).Return(id, nil).Once()
	prop.On("SubmitLocal", coll).Return().Once()

	err := e.onCollectionGuarantee(originID, coll)
	require.NoError(t, err)

	state.AssertExpectations(t)
	final.AssertExpectations(t)
	prop.AssertExpectations(t)
}

func TestOnCollectionGuaranteeMissingIdentity(t *testing.T) {

	prop := &network.Engine{}
	state := &protocol.State{}
	final := &protocol.Snapshot{}

	e := &Engine{
		prop:  prop,
		state: state,
	}

	originID := unittest.IdentifierFixture()
	coll := &flow.CollectionGuarantee{Hash: unittest.HashFixture(32)}
	id := unittest.IdentityFixture()
	id.Role = flow.RoleCollection

	state.On("Final").Return(final).Once()
	final.On("Identity", originID).Return(flow.Identity{}, errors.New("identity error")).Once()

	err := e.onCollectionGuarantee(originID, coll)
	require.Error(t, err)

	state.AssertExpectations(t)
	final.AssertExpectations(t)
	prop.AssertExpectations(t)
}

func TestOnCollectionGuaranteeInvalidRole(t *testing.T) {

	prop := &network.Engine{}
	state := &protocol.State{}
	final := &protocol.Snapshot{}

	e := &Engine{
		prop:  prop,
		state: state,
	}

	originID := unittest.IdentifierFixture()
	coll := &flow.CollectionGuarantee{Hash: unittest.HashFixture(32)}
	id := unittest.IdentityFixture()
	id.Role = flow.RoleConsensus

	state.On("Final").Return(final).Once()
	final.On("Identity", originID).Return(id, nil).Once()

	err := e.onCollectionGuarantee(originID, coll)
	require.Error(t, err)

	state.AssertExpectations(t)
	final.AssertExpectations(t)
	prop.AssertExpectations(t)
}
