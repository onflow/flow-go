package testutil

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/consensus"
	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	mockhotstuff "github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine"
	collectioningest "github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/engine/collection/pusher"
	"github.com/dapperlabs/flow-go/engine/common/follower"
	"github.com/dapperlabs/flow-go/engine/common/provider"
	"github.com/dapperlabs/flow-go/engine/common/requester"
	"github.com/dapperlabs/flow-go/engine/common/synchronization"
	consensusingest "github.com/dapperlabs/flow-go/engine/consensus/ingestion"
	"github.com/dapperlabs/flow-go/engine/consensus/matching"
	"github.com/dapperlabs/flow-go/engine/execution/computation"
	"github.com/dapperlabs/flow-go/engine/execution/ingestion"
	executionprovider "github.com/dapperlabs/flow-go/engine/execution/provider"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	bootstrapexec "github.com/dapperlabs/flow-go/engine/execution/state/bootstrap"
	testmock "github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/engine/verification/finder"
	"github.com/dapperlabs/flow-go/engine/verification/match"
	"github.com/dapperlabs/flow-go/engine/verification/verifier"
	"github.com/dapperlabs/flow-go/fvm"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/buffer"
	"github.com/dapperlabs/flow-go/module/chunks"
	confinalizer "github.com/dapperlabs/flow-go/module/finalizer/consensus"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/module/metrics"
	chainsync "github.com/dapperlabs/flow-go/module/synchronization"
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/stub"
	protocol "github.com/dapperlabs/flow-go/state/protocol/badger"
	"github.com/dapperlabs/flow-go/state/protocol/events"
	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func GenericNode(t testing.TB, hub *stub.Hub, identity *flow.Identity, participants []*flow.Identity, chainID flow.ChainID, options ...func(*protocol.State)) testmock.GenericNode {

	var i int
	var participant *flow.Identity
	for i, participant = range participants {
		if identity.NodeID == participant.NodeID {
			break
		}
	}

	log := zerolog.New(os.Stderr).With().Int("index", i).Hex("node_id", identity.NodeID[:]).Str("role", identity.Role.String()).Logger()

	dbDir := unittest.TempDir(t)
	db := unittest.BadgerDB(t, dbDir)

	metrics := metrics.NewNoopCollector()

	guarantees := storage.NewGuarantees(metrics, db)
	seals := storage.NewSeals(metrics, db)
	headers := storage.NewHeaders(metrics, db)
	index := storage.NewIndex(metrics, db)
	payloads := storage.NewPayloads(db, index, guarantees, seals)
	blocks := storage.NewBlocks(db, headers, payloads)
	setups := storage.NewEpochSetups(metrics, db)
	commits := storage.NewEpochCommits(metrics, db)
	consumer := events.NewNoop()
	statuses := storage.NewEpochStatuses(metrics, db)

	state, err := protocol.NewState(metrics, db, headers, seals, index, payloads, blocks, setups, commits, statuses, consumer)
	require.NoError(t, err)

	root, result, seal := unittest.BootstrapFixture(participants)
	err = state.Mutate().Bootstrap(root, result, seal)
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

	// sets staking public key of the node
	identity.StakingPubKey = sk.PublicKey()

	me, err := local.New(identity, sk)
	require.NoError(t, err)

	stubnet := stub.NewNetwork(state, me, hub)

	tracer, err := trace.NewTracer(log, "test")
	require.NoError(t, err)

	return testmock.GenericNode{
		Log:        log,
		Metrics:    metrics,
		Tracer:     tracer,
		DB:         db,
		Headers:    headers,
		Guarantees: guarantees,
		Seals:      seals,
		Payloads:   payloads,
		Blocks:     blocks,
		Index:      index,
		State:      state,
		Me:         me,
		Net:        stubnet,
		DBDir:      dbDir,
		ChainID:    chainID,
	}
}

// CollectionNode returns a mock collection node.
func CollectionNode(t *testing.T, hub *stub.Hub, identity *flow.Identity, identities []*flow.Identity, chainID flow.ChainID, options ...func(*protocol.State)) testmock.CollectionNode {

	node := GenericNode(t, hub, identity, identities, chainID, options...)

	pool, err := stdmap.NewTransactions(1000)
	require.NoError(t, err)

	transactions := storage.NewTransactions(node.Metrics, node.DB)
	collections := storage.NewCollections(node.DB, transactions)

	ingestionEngine, err := collectioningest.New(node.Log, node.Net, node.State, node.Metrics, node.Metrics, node.Me, pool, collectioningest.DefaultConfig())
	require.NoError(t, err)

	selector := filter.HasRole(flow.RoleAccess, flow.RoleVerification)
	retrieve := func(collID flow.Identifier) (flow.Entity, error) {
		coll, err := collections.ByID(collID)
		return coll, err
	}
	providerEngine, err := provider.New(node.Log, node.Metrics, node.Net, node.Me, node.State, engine.ProvideCollections, selector, retrieve)
	require.NoError(t, err)

	pusherEngine, err := pusher.New(node.Log, node.Net, node.State, node.Metrics, node.Metrics, node.Me, pool, collections, transactions)
	require.NoError(t, err)

	return testmock.CollectionNode{
		GenericNode:     node,
		Pool:            pool,
		Collections:     collections,
		Transactions:    transactions,
		IngestionEngine: ingestionEngine,
		PusherEngine:    pusherEngine,
		ProviderEngine:  providerEngine,
	}
}

// CollectionNodes returns n collection nodes connected to the given hub.
func CollectionNodes(t *testing.T, hub *stub.Hub, nNodes int, chainID flow.ChainID, options ...func(*protocol.State)) []testmock.CollectionNode {
	colIdentities := unittest.IdentityListFixture(nNodes, unittest.WithRole(flow.RoleCollection))

	// add some extra dummy identities so we have one of each role
	others := unittest.IdentityListFixture(5, unittest.WithAllRolesExcept(flow.RoleCollection))

	identities := append(colIdentities, others...)

	nodes := make([]testmock.CollectionNode, 0, len(colIdentities))
	for _, identity := range colIdentities {
		nodes = append(nodes, CollectionNode(t, hub, identity, identities, chainID, options...))
	}

	return nodes
}

func ConsensusNode(t *testing.T, hub *stub.Hub, identity *flow.Identity, identities []*flow.Identity, chainID flow.ChainID) testmock.ConsensusNode {

	node := GenericNode(t, hub, identity, identities, chainID)

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

	ingestionEngine, err := consensusingest.New(node.Log, node.Tracer, node.Metrics, node.Metrics, node.Metrics, node.Net, node.State, node.Headers, node.Me, guarantees)
	require.Nil(t, err)

	requesterEng, err := requester.New(node.Log, node.Metrics, node.Net, node.Me, node.State, engine.RequestReceiptsByBlockID, filter.Any, func() flow.Entity { return &flow.ExecutionReceipt{} })
	require.Nil(t, err)

	matchingEngine, err := matching.New(node.Log, node.Metrics, node.Tracer, node.Metrics, node.Net, node.State, node.Me, requesterEng, resultsDB, sealsDB, node.Headers, node.Index, results, receipts, approvals, seals)
	require.Nil(t, err)

	return testmock.ConsensusNode{
		GenericNode:     node,
		Guarantees:      guarantees,
		Approvals:       approvals,
		Receipts:        receipts,
		Seals:           seals,
		IngestionEngine: ingestionEngine,
		MatchingEngine:  matchingEngine,
	}
}

func ConsensusNodes(t *testing.T, hub *stub.Hub, nNodes int, chainID flow.ChainID) []testmock.ConsensusNode {
	conIdentities := unittest.IdentityListFixture(nNodes, unittest.WithRole(flow.RoleConsensus))
	for _, id := range conIdentities {
		t.Log(id.String())
	}

	// add some extra dummy identities so we have one of each role
	others := unittest.IdentityListFixture(5, unittest.WithAllRolesExcept(flow.RoleConsensus))

	identities := append(conIdentities, others...)

	nodes := make([]testmock.ConsensusNode, 0, len(conIdentities))
	for _, identity := range conIdentities {
		nodes = append(nodes, ConsensusNode(t, hub, identity, identities, chainID))
	}

	return nodes
}

func ExecutionNode(t *testing.T, hub *stub.Hub, identity *flow.Identity, identities []*flow.Identity, syncThreshold uint64, chainID flow.ChainID) testmock.ExecutionNode {
	node := GenericNode(t, hub, identity, identities, chainID)

	transactionsStorage := storage.NewTransactions(node.Metrics, node.DB)
	collectionsStorage := storage.NewCollections(node.DB, transactionsStorage)
	eventsStorage := storage.NewEvents(node.DB)
	txResultStorage := storage.NewTransactionResults(node.DB)
	commitsStorage := storage.NewCommits(node.Metrics, node.DB)
	chunkDataPackStorage := storage.NewChunkDataPacks(node.DB)
	results := storage.NewExecutionResults(node.DB)
	receipts := storage.NewExecutionReceipts(node.DB, results)

	pendingBlocks := buffer.NewPendingBlocks() // for following main chain consensus

	dbDir := unittest.TempDir(t)

	metricsCollector := &metrics.NoopCollector{}
	ls, err := ledger.NewMTrieStorage(dbDir, 100, metricsCollector, nil)
	require.NoError(t, err)

	genesisHead, err := node.State.Final().Head()
	require.NoError(t, err)

	bootstrapper := bootstrapexec.NewBootstrapper(node.Log)
	commit, err := bootstrapper.BootstrapLedger(ls, unittest.ServiceAccountPublicKey, unittest.GenesisTokenSupply, node.ChainID.Chain())
	require.NoError(t, err)

	err = bootstrapper.BootstrapExecutionDatabase(node.DB, commit, genesisHead)
	require.NoError(t, err)

	execState := state.NewExecutionState(
		ls, commitsStorage, node.Blocks, collectionsStorage, chunkDataPackStorage, results, receipts, node.DB, node.Tracer,
	)

	requestEngine, err := requester.New(
		node.Log, node.Metrics, node.Net, node.Me, node.State,
		engine.RequestCollections,
		filter.HasRole(flow.RoleCollection),
		func() flow.Entity { return &flow.Collection{} },
	)
	require.NoError(t, err)

	metrics := metrics.NewNoopCollector()
	pusherEngine, err := executionprovider.New(
		node.Log, node.Tracer, node.Net, node.State, node.Me, execState, metrics,
	)
	require.NoError(t, err)

	rt := runtime.NewInterpreterRuntime()

	vm := fvm.New(rt)

	vmCtx := fvm.NewContext(
		fvm.WithChain(node.ChainID.Chain()),
		fvm.WithBlocks(node.Blocks),
	)

	computationEngine, err := computation.New(
		node.Log,
		node.Metrics,
		node.Tracer,
		node.Me,
		node.State,
		vm,
		vmCtx,
	)
	require.NoError(t, err)

	syncCore, err := chainsync.New(node.Log, chainsync.DefaultConfig())
	require.NoError(t, err)

	rootHead, rootQC := getRoot(t, &node)
	ingestionEngine, err := ingestion.New(
		node.Log,
		node.Net,
		node.Me,
		requestEngine,
		node.State,
		node.Blocks,
		collectionsStorage,
		eventsStorage,
		txResultStorage,
		computationEngine,
		pusherEngine,
		execState,
		node.Metrics,
		node.Tracer,
		false,
		rootHead,
	)
	require.NoError(t, err)
	requestEngine.WithHandle(ingestionEngine.OnCollection)

	followerCore := createFollowerCore(t, &node, ingestionEngine, rootHead, rootQC)

	// initialize cleaner for DB
	cleaner := storage.NewCleaner(node.Log, node.DB, node.Metrics, flow.DefaultValueLogGCFrequency)

	followerEng, err := follower.New(node.Log, node.Net, node.Me, node.Metrics, node.Metrics, cleaner,
		node.Headers, node.Payloads, node.State, pendingBlocks, followerCore, syncCore)
	require.NoError(t, err)

	syncEngine, err := synchronization.New(
		node.Log,
		node.Metrics,
		node.Net,
		node.Me,
		node.State,
		node.Blocks,
		followerEng,
		syncCore,
		synchronization.WithPollInterval(time.Duration(0)),
	)
	require.NoError(t, err)

	return testmock.ExecutionNode{
		GenericNode:     node,
		IngestionEngine: ingestionEngine,
		FollowerEngine:  followerEng,
		SyncEngine:      syncEngine,
		ExecutionEngine: computationEngine,
		RequestEngine:   requestEngine,
		ReceiptsEngine:  pusherEngine,
		BadgerDB:        node.DB,
		VM:              vm,
		ExecutionState:  execState,
		Ledger:          ls,
		LevelDbDir:      dbDir,
		Collections:     collectionsStorage,
	}
}

func getRoot(t *testing.T, node *testmock.GenericNode) (*flow.Header, *model.QuorumCertificate) {
	rootHead, err := node.State.AtHeight(0).Head()
	require.NoError(t, err)

	signers, err := node.State.AtHeight(0).Identities(filter.HasRole(flow.RoleConsensus))
	require.NoError(t, err)

	signerIDs := signers.NodeIDs()

	rootQC := &model.QuorumCertificate{
		View:      rootHead.View,
		BlockID:   rootHead.ID(),
		SignerIDs: signerIDs,
		SigData:   unittest.SignatureFixture(),
	}

	return rootHead, rootQC
}

type RoundRobinLeaderSelection struct {
	identities flow.IdentityList
	me         flow.Identifier
}

func (s *RoundRobinLeaderSelection) Identities(blockID flow.Identifier, selector flow.IdentityFilter) (flow.IdentityList, error) {
	return s.identities.Filter(selector), nil
}

func (s *RoundRobinLeaderSelection) Identity(blockID flow.Identifier, participantID flow.Identifier) (*flow.Identity, error) {
	id, found := s.identities.ByNodeID(participantID)
	if !found {
		return nil, fmt.Errorf("not found")
	}
	return id, nil
}

func (s *RoundRobinLeaderSelection) LeaderForView(view uint64) (flow.Identifier, error) {
	return s.identities[int(view)%len(s.identities)].NodeID, nil
}

func (s *RoundRobinLeaderSelection) Self() flow.Identifier {
	return s.me
}

func createFollowerCore(t *testing.T, node *testmock.GenericNode, notifier hotstuff.FinalizationConsumer, rootHead *flow.Header, rootQC *model.QuorumCertificate) module.HotStuffFollower {

	identities, err := node.State.AtHeight(0).Identities(filter.HasRole(flow.RoleConsensus))
	require.NoError(t, err)

	committee := &RoundRobinLeaderSelection{
		identities: identities,
		me:         node.Me.NodeID(),
	}

	// mock finalization updater
	verifier := &mockhotstuff.Verifier{}
	verifier.On("VerifyVote", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	verifier.On("VerifyQC", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	finalizer := confinalizer.NewFinalizer(node.DB, node.Headers, node.State)

	pending := make([]*flow.Header, 0)

	// creates a consensus follower with noop consumer as the notifier
	followerCore, err := consensus.NewFollower(
		node.Log,
		committee,
		node.Headers,
		finalizer,
		verifier,
		notifier,
		rootHead,
		rootQC,
		rootHead,
		pending,
	)
	require.NoError(t, err)
	return followerCore
}

type VerificationOpt func(*testmock.VerificationNode)

func WithVerifierEngine(eng network.Engine) VerificationOpt {
	return func(node *testmock.VerificationNode) {
		node.VerifierEngine = eng
	}
}

func WithMatchEngine(eng network.Engine) VerificationOpt {
	return func(node *testmock.VerificationNode) {
		node.MatchEngine = eng
	}
}

func VerificationNode(t testing.TB,
	hub *stub.Hub,
	identity *flow.Identity,
	identities []*flow.Identity,
	assigner module.ChunkAssigner,
	requestInterval time.Duration,
	processInterval time.Duration,
	failureThreshold uint,
	chainID flow.ChainID,
	collector module.VerificationMetrics, // used to enable collecting metrics on happy path integration
	mempoolCollector module.MempoolMetrics, // used to enable collecting metrics on happy path integration
	opts ...VerificationOpt) testmock.VerificationNode {

	var err error
	node := testmock.VerificationNode{
		GenericNode: GenericNode(t, hub, identity, identities, chainID),
	}

	for _, apply := range opts {
		apply(&node)
	}

	if node.CachedReceipts == nil {
		node.CachedReceipts, err = stdmap.NewReceiptDataPacks(1000)
		require.Nil(t, err)
		// registers size method of backend for metrics
		err = mempoolCollector.Register(metrics.ResourceCachedReceipt, node.CachedReceipts.Size)
		require.Nil(t, err)
	}

	if node.PendingReceipts == nil {
		node.PendingReceipts, err = stdmap.NewReceiptDataPacks(1000)
		require.Nil(t, err)

		// registers size method of backend for metrics
		err = mempoolCollector.Register(metrics.ResourcePendingReceipt, node.PendingReceipts.Size)
		require.Nil(t, err)
	}

	if node.ReadyReceipts == nil {
		node.ReadyReceipts, err = stdmap.NewReceiptDataPacks(1000)
		require.Nil(t, err)
		// registers size method of backend for metrics
		err = mempoolCollector.Register(metrics.ResourceReceipt, node.ReadyReceipts.Size)
		require.Nil(t, err)
	}

	if node.PendingResults == nil {
		node.PendingResults = stdmap.NewPendingResults()
		require.Nil(t, err)

		// registers size method of backend for metrics
		err = mempoolCollector.Register(metrics.ResourcePendingResult, node.PendingResults.Size)
		require.Nil(t, err)
	}

	if node.HeaderStorage == nil {
		node.HeaderStorage = storage.NewHeaders(node.Metrics, node.DB)
	}

	if node.PendingChunks == nil {
		node.PendingChunks = match.NewChunks(1000)

		// registers size method of backend for metrics
		err = mempoolCollector.Register(metrics.ResourcePendingChunk, node.PendingChunks.Size)
		require.Nil(t, err)
	}

	if node.ProcessedResultIDs == nil {
		node.ProcessedResultIDs, err = stdmap.NewIdentifiers(1000)
		require.Nil(t, err)

		// registers size method of backend for metrics
		err = mempoolCollector.Register(metrics.ResourceProcessedResultID, node.ProcessedResultIDs.Size)
		require.Nil(t, err)
	}

	if node.BlockIDsCache == nil {
		node.BlockIDsCache, err = stdmap.NewIdentifiers(1000)
		require.Nil(t, err)

		// registers size method of backend for metrics
		err = mempoolCollector.Register(metrics.ResourceCachedBlockID, node.BlockIDsCache.Size)
		require.Nil(t, err)
	}

	if node.PendingReceiptIDsByBlock == nil {
		node.PendingReceiptIDsByBlock, err = stdmap.NewIdentifierMap(1000)
		require.Nil(t, err)

		// registers size method of backend for metrics
		err = mempoolCollector.Register(metrics.ResourcePendingReceiptIDsByBlock, node.PendingReceiptIDsByBlock.Size)
		require.Nil(t, err)
	}

	if node.ReceiptIDsByResult == nil {
		node.ReceiptIDsByResult, err = stdmap.NewIdentifierMap(1000)
		require.Nil(t, err)

		// registers size method of backend for metrics
		err = mempoolCollector.Register(metrics.ResourceReceiptIDsByResult, node.ReceiptIDsByResult.Size)
		require.Nil(t, err)
	}

	if node.VerifierEngine == nil {
		rt := runtime.NewInterpreterRuntime()

		vm := fvm.New(rt)

		vmCtx := fvm.NewContext(
			fvm.WithChain(node.ChainID.Chain()),
			fvm.WithBlocks(node.Blocks),
		)

		chunkVerifier := chunks.NewChunkVerifier(vm, vmCtx)

		node.VerifierEngine, err = verifier.New(node.Log,
			collector,
			node.Tracer,
			node.Net,
			node.State,
			node.Me,
			chunkVerifier)
		require.Nil(t, err)
	}

	if node.MatchEngine == nil {
		node.MatchEngine, err = match.New(node.Log,
			collector,
			node.Tracer,
			node.Net,
			node.Me,
			node.PendingResults,
			node.VerifierEngine,
			assigner,
			node.State,
			node.PendingChunks,
			node.HeaderStorage,
			requestInterval,
			int(failureThreshold))
		require.Nil(t, err)
	}

	if node.FinderEngine == nil {
		node.FinderEngine, err = finder.New(node.Log,
			collector,
			node.Tracer,
			node.Net,
			node.Me,
			node.MatchEngine,
			node.CachedReceipts,
			node.PendingReceipts,
			node.ReadyReceipts,
			node.Headers,
			node.ProcessedResultIDs,
			node.PendingReceiptIDsByBlock,
			node.ReceiptIDsByResult,
			node.BlockIDsCache,
			processInterval)
		require.Nil(t, err)
	}

	return node
}
