package testutil

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/network/stub"
	protocol "github.com/dapperlabs/flow-go/protocol/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// CollectionNode returns a mock collection node.
func CollectionNode(t *testing.T, hub *stub.Hub, identity flow.Identity, genesis *flow.Block) *mock.CollectionNode {
	log := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)

	me, err := local.New(identity)
	require.NoError(t, err)

	dir := filepath.Join(os.TempDir(), fmt.Sprintf("flow-test-db-%d", rand.Uint64()))
	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	require.NoError(t, err)

	state, err := protocol.NewState(db)
	require.NoError(t, err)

	err = state.Mutate().Bootstrap(genesis)
	require.NoError(t, err)

	stub := stub.NewNetwork(state, me, hub)
	pool, err := mempool.NewTransactionPool()
	require.NoError(t, err)

	ingestionEngine, err := ingest.New(log, stub, me, state, pool)
	require.Nil(t, err)

	return &mock.CollectionNode{
		State:           state,
		Me:              me,
		IngestionEngine: ingestionEngine,
		Pool:            pool,
	}
}

// CollectionNodes returns n collection nodes connected to the given hub.
func CollectionNodes(t *testing.T, hub *stub.Hub, n int) []*mock.CollectionNode {
	identities := unittest.IdentityListFixture(n, func(node *flow.Identity) {
		node.Role = flow.RoleCollection
	})
	for _, id := range identities {
		t.Log(id.String())
	}

	genesis := mock.Genesis(identities)

	nodes := make([]*mock.CollectionNode, n)
	for i := 0; i < n; i++ {
		nodes[i] = CollectionNode(t, hub, identities[i], genesis)
	}

	return nodes
}
