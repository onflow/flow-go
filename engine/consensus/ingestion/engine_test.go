// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package ingestion

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	mocknetwork "github.com/dapperlabs/flow-go/network/mock"
	mockprotocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestOnCollectionGuaranteeValid(t *testing.T) {

	prop := &mocknetwork.Engine{}
	state := &mockprotocol.State{}
	final := &mockprotocol.Snapshot{}

	e := &Engine{
		prop:  prop,
		state: state,
	}

	originID := unittest.IdentifierFixture()
	guarantee := unittest.CollectionGuaranteeFixture()
	guarantee.SignerIDs = []flow.Identifier{originID}
	guarantee.Signature = unittest.SignatureFixture()

	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	identity.NodeID = originID
	clusters := flow.NewClusterList(1)
	clusters.Add(0, identity)

	state.On("Final").Return(final).Once()
	final.On("Identity", originID).Return(identity, nil).Once()
	final.On("Clusters").Return(clusters, nil).Once()
	prop.On("SubmitLocal", guarantee).Return().Once()

	err := e.onCollectionGuarantee(originID, guarantee)
	require.NoError(t, err)

	state.AssertExpectations(t)
	final.AssertExpectations(t)
	prop.AssertExpectations(t)
}

func TestOnCollectionGuaranteeMissingIdentity(t *testing.T) {

	prop := &mocknetwork.Engine{}
	state := &mockprotocol.State{}
	final := &mockprotocol.Snapshot{}

	e := &Engine{
		prop:  prop,
		state: state,
	}

	originID := unittest.IdentifierFixture()
	guarantee := unittest.CollectionGuaranteeFixture()
	guarantee.SignerIDs = []flow.Identifier{originID}
	guarantee.Signature = unittest.SignatureFixture()

	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	identity.NodeID = originID
	clusters := flow.NewClusterList(1)
	clusters.Add(0, identity)

	state.On("Final").Return(final).Once()
	final.On("Clusters").Return(clusters, nil).Once()
	final.On("Identity", originID).Return(nil, errors.New("identity error")).Once()

	err := e.onCollectionGuarantee(originID, guarantee)
	require.Error(t, err)

	state.AssertExpectations(t)
	final.AssertExpectations(t)
	prop.AssertExpectations(t)
}

func TestOnCollectionGuaranteeInvalidRole(t *testing.T) {

	prop := &mocknetwork.Engine{}
	state := &mockprotocol.State{}
	final := &mockprotocol.Snapshot{}

	e := &Engine{
		prop:  prop,
		state: state,
	}

	originID := unittest.IdentifierFixture()
	guarantee := unittest.CollectionGuaranteeFixture()
	guarantee.SignerIDs = []flow.Identifier{originID}
	guarantee.Signature = unittest.SignatureFixture()

	// origin node has wrong role (consensus)
	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	identity.NodeID = originID
	clusters := flow.NewClusterList(1)
	clusters.Add(0, identity)

	state.On("Final").Return(final).Once()
	final.On("Clusters").Return(clusters, nil).Once()
	final.On("Identity", originID).Return(identity, nil).Once()

	err := e.onCollectionGuarantee(originID, guarantee)
	require.Error(t, err)

	state.AssertExpectations(t)
	final.AssertExpectations(t)
	prop.AssertExpectations(t)
}
