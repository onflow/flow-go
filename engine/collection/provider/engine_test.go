package provider_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/stub"
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

		genesis := mock.Genesis(identities)

		collNode := testutil.CollectionNode(t, hub, collID, genesis)
		consNode := testutil.ConsensusNode(t, hub, consID, genesis)

		guarantee := unittest.CollectionGuaranteeFixture()

		err := collNode.ProviderEngine.SubmitCollectionGuarantee(guarantee)
		assert.NoError(t, err)

		// flush messages from the collection node
		net, ok := hub.GetNetwork(collNode.Me.NodeID())
		require.True(t, ok)
		fmt.Println("flushing")
		net.FlushAll()
		fmt.Println("flushed")

		assert.Eventually(t, func() bool {
			has := consNode.Pool.Has(guarantee.Fingerprint())
			return has
		}, time.Millisecond*5, time.Millisecond)
	})
}
