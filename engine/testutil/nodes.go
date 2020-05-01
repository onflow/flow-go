package testutil

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	collectioningest "github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/engine/collection/provider"
	consensusingest "github.com/dapperlabs/flow-go/engine/consensus/ingestion"
	"github.com/dapperlabs/flow-go/engine/consensus/matching"
	"github.com/dapperlabs/flow-go/engine/consensus/propagation"
	"github.com/dapperlabs/flow-go/engine/execution/computation"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/ingestion"
	executionprovider "github.com/dapperlabs/flow-go/engine/execution/provider"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/bootstrap"
	"github.com/dapperlabs/flow-go/engine/execution/sync"
	"github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/engine/verification/ingest"
	"github.com/dapperlabs/flow-go/engine/verification/verifier"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/chunks"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/stub"
	protocol "github.com/dapperlabs/flow-go/state/protocol/badger"
	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func GenericNode(t *testing.T, hub *stub.Hub, identity *flow.Identity, identities []*flow.Identity, options ...func(*protocol.State)) mock.GenericNode {
	log := zerolog.New(os.Stderr).Level(zerolog.DebugLevel)

	db, dbDir := unittest.TempBadgerDB(t)

	state, err := UncheckedState(db, flow.GenesisStateCommitment, identities)
	require.NoError(t, err)

	for _, option := range options {
		option(state)
	}

	// Generates test signing oracle for the nodes
	// Disclaimer: it should not be used for practical applications
	//
	// uses identity of node as its seed
	seed, err := json.Marshal(identity)
	require.NoError(t, err)
	// creates signing key of the node
	sk, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
	require.NoError(t, err)

	me, err := local.New(identity, sk)
	require.NoError(t, err)

	stub := stub.NewNetwork(state, me, hub)

	metrics, err := metrics.NewCollector(log)
	require.NoError(t, err)

	return mock.GenericNode{
		Log:     log,
		Metrics: metrics,
		DB:      db,
		State:   state,
		Me:      me,
		Net:     stub,
		DBDir:   dbDir,
	}
}

// CollectionNode returns a mock collection node.
func CollectionNode(t *testing.T, hub *stub.Hub, identity *flow.Identity, identities []*flow.Identity, options ...func(*protocol.State)) mock.CollectionNode {

	node := GenericNode(t, hub, identity, identities, options...)

	pool, err := stdmap.NewTransactions(1000)
	require.NoError(t, err)

	collections := storage.NewCollections(node.DB)
	transactions := storage.NewTransactions(node.DB)

	ingestionEngine, err := collectioningest.New(node.Log, node.Net, node.State, node.Metrics, node.Me, pool)
	require.Nil(t, err)

	providerEngine, err := provider.New(node.Log, node.Net, node.State, node.Metrics, node.Me, pool, collections, transactions)
	require.Nil(t, err)

	return mock.CollectionNode{
		GenericNode:     node,
		Pool:            pool,
		Collections:     collections,
		Transactions:    transactions,
		IngestionEngine: ingestionEngine,
		ProviderEngine:  providerEngine,
	}
}

// CollectionNodes returns n collection nodes connected to the given hub.
func CollectionNodes(t *testing.T, hub *stub.Hub, nNodes int, options ...func(*protocol.State)) []mock.CollectionNode {
	identities := unittest.IdentityListFixture(nNodes, func(node *flow.Identity) {
		node.Role = flow.RoleCollection
	})

	nodes := make([]mock.CollectionNode, 0, len(identities))
	for _, identity := range identities {
		nodes = append(nodes, CollectionNode(t, hub, identity, identities, options...))
	}

	return nodes
}

func ConsensusNode(t *testing.T, hub *stub.Hub, identity *flow.Identity, identities []*flow.Identity) mock.ConsensusNode {

	node := GenericNode(t, hub, identity, identities)

	results := storage.NewExecutionResults(node.DB)

	guarantees, err := stdmap.NewGuarantees(1000)
	require.NoError(t, err)

	receipts, err := stdmap.NewReceipts(1000)
	require.NoError(t, err)

	approvals, err := stdmap.NewApprovals(1000)
	require.NoError(t, err)

	seals, err := stdmap.NewSeals(1000)
	require.NoError(t, err)

	propagationEngine, err := propagation.New(node.Log, node.Net, node.State, node.Me, guarantees)
	require.NoError(t, err)

	ingestionEngine, err := consensusingest.New(node.Log, node.Net, propagationEngine, node.State, node.Metrics, node.Me)
	require.Nil(t, err)

	matchingEngine, err := matching.New(node.Log, node.Net, node.State, node.Me, results, receipts, approvals, seals)
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

	nodes := make([]mock.ConsensusNode, 0, len(identities))
	for _, identity := range identities {
		nodes = append(nodes, ConsensusNode(t, hub, identity, identities))
	}

	return nodes
}

func ExecutionNode(t *testing.T, hub *stub.Hub, identity *flow.Identity, identities []*flow.Identity, syncThreshold uint64) mock.ExecutionNode {
	node := GenericNode(t, hub, identity, identities)

	blocksStorage := storage.NewBlocks(node.DB)
	payloadsStorage := storage.NewPayloads(node.DB)
	collectionsStorage := storage.NewCollections(node.DB)
	eventsStorage := storage.NewEvents(node.DB)
	txResultStorage := storage.NewTransactionResults(node.DB)
	commitsStorage := storage.NewCommits(node.DB)
	chunkDataPackStorage := storage.NewChunkDataPacks(node.DB)
	executionResults := storage.NewExecutionResults(node.DB)

	dbDir := unittest.TempDBDir(t)

	ls, err := ledger.NewTrieStorage(dbDir)
	require.NoError(t, err)

	genesisHead, err := node.State.Final().Head()
	require.NoError(t, err)

	_, err = bootstrap.BootstrapLedger(ls)
	require.NoError(t, err)

	err = bootstrap.BootstrapExecutionDatabase(node.DB, genesisHead)
	require.NoError(t, err)

	execState := state.NewExecutionState(ls, commitsStorage, chunkDataPackStorage, executionResults, node.DB)

	stateSync := sync.NewStateSynchronizer(execState)

	providerEngine, err := executionprovider.New(node.Log, node.Net, node.State, node.Me, execState, stateSync)
	require.NoError(t, err)

	rt := runtime.NewInterpreterRuntime()
	vm := virtualmachine.New(rt)

	require.NoError(t, err)

	computationEngine := computation.New(
		node.Log,
		node.Me,
		node.State,
		vm,
	)
	require.NoError(t, err)

	ingestionEngine, err := ingestion.New(node.Log,
		node.Net,
		node.Me,
		node.State,
		blocksStorage,
		payloadsStorage,
		collectionsStorage,
		eventsStorage,
		txResultStorage,
		computationEngine,
		providerEngine,
		execState,
		syncThreshold,
		node.Metrics,
		false,
	)
	require.NoError(t, err)

	return mock.ExecutionNode{
		GenericNode:     node,
		IngestionEngine: ingestionEngine,
		ExecutionEngine: computationEngine,
		ReceiptsEngine:  providerEngine,
		BadgerDB:        node.DB,
		VM:              vm,
		ExecutionState:  execState,
		Ledger:          ls,
		LevelDbDir:      dbDir,
	}
}

type VerificationOpt func(*mock.VerificationNode)

func WithVerifierEngine(eng network.Engine) VerificationOpt {
	return func(node *mock.VerificationNode) {
		node.VerifierEngine = eng
	}
}

func VerificationNode(t *testing.T,
	hub *stub.Hub,
	identity *flow.Identity,
	identities []*flow.Identity,
	assigner module.ChunkAssigner,
	requestIntervalMs uint,
	failureThreshold uint,
	opts ...VerificationOpt) mock.VerificationNode {

	var err error
	node := mock.VerificationNode{
		GenericNode: GenericNode(t, hub, identity, identities),
	}

	for _, apply := range opts {
		apply(&node)
	}

	if node.AuthReceipts == nil {
		node.AuthReceipts, err = stdmap.NewReceipts(1000)
		require.Nil(t, err)
	}

	if node.PendingReceipts == nil {
		node.PendingReceipts, err = stdmap.NewPendingReceipts(1000)
		require.Nil(t, err)
	}

	if node.AuthCollections == nil {
		node.AuthCollections, err = stdmap.NewCollections(1000)
		require.Nil(t, err)
	}

	if node.PendingCollections == nil {
		node.PendingCollections, err = stdmap.NewPendingCollections(1000)
		require.Nil(t, err)
	}

	if node.CollectionTrackers == nil {
		node.CollectionTrackers, err = stdmap.NewCollectionTrackers(1000)
		require.Nil(t, err)
	}

	if node.ChunkDataPacks == nil {
		node.ChunkDataPacks, err = stdmap.NewChunkDataPacks(1000)
		require.Nil(t, err)
	}

	if node.ChunkDataPackTrackers == nil {
		node.ChunkDataPackTrackers, err = stdmap.NewChunkDataPackTrackers(1000)
		require.Nil(t, err)
	}

	if node.BlockStorage == nil {
		node.BlockStorage = storage.NewBlocks(node.DB)
	}

	if node.VerifierEngine == nil {
		rt := runtime.NewInterpreterRuntime()
		vm := virtualmachine.New(rt)
		chunkVerifier := chunks.NewChunkVerifier(vm)

		require.NoError(t, err)
		node.VerifierEngine, err = verifier.New(node.Log, node.Net, node.State, node.Me, chunkVerifier, node.Metrics)
		require.Nil(t, err)
	}

	if node.IngestedChunkIDs == nil {
		node.IngestedChunkIDs, err = stdmap.NewIdentifiers(1000)
		require.Nil(t, err)
	}

	if node.IngestedResultIDs == nil {
		node.IngestedResultIDs, err = stdmap.NewIdentifiers(1000)
		require.Nil(t, err)
	}

	if node.IngestEngine == nil {
		node.IngestEngine, err = ingest.New(node.Log,
			node.Net,
			node.State,
			node.Me,
			node.VerifierEngine,
			node.AuthReceipts,
			node.PendingReceipts,
			node.AuthCollections,
			node.PendingCollections,
			node.CollectionTrackers,
			node.ChunkDataPacks,
			node.ChunkDataPackTrackers,
			node.IngestedChunkIDs,
			node.IngestedResultIDs,
			node.BlockStorage,
			assigner,
			requestIntervalMs,
			failureThreshold,
		)
		require.Nil(t, err)
	}

	return node
}
