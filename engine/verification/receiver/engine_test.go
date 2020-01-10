package receiver_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	networkmock "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestCollectionReceiver(t *testing.T) {

	hub := stub.NewNetworkHub()

	collID := unittest.IdentityFixture(func(id *flow.Identity) {
		id.Role = flow.RoleCollection
	})
	verID := unittest.IdentityFixture(func(id *flow.Identity) {
		id.Role = flow.RoleVerification
	})

	identities := flow.IdentityList{collID, verID}
	genesis := mock.Genesis(identities)

	// create a mock collection that we will request from the collection node
	coll := unittest.CollectionFixture(3)

	// create mock verifier engine, expect it to be called with the collection
	mockVerifierEng := new(networkmock.Engine)
	mockVerifierEng.On("SubmitLocal", &coll).Once()

	verNode := testutil.VerificationNode(t, hub, verID, genesis, testutil.WithVerifierEngine(mockVerifierEng))
	collNode := testutil.CollectionNode(t, hub, collID, genesis)

	// save the collection to the collection node's store
	err := collNode.Collections.Save(&coll)
	assert.Nil(t, err)

	// request the collection
	err = verNode.CollectionReceiverEngine.RequestCollection(coll.Fingerprint())
	assert.Nil(t, err)

	// flush the request
	net, ok := hub.GetNetwork(verID.NodeID)
	assert.True(t, ok)
	net.FlushAll()

	// flush the response
	net, ok = hub.GetNetwork(collID.NodeID)
	assert.True(t, ok)
	net.FlushAll()

	// assert that the collection was received by verifier engine
	mockVerifierEng.AssertExpectations(t)
}
