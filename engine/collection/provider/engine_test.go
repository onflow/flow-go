package provider_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	mocknetwork "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestSubmitCollectionGuarantee(t *testing.T) {
	t.Run("should submit guarantee to consensus nodes", func(t *testing.T) {
		hub := stub.NewNetworkHub()

		collID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
		consID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
		identities := flow.IdentityList{collID, consID}

		genesis := mock.Genesis(identities)

		collNode := testutil.CollectionNode(t, hub, collID, genesis)
		consNode := testutil.ConsensusNode(t, hub, consID, genesis)

		guarantee := unittest.CollectionGuaranteeFixture()

		err := collNode.ProviderEngine.SubmitCollectionGuarantee(guarantee)
		assert.NoError(t, err)

		// flush messages from the collection node
		net, ok := hub.GetNetwork(collNode.Me.NodeID())
		require.True(t, ok)
		net.DeliverAllRecursive()

		assert.Eventually(t, func() bool {
			has := consNode.Guarantees.Has(guarantee.ID())
			return has
		}, time.Millisecond*5, time.Millisecond)
	})
}

func TestCollectionRequest(t *testing.T) {
	t.Run("should respond to request", func(t *testing.T) {
		hub := stub.NewNetworkHub()

		collID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
		requesterID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

		identities := flow.IdentityList{collID, requesterID}
		genesis := mock.Genesis(identities)

		collNode := testutil.CollectionNode(t, hub, collID, genesis)
		requesterNode := testutil.GenericNode(t, hub, requesterID, genesis)

		// set up a mock requester engine that will receive and verify the collection response
		requesterEngine := new(mocknetwork.Engine)
		con, err := requesterNode.Net.Register(engine.CollectionProvider, requesterEngine)
		assert.NoError(t, err)

		// save a collection to the store
		coll := unittest.CollectionFixture(3)
		err = collNode.Collections.Store(&coll)
		assert.NoError(t, err)

		// expect that the requester will receive the collection
		expectedRes := &messages.CollectionResponse{
			Collection: coll,
		}
		requesterEngine.On("Process", collID.NodeID, expectedRes).Return(nil).Once()

		// send a request for the collection
		req := messages.CollectionRequest{ID: coll.ID()}
		err = con.Submit(&req, collID.NodeID)
		assert.NoError(t, err)

		// flush the request
		net, ok := hub.GetNetwork(requesterID.NodeID)
		require.True(t, ok)
		net.DeliverAllRecursive()

		// flush the response
		net, ok = hub.GetNetwork(collID.NodeID)
		require.True(t, ok)
		net.DeliverAllRecursive()

		//assert we received the right collection
		requesterEngine.AssertExpectations(t)
	})

	t.Run("should return error for non-existent collection", func(t *testing.T) {
		hub := stub.NewNetworkHub()

		collID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))

		identities := flow.IdentityList{collID}
		genesis := mock.Genesis(identities)

		collNode := testutil.CollectionNode(t, hub, collID, genesis)

		// create request with invalid/nonexistent fingerprint
		req := &messages.CollectionRequest{ID: flow.ZeroID}

		// provider should return error
		err := collNode.ProviderEngine.ProcessLocal(req)
		if assert.Error(t, err) {
			assert.True(t, errors.Is(err, storage.ErrNotFound))
		}
	})
}
