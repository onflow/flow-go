package unittest

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
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/protocol"
	badgerprotocol "github.com/dapperlabs/flow-go/protocol/badger"
	storage "github.com/dapperlabs/flow-go/storage"
	badgerstorage "github.com/dapperlabs/flow-go/storage/badger"
)

// GenericNode implements a generic node for tests.
type GenericNode struct {
	Log   zerolog.Logger
	DB    *badger.DB
	State protocol.State
	Me    module.Local
	Net   *stub.Network
}

// CollectionNode implements a mocked collection node for tests.
type CollectionNode struct {
	GenericNode
	Pool            module.TransactionPool
	Collections     storage.Collections
	IngestionEngine *collectioningest.Engine
	ProviderEngine  *provider.Engine
}

// ConsensusNode implements a mocked consensus node for tests.
type ConsensusNode struct {
	GenericNode
	Pool              module.CollectionGuaranteePool
	IngestionEngine   *consensusingest.Engine
	PropagationEngine *propagation.Engine
}

// NewGenericNode returns a generic node.
func NewGenericNode(t *testing.T, hub *stub.Hub, identity flow.Identity, genesis *flow.Block) GenericNode {
	log := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)

	dir := filepath.Join(os.TempDir(), fmt.Sprintf("flow-test-db-%d", rand.Uint64()))
	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	require.NoError(t, err)

	state, err := badgerprotocol.NewState(db)
	require.NoError(t, err)

	err = state.Mutate().Bootstrap(genesis)
	require.NoError(t, err)

	me, err := local.New(identity)
	require.NoError(t, err)

	stub := stub.NewNetwork(state, me, hub)

	return GenericNode{
		Log:   log,
		DB:    db,
		State: state,
		Me:    me,
		Net:   stub,
	}
}

// NewCollectionNode returns a collection node.
func NewCollectionNode(t *testing.T, hub *stub.Hub, identity flow.Identity, genesis *flow.Block) CollectionNode {

	node := NewGenericNode(t, hub, identity, genesis)

	pool, err := mempool.NewTransactionPool()
	require.NoError(t, err)

	collections := badgerstorage.NewCollections(node.DB)

	ingestionEngine, err := collectioningest.New(node.Log, node.Net, node.State, node.Me, pool)
	require.Nil(t, err)

	providerEngine, err := provider.New(node.Log, node.Net, node.State, node.Me, collections)

	return CollectionNode{
		GenericNode:     node,
		Pool:            pool,
		Collections:     collections,
		IngestionEngine: ingestionEngine,
		ProviderEngine:  providerEngine,
	}
}

// NewCollectionNodeSet returns a group of n collection nodes connected to the given hub.
func NewCollectionNodeSet(t *testing.T, hub *stub.Hub, n int) []CollectionNode {
	identities := IdentityListFixture(n, func(node *flow.Identity) {
		node.Role = flow.RoleCollection
	})
	for _, id := range identities {
		t.Log(id.String())
	}

	genesis := Genesis(identities)

	nodes := make([]CollectionNode, n)
	for i := 0; i < n; i++ {
		nodes[i] = NewCollectionNode(t, hub, identities[i], genesis)
	}

	return nodes
}

// NewConsensusNode returns a consensus node.
func NewConsensusNode(t *testing.T, hub *stub.Hub, identity flow.Identity, genesis *flow.Block) ConsensusNode {

	node := NewGenericNode(t, hub, identity, genesis)

	pool, err := mempool.NewCollectionPool()
	require.NoError(t, err)

	propagationEngine, err := propagation.New(node.Log, node.Net, node.State, node.Me, pool)
	require.NoError(t, err)

	ingestionEngine, err := consensusingest.New(node.Log, node.Net, propagationEngine, node.State, node.Me)
	require.Nil(t, err)

	return ConsensusNode{
		GenericNode:       node,
		Pool:              pool,
		PropagationEngine: propagationEngine,
		IngestionEngine:   ingestionEngine,
	}
}

// NewConsensusNodeSet returns a group of n consensus nodes connected to the given hub.
func NewConsensusNodeSet(t *testing.T, hub *stub.Hub, n int) []ConsensusNode {
	identities := IdentityListFixture(n, func(node *flow.Identity) {
		node.Role = flow.RoleConsensus
	})
	for _, id := range identities {
		t.Log(id.String())
	}

	genesis := Genesis(identities)

	nodes := make([]ConsensusNode, n)
	for i := 0; i < n; i++ {
		nodes[i] = NewConsensusNode(t, hub, identities[i], genesis)
	}

	return nodes
}
