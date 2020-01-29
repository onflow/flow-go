package hotstuff

import (
	"fmt"
	"testing"

	badger "github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/local"
	protocol "github.com/dapperlabs/flow-go/protocol/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestProduceVote(t *testing.T) {
	voter := Voter{}

	eventHandler := &EventHandler{}
	fmt.Printf("%v", eventHandler)
	fmt.Printf("%v", voter)
	CreateProtocolState(t)
}

// TODO: Need to wait until viewState, signer, and validator have all been
// implemented to test produceVote. CreateProtocolState will be used then.
func CreateProtocolState(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		state, err := protocol.NewState(db)
		ids := unittest.IdentityListFixture(5, func(node *flow.Identity) {
			node.Role = flow.RoleConsensus
		})

		err = state.Mutate().Bootstrap(flow.Genesis(ids))
		if err != nil {
			panic("could not bootstrap protocol State")
		}

		trueID, err := flow.HexStringToIdentifier("node1")
		allIdentities, err := state.Final().Identities()
		fmt.Sprintf("%v", allIdentities)

		id, err := state.Final().Identity(trueID)
		// fnb.MustNot(err).Msg("could not get identity")

		me, err := local.New(id)
		fmt.Sprintf("%v", me)
	})
}
