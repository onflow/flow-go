package hotstuff

import (
	"fmt"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/consensus/eventdriven/components/voter"
	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestProduceVote(t *testing.T) {
	voter := voter.Voter{}

	eventHandler := &EventHandler{}
	fmt.Printf("%v", eventHandler)
	fmt.Printf("%v", voter)
	CreateProtocolState(t)
}

// TODO: Need to wait until viewState, signer, and validator have all been
// implemented to test produceVote. CreateProtocolState will be used then.
func CreateProtocolState(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		identities := unittest.IdentityListFixture(5, func(identity *flow.Identity) {
			identity.Role = flow.RoleConsensus
		})

		state, err := testutil.UncheckedState(db, identities)
		require.NoError(t, err)

		allIdentities, err := state.Final().Identities()
		require.NoError(t, err)

		t.Log(allIdentities)
	})
}
