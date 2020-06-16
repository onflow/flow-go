package testutil

import (
	"encoding/json"
	"os"
	"testing"
	"time"

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
	"github.com/dapperlabs/flow-go/engine/verification/finder"
	"github.com/dapperlabs/flow-go/engine/verification/match"
	"github.com/dapperlabs/flow-go/engine/verification/verifier"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/chunks"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/stub"
	protocol "github.com/dapperlabs/flow-go/state/protocol/badger"
	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func GenericNode(t testing.TB, hub *stub.Hub, identity *flow.Identity, participants []*flow.Identity, options ...func(*protocol.State)) mock.GenericNode {

	var i int
	var participant *flow.Identity
	for i, participant = range participants {
		if identity.NodeID == participant.NodeID {
			break
		}
	}

	log := zerolog.New(os.Stderr).With().Int("index", i).Hex("node_id", identity.NodeID[:]).Logger()

	dbDir := unittest.TempDir(t)
	db := unittest.BadgerDB(t, dbDir)

	metrics := metrics.NewNoopCollector()

	identities := storage.NewIdentities(metrics, db)
	guarantees := storage.NewGuarantees(metrics, db)
	seals := storage.NewSeals(metrics, db)
	headers := storage.NewHeaders(metrics, db)
	index := storage.NewIndex(metrics, db)
	payloads := storage.NewPayloads(db, index, identities, guarantees, seals)
	blocks := storage.NewBlocks(db, headers, payloads)

	state, err := protocol.NewState(metrics, db, headers, identities, seals, index, payloads, blocks)
	require.NoError(t, err)

	genesis := flow.Genesis(participants)
	err = state.Mutate().Bootstrap(unittest.GenesisStateCommitment, genesis)
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

	stubnet := stub.NewNetwork(state, me, hub)

	tracer, err := trace.NewTracer(log, "test")
	require.NoError(t, err)

	return mock.GenericNode{
		Log:        log,
		Metrics:    metrics,
		Tracer:     tracer,
		DB:         db,
		Headers:    headers,
		Identities: identities,
		Guarantees: guarantees,
		Seals:      seals,
		Payloads:   payloads,
		Blocks:     blocks,
		State:      state,
		Me:         me,
		Net:        stubnet,
		DBDir:      dbDir,
	}
}

// CollectionNode returns a mock collection node.
func CollectionNode(t *testing.T, hub *stub.Hub, identity *flow.Identity, identities []*flow.Identity, options ...func(*protocol.State)) mock.CollectionNode {

	node := GenericNode(t, hub, identity, identities, options...)

	pool, err := stdmap.NewTransactions(1000)
	require.NoError(t, err)

	transactions := storage.NewTransactions(node.Metrics, node.DB)
	collections := storage.NewCollections(node.DB, transactions)

	ingestionEngine, err := collectioningest.New(node.Log, node.Net, node.State, node.Metrics, node.Metrics, node.Me, pool, collectioningest.DefaultConfig())
	require.Nil(t, err)

	providerEngine, err := provider.New(node.Log, node.Net, node.State, node.Metrics, node.Metrics, node.Me, pool, collections, transactions)
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
	colIdentities := unittest.IdentityListFixture(nNodes, unittest.WithRole(flow.RoleCollection))

	// add some extra dummy identities so we have one of each role
	others := unittest.IdentityListFixture(5, unittest.WithAllRolesExcept(flow.RoleCollection))

	identities := append(colIdentities, others...)

	nodes := make([]mock.CollectionNode, 0, len(colIdentities))
	for _, identity := range colIdentities {
		nodes = append(nodes, CollectionNode(t, hub, identity, identities, options...))
	}

	return nodes
}

func ConsensusNode(t *testing.T, hub *stub.Hub, identity *flow.Identity, identities []*flow.Identity) mock.ConsensusNode {

	node := GenericNode(t, hub, identity, identities)

	resultsDB := storage.NewExecutionResults(node.DB)
	sealsDB := storage.NewSeals(node.Metrics, node.DB)

	guarantees, err := stdmap.NewGuarantees(1000)
	require.NoError(t, err)

	results, err := stdmap.NewResults(1000)
	require.NoError(t, err)

	receipts, err := stdmap.NewReceipts(1000)
	require.NoError(t, err)

	approvals, err := stdmap.NewApprovals(1000)
	require.NoError(t, err)

	seals, err := stdmap.NewSeals(1000)
	require.NoError(t, err)

	propagationEngine, err := propagation.New(node.Log, node.Metrics, node.Metrics, node.Metrics, node.Net, node.State, node.Me, guarantees)
	require.NoError(t, err)

	ingestionEngine, err := consensusingest.New(node.Log, node.Metrics, node.Metrics, node.Net, propagationEngine, node.State, node.Headers, node.Me)
	require.Nil(t, err)

	matchingEngine, err := matching.New(node.Log, node.Metrics, node.Metrics, node.Net, node.State, node.Me, resultsDB, sealsDB, node.Headers, results, receipts, approvals, seals)
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
	conIdentities := unittest.IdentityListFixture(nNodes, unittest.WithRole(flow.RoleConsensus))
	for _, id := range conIdentities {
		t.Log(id.String())
	}

	// add some extra dummy identities so we have one of each role
	others := unittest.IdentityListFixture(5, unittest.WithAllRolesExcept(flow.RoleConsensus))

	identities := append(conIdentities, others...)

	nodes := make([]mock.ConsensusNode, 0, len(conIdentities))
	for _, identity := range conIdentities {
		nodes = append(nodes, ConsensusNode(t, hub, identity, identities))
	}

	return nodes
}

func ExecutionNode(t *testing.T, hub *stub.Hub, identity *flow.Identity, identities []*flow.Identity, syncThreshold uint64) mock.ExecutionNode {
	node := GenericNode(t, hub, identity, identities)

	transactionsStorage := storage.NewTransactions(node.Metrics, node.DB)
	collectionsStorage := storage.NewCollections(node.DB, transactionsStorage)
	eventsStorage := storage.NewEvents(node.DB)
	txResultStorage := storage.NewTransactionResults(node.DB)
	commitsStorage := storage.NewCommits(node.Metrics, node.DB)
	chunkDataPackStorage := storage.NewChunkDataPacks(node.DB)
	executionResults := storage.NewExecutionResults(node.DB)

	dbDir := unittest.TempDir(t)

	metricsCollector := &metrics.NoopCollector{}
	ls, err := ledger.NewMTrieStorage(dbDir, 100, metricsCollector, nil)
	require.NoError(t, err)

	genesisHead, err := node.State.Final().Head()
	require.NoError(t, err)

	commit, err := bootstrap.BootstrapLedger(ls, unittest.ServiceAccountPublicKey, unittest.GenesisTokenSupply)
	require.NoError(t, err)

	err = bootstrap.BootstrapExecutionDatabase(node.DB, commit, genesisHead)
	require.NoError(t, err)

	execState := state.NewExecutionState(
		ls, commitsStorage, node.Blocks, collectionsStorage, chunkDataPackStorage, executionResults, node.DB, node.Tracer,
	)

	stateSync := sync.NewStateSynchronizer(execState)

	providerEngine, err := executionprovider.New(
		node.Log, node.Tracer, node.Net, node.State, node.Me, execState, stateSync,
	)
	require.NoError(t, err)

	rt := runtime.NewInterpreterRuntime()
	vm, err := virtualmachine.New(rt)

	require.NoError(t, err)

	computationEngine := computation.New(
		node.Log,
		node.Tracer,
		node.Me,
		node.State,
		vm,
		node.Blocks,
	)
	require.NoError(t, err)

	ingestionEngine, err := ingestion.New(
		node.Log,
		node.Net,
		node.Me,
		node.State,
		node.Blocks,
		node.Payloads,
		collectionsStorage,
		eventsStorage,
		txResultStorage,
		computationEngine,
		providerEngine,
		execState,
		syncThreshold,
		node.Metrics,
		node.Tracer,
		false,
		2137*time.Hour, // just don't retry
		10,
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
		Collections:     collectionsStorage,
	}
}

type VerificationOpt func(*mock.VerificationNode)

func WithVerifierEngine(eng network.Engine) VerificationOpt {
	return func(node *mock.VerificationNode) {
		node.VerifierEngine = eng
	}
}

func WithMatchEngine(eng network.Engine) VerificationOpt {
	return func(node *mock.VerificationNode) {
		node.MatchEngine = eng
	}
}

func VerificationNode(t testing.TB,
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

	if node.Receipts == nil {
		node.Receipts, err = stdmap.NewReceipts(1000)
		require.Nil(t, err)
	}

	if node.PendingReceipts == nil {
		node.PendingReceipts, err = stdmap.NewPendingReceipts(1000)
		require.Nil(t, err)
	}

	if node.PendingResults == nil {
		node.PendingResults = stdmap.NewPendingResults()
		require.Nil(t, err)
	}

	if node.HeaderStorage == nil {
		node.HeaderStorage = storage.NewHeaders(node.Metrics, node.DB)
	}

	if node.Chunks == nil {
		node.Chunks = match.NewChunks(1000)
	}

	if node.IngestedResultIDs == nil {
		node.IngestedResultIDs, err = stdmap.NewIdentifiers(1000)
		require.Nil(t, err)
	}

	if node.ReceiptIDsByBlock == nil {
		node.ReceiptIDsByBlock, err = stdmap.NewIdentifierMap(1000)
		require.Nil(t, err)
	}

	if node.VerifierEngine == nil {
		rt := runtime.NewInterpreterRuntime()
		vm, err := virtualmachine.New(rt)
		require.NoError(t, err)
		chunkVerifier := chunks.NewChunkVerifier(vm, node.Blocks)

		require.NoError(t, err)
		node.VerifierEngine, err = verifier.New(node.Log, node.Metrics, node.Net, node.State, node.Me, chunkVerifier)
		require.Nil(t, err)
	}

	if node.MatchEngine == nil {
		node.MatchEngine, err = match.New(node.Log,
			node.Net,
			node.Me,
			node.PendingResults,
			node.VerifierEngine,
			assigner,
			node.State,
			node.Chunks,
			node.HeaderStorage,
			time.Duration(requestIntervalMs)*time.Millisecond,
			int(failureThreshold))
		require.Nil(t, err)
	}

	if node.FinderEngine == nil {
		node.FinderEngine, err = finder.New(node.Log,
			node.Net,
			node.Me,
			node.MatchEngine,
			node.PendingReceipts,
			node.Headers,
			node.IngestedResultIDs,
			node.ReceiptIDsByBlock)
		require.Nil(t, err)
	}

	return node
}
