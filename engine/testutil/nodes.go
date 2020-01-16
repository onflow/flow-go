package testutil

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	collectioningest "github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/engine/collection/provider"
	consensusingest "github.com/dapperlabs/flow-go/engine/consensus/ingestion"
	"github.com/dapperlabs/flow-go/engine/consensus/propagation"
	"github.com/dapperlabs/flow-go/engine/execution/blocks"
	"github.com/dapperlabs/flow-go/engine/execution/execution"
	"github.com/dapperlabs/flow-go/engine/execution/execution/executor"
	"github.com/dapperlabs/flow-go/engine/execution/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/receipts"
	"github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/engine/verification/verifier"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/network/stub"
	protocol "github.com/dapperlabs/flow-go/protocol/badger"
	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func GenericNode(t *testing.T, hub *stub.Hub, identity flow.Identity, genesis *flow.Block, options ...func(*protocol.State)) mock.GenericNode {
	log := zerolog.New(os.Stderr).Level(zerolog.DebugLevel)

	db := unittest.TempBadgerDB(t)

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
func CollectionNode(t *testing.T, hub *stub.Hub, identity flow.Identity, genesis *flow.Block, options ...func(*protocol.State)) mock.CollectionNode {

	node := GenericNode(t, hub, identity, genesis, options...)

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
func CollectionNodes(t *testing.T, hub *stub.Hub, nNodes int, options ...func(*protocol.State)) []mock.CollectionNode {
	identities := unittest.IdentityListFixture(nNodes, func(node *flow.Identity) {
		node.Role = flow.RoleCollection
	})

	genesis := mock.Genesis(identities)

	nodes := make([]mock.CollectionNode, 0, len(identities))
	for _, identity := range identities {
		nodes = append(nodes, CollectionNode(t, hub, identity, genesis, options...))
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

func ConsensusNodes(t *testing.T, hub *stub.Hub, nNodes int) []mock.ConsensusNode {
	identities := unittest.IdentityListFixture(nNodes, func(node *flow.Identity) {
		node.Role = flow.RoleConsensus
	})
	for _, id := range identities {
		t.Log(id.String())
	}

	genesis := mock.Genesis(identities)

	nodes := make([]mock.ConsensusNode, 0, len(identities))
	for _, identity := range identities {
		nodes = append(nodes, ConsensusNode(t, hub, identity, genesis))
	}

	return nodes
}

func ExecutionNode(t *testing.T, hub *stub.Hub, identity flow.Identity, genesis *flow.Block) mock.ExecutionNode {
	node := GenericNode(t, hub, identity, genesis)

	blocksStorage := storage.NewBlocks(node.DB)
	collectionsStorage := storage.NewCollections(node.DB)
	commitmentsStorage := storage.NewStateCommitments(node.DB)

	receiptsEngine, err := receipts.New(node.Log, node.Net, node.State, node.Me)
	require.NoError(t, err)

	rt := runtime.NewInterpreterRuntime()
	vm := virtualmachine.New(rt)

	levelDB := unittest.TempLevelDB(t)

	ls, err := ledger.NewTrieStorage(levelDB)
	require.NoError(t, err)

	initialStateCommitment := ls.LatestStateCommitment()

	err = commitmentsStorage.Persist(genesis.ID(), &initialStateCommitment)
	require.NoError(t, err)

	execState := state.NewExecutionState(ls, commitmentsStorage)
	blockExec := executor.NewBlockExecutor(vm, execState)

	mempool, err := blocks.NewMempool()
	require.NoError(t, err)

	execEngine, err := execution.New(
		node.Log,
		node.Net,
		node.Me,
		receiptsEngine,
		blockExec,
	)
	require.NoError(t, err)

	blocksEngine, err := blocks.New(
		node.Log,
		node.Net,
		node.Me,
		blocksStorage,
		collectionsStorage,
		node.State,
		execEngine,
		mempool,
	)
	require.NoError(t, err)

	return mock.ExecutionNode{
		GenericNode:     node,
		BlocksEngine:    blocksEngine,
		ExecutionEngine: execEngine,
		ReceiptsEngine:  receiptsEngine,
		BadgerDB:        node.DB,
		LevelDB:         levelDB,
		VM:              vm,
		State:           execState,
	}
}

func VerificationNode(t *testing.T, hub *stub.Hub, identity flow.Identity, genesis *flow.Block) mock.VerificationNode {
	var err error
	node := mock.VerificationNode{
		GenericNode: GenericNode(t, hub, identity, genesis),
	}

	node.Receipts, err = stdmap.NewReceipts()
	require.Nil(t, err)

	node.Blocks, err = stdmap.NewBlocks()
	require.Nil(t, err)

	node.Collections, err = stdmap.NewCollections()
	require.Nil(t, err)

	node.VerifierEngine, err = verifier.New(node.Log, node.Net, node.State, node.Me, node.Receipts, node.Blocks, node.Collections)
	require.Nil(t, err)

	return node
}
