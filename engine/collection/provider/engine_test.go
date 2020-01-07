package provider_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine"
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

		collID := unittest.IdentityFixture(func(id *flow.Identity) {
			id.Role = flow.RoleCollection
		})
		consID := unittest.IdentityFixture(func(id *flow.Identity) {
			id.Role = flow.RoleConsensus
		})
		identities := flow.IdentityList{collID, consID}

		genesis := unittest.Genesis(identities)

		collNode := unittest.NewCollectionNode(t, hub, collID, genesis)
		consNode := unittest.NewConsensusNode(t, hub, consID, genesis)

		guarantee := unittest.CollectionGuaranteeFixture()

		err := collNode.ProviderEngine.SubmitCollectionGuarantee(guarantee)
		assert.NoError(t, err)

		// flush messages from the collection node
		net, ok := hub.GetNetwork(collNode.Me.NodeID())
		require.True(t, ok)
		net.FlushAll()

		assert.Eventually(t, func() bool {
			has := consNode.Pool.Has(guarantee.Fingerprint())
			return has
		}, time.Millisecond*5, time.Millisecond)
	})
}

func TestCollectionRequest(t *testing.T) {
	t.Run("should respond to request", func(t *testing.T) {
		hub := stub.NewNetworkHub()

		collID := unittest.IdentityFixture(func(id *flow.Identity) {
			id.Role = flow.RoleCollection
		})
		requesterID := unittest.IdentityFixture(func(id *flow.Identity) {
			id.Role = flow.RoleExecution
		})

		identities := flow.IdentityList{collID, requesterID}
		genesis := unittest.Genesis(identities)

		collNode := unittest.NewCollectionNode(t, hub, collID, genesis)
		requesterNode := unittest.NewGenericNode(t, hub, requesterID, genesis)

		// set up a mock requester engine that will receive and verify the collection response
		requesterEngine := new(mocknetwork.Engine)
		requesterEngine.On("Process", collID.NodeID, testifymock.Anything).Return(nil).Once()
		con, err := requesterNode.Net.Register(engine.CollectionProvider, requesterEngine)
		assert.NoError(t, err)

		// save a collection to the store
		coll := unittest.CollectionFixture(3)
		err = collNode.Collections.Save(&coll)
		assert.NoError(t, err)

		// send a request for the collection
		req := messages.CollectionRequest{Fingerprint: coll.Fingerprint()}
		err = con.Submit(&req, collID.NodeID)
		assert.NoError(t, err)

		// flush the request
		net, ok := hub.GetNetwork(requesterID.NodeID)
		require.True(t, ok)
		net.FlushAll()

		// flush the response
		net, ok = hub.GetNetwork(collID.NodeID)
		require.True(t, ok)
		net.FlushAll()

		// the requester engine should have received the right collection
		res := &messages.CollectionResponse{
			Fingerprint:  coll.Fingerprint(),
			Transactions: coll.Transactions,
		}
		requesterEngine.AssertCalled(t, "Process", collID.NodeID, res)
	})

	t.Run("should return error for non-existent collection", func(t *testing.T) {
		hub := stub.NewNetworkHub()

		collID := unittest.IdentityFixture(func(id *flow.Identity) {
			id.Role = flow.RoleCollection
		})

		identities := flow.IdentityList{collID}
		genesis := unittest.Genesis(identities)

		collNode := unittest.NewCollectionNode(t, hub, collID, genesis)

		// create request with invalid/nonexistent fingerprint
		req := &messages.CollectionRequest{Fingerprint: flow.Fingerprint{}}

		// provider should return error
		err := collNode.ProviderEngine.ProcessLocal(req)
		if assert.Error(t, err) {
			assert.True(t, errors.Is(err, storage.NotFoundErr))
		}
	})
}
