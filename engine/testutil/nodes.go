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

	collectioningest "github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/engine/collection/provider"
	consensusingest "github.com/dapperlabs/flow-go/engine/consensus/ingestion"
	"github.com/dapperlabs/flow-go/engine/consensus/propagation"
	"github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/engine/verification/verifier"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/network/stub"
	protocol "github.com/dapperlabs/flow-go/protocol/badger"
	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func GenericNode(t *testing.T, hub *stub.Hub, identity flow.Identity, genesis *flow.Block) mock.GenericNode {
	log := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)

	dir := filepath.Join(os.TempDir(), fmt.Sprintf("flow-test-db-%d", rand.Uint64()))
	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	require.NoError(t, err)

	state, err := protocol.NewState(db)
	require.NoError(t, err)

	err = state.Mutate().Bootstrap(genesis)
	require.NoError(t, err)

	me, err := local.New(identity)
	require.NoError(t, err)

	stub := stub.NewNetwork(state, me, hub)

	return mock.GenericNode{
		Log:   log,
		DB:    db,
		State: state,
		Me:    me,
		Net:   stub,
	}
}

// CollectionNode returns a mock collection node.
func CollectionNode(t *testing.T, hub *stub.Hub, identity flow.Identity, genesis *flow.Block) mock.CollectionNode {

	node := GenericNode(t, hub, identity, genesis)

	pool, err := stdmap.NewTransactions()
	require.NoError(t, err)

	collections := storage.NewCollections(node.DB)

	ingestionEngine, err := collectioningest.New(node.Log, node.Net, node.State, node.Me, pool)
	require.Nil(t, err)

	providerEngine, err := provider.New(node.Log, node.Net, node.State, node.Me, collections)
	require.Nil(t, err)

	return mock.CollectionNode{
		GenericNode:     node,
		Pool:            pool,
		Collections:     collections,
		IngestionEngine: ingestionEngine,
		ProviderEngine:  providerEngine,
	}
}

// CollectionNodes returns n collection nodes connected to the given hub.
func CollectionNodes(t *testing.T, hub *stub.Hub, n int) []mock.CollectionNode {
	identities := unittest.IdentityListFixture(n, func(node *flow.Identity) {
		node.Role = flow.RoleCollection
	})
	for _, id := range identities {
		t.Log(id.String())
	}

	genesis := mock.Genesis(identities)

	nodes := make([]mock.CollectionNode, n)
	for i := 0; i < n; i++ {
		nodes[i] = CollectionNode(t, hub, identities[i], genesis)
	}

	return nodes
}

func ConsensusNode(t *testing.T, hub *stub.Hub, identity flow.Identity, genesis *flow.Block) mock.ConsensusNode {

	node := GenericNode(t, hub, identity, genesis)

	pool, err := stdmap.NewGuarantees()
	require.NoError(t, err)

	propagationEngine, err := propagation.New(node.Log, node.Net, node.State, node.Me, pool)
	require.NoError(t, err)

	ingestionEngine, err := consensusingest.New(node.Log, node.Net, propagationEngine, node.State, node.Me)
	require.Nil(t, err)

	return mock.ConsensusNode{
		GenericNode:       node,
		Pool:              pool,
		PropagationEngine: propagationEngine,
		IngestionEngine:   ingestionEngine,
	}
}

func ConsensusNodes(t *testing.T, hub *stub.Hub, n int) []mock.ConsensusNode {
	identities := unittest.IdentityListFixture(n, func(node *flow.Identity) {
		node.Role = flow.RoleConsensus
	})
	for _, id := range identities {
		t.Log(id.String())
	}

	genesis := mock.Genesis(identities)

	nodes := make([]mock.ConsensusNode, n)
	for i := 0; i < n; i++ {
		nodes[i] = ConsensusNode(t, hub, identities[i], genesis)
	}

	return nodes
}

func VerificationNode(t *testing.T, hub *stub.Hub, identity flow.Identity, genesis *flow.Block) mock.VerificationNode {

	var err error
	node := mock.VerificationNode{
		GenericNode: GenericNode(t, hub, identity, genesis),
	}

	node.Receipts, err = stdmap.NewReceipts()
	require.Nil(t, err)

	node.Approvals, err = stdmap.NewApprovals()
	require.Nil(t, err)

	node.VerifierEngine, err = verifier.New(node.Log, node.Net, node.State, node.Me, node.Receipts, node.Approvals)
	require.NoError(t, err)

	return node
}
