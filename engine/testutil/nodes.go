package testutil

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	collectioningest "github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/engine/collection/provider"
	consensusingest "github.com/dapperlabs/flow-go/engine/consensus/ingestion"
	"github.com/dapperlabs/flow-go/engine/consensus/matching"
	"github.com/dapperlabs/flow-go/engine/consensus/propagation"
	"github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/engine/verification/ingest"
	"github.com/dapperlabs/flow-go/engine/verification/verifier"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/stub"
	protocol "github.com/dapperlabs/flow-go/protocol/badger"
	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func GenericNode(t *testing.T, hub *stub.Hub, identity *flow.Identity, genesis *flow.Block, options ...func(*protocol.State)) mock.GenericNode {
	log := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)

	dir := filepath.Join(os.TempDir(), fmt.Sprintf("flow-test-db-%d", rand.Uint64()))
	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	require.NoError(t, err)

	state, err := protocol.NewState(db, options...)
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
func CollectionNode(t *testing.T, hub *stub.Hub, identity *flow.Identity, genesis *flow.Block, options ...func(*protocol.State)) mock.CollectionNode {

	node := GenericNode(t, hub, identity, genesis, options...)

	pool, err := stdmap.NewTransactions()
	require.NoError(t, err)

	collections := storage.NewCollections(node.DB)

	ingestionEngine, err := collectioningest.New(node.Log, node.Net, node.State, &opentracing.NoopTracer{}, node.Me, pool)
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
func CollectionNodes(t *testing.T, hub *stub.Hub, nNodes int, options ...func(*protocol.State)) []mock.CollectionNode {
	identities := unittest.IdentityListFixture(nNodes, func(node *flow.Identity) {
		node.Role = flow.RoleCollection
	})

	genesis := flow.Genesis(identities)

	nodes := make([]mock.CollectionNode, 0, len(identities))
	for _, identity := range identities {
		nodes = append(nodes, CollectionNode(t, hub, identity, genesis, options...))
	}

	return nodes
}

func ConsensusNode(t *testing.T, hub *stub.Hub, identity *flow.Identity, genesis *flow.Block) mock.ConsensusNode {

	node := GenericNode(t, hub, identity, genesis)

	guarantees, err := stdmap.NewGuarantees()
	require.NoError(t, err)

	approvals, err := stdmap.NewApprovals()
	require.NoError(t, err)

	receipts, err := stdmap.NewReceipts()
	require.NoError(t, err)

	seals, err := stdmap.NewSeals()
	require.NoError(t, err)

	propagationEngine, err := propagation.New(node.Log, node.Net, node.State, node.Me, guarantees)
	require.NoError(t, err)

	ingestionEngine, err := consensusingest.New(node.Log, node.Net, propagationEngine, node.State, node.Me)
	require.Nil(t, err)

	matchingEngine, err := matching.New(node.Log, node.Net, node.State, node.Me, receipts, approvals, seals)
	require.Nil(t, err)

	return mock.ConsensusNode{
		GenericNode:       node,
		Guarantees:        guarantees,
		Approvals:         approvals,
		Receipts:          receipts,
		Seals:             seals,
		PropagationEngine: propagationEngine,
		IngestionEngine:   ingestionEngine,
		MatchingEngine:    matchingEngine,
	}
}

func ConsensusNodes(t *testing.T, hub *stub.Hub, nNodes int) []mock.ConsensusNode {
	identities := unittest.IdentityListFixture(nNodes, func(node *flow.Identity) {
		node.Role = flow.RoleConsensus
	})
	for _, id := range identities {
		t.Log(id.String())
	}

	genesis := flow.Genesis(identities)

	nodes := make([]mock.ConsensusNode, 0, len(identities))
	for _, identity := range identities {
		nodes = append(nodes, ConsensusNode(t, hub, identity, genesis))
	}

	return nodes
}

type VerificationOpt func(*mock.VerificationNode)

func WithVerifierEngine(eng network.Engine) VerificationOpt {
	return func(node *mock.VerificationNode) {
		node.VerifierEngine = eng
	}
}

func VerificationNode(t *testing.T, hub *stub.Hub, identity *flow.Identity, genesis *flow.Block, opts ...VerificationOpt) mock.VerificationNode {

	var err error
	node := mock.VerificationNode{
		GenericNode: GenericNode(t, hub, identity, genesis),
	}

	for _, apply := range opts {
		apply(&node)
	}

	if node.Receipts == nil {
		node.Receipts, err = stdmap.NewReceipts()
		require.Nil(t, err)
	}

	if node.Blocks == nil {
		node.Blocks, err = stdmap.NewBlocks()
		require.Nil(t, err)
	}

	if node.Collections == nil {
		node.Collections, err = stdmap.NewCollections()
		require.Nil(t, err)
	}

	if node.ChunkStates == nil {
		node.ChunkStates, err = stdmap.NewChunkStates()
		require.Nil(t, err)
	}

	if node.VerifierEngine == nil {
		node.VerifierEngine, err = verifier.New(node.Log, node.Net, node.State, node.Me)
		require.Nil(t, err)
	}

	if node.IngestEngine == nil {
		node.IngestEngine, err = ingest.New(node.Log, node.Net, node.State, node.Me, node.VerifierEngine, node.Receipts, node.Blocks, node.Collections, node.ChunkStates)
		require.Nil(t, err)
	}

	return node
}
