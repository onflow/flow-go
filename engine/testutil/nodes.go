package testutil

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	mockhotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	collectioningest "github.com/onflow/flow-go/engine/collection/ingest"
	"github.com/onflow/flow-go/engine/collection/pusher"
	"github.com/onflow/flow-go/engine/common/follower"
	"github.com/onflow/flow-go/engine/common/provider"
	"github.com/onflow/flow-go/engine/common/requester"
	"github.com/onflow/flow-go/engine/common/synchronization"
	consensusingest "github.com/onflow/flow-go/engine/consensus/ingestion"
	"github.com/onflow/flow-go/engine/consensus/matching"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/ingestion"
	executionprovider "github.com/onflow/flow-go/engine/execution/provider"
	executionState "github.com/onflow/flow-go/engine/execution/state"
	bootstrapexec "github.com/onflow/flow-go/engine/execution/state/bootstrap"
	testmock "github.com/onflow/flow-go/engine/testutil/mock"
	"github.com/onflow/flow-go/engine/verification/finder"
	"github.com/onflow/flow-go/engine/verification/match"
	"github.com/onflow/flow-go/engine/verification/verifier"
	"github.com/onflow/flow-go/fvm"
	completeLedger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	"github.com/onflow/flow-go/module/chunks"
	confinalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/module/mempool"
	consensusMempools "github.com/onflow/flow-go/module/mempool/consensus"
	"github.com/onflow/flow-go/module/mempool/epochs"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/signature"
	chainsync "github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/module/validation"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/state/protocol"
	badgerstate "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/state/protocol/util"
	storage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

// GenericNode is a test helper that creates and returns a generic node.
// The generic node is used as the core data structure to create other types of flow nodes.
func GenericNode(t testing.TB, hub *stub.Hub, identity *flow.Identity, participants []*flow.Identity, chainID flow.ChainID,
	options ...func(protocol.State)) testmock.GenericNode {
	var i int
	var participant *flow.Identity
	for i, participant = range participants {
		if identity.NodeID == participant.NodeID {
			break
		}
	}

	// creates logger, metrics collector and tracer.
	log := unittest.Logger().With().Int("index", i).Hex("node_id", identity.NodeID[:]).Str("role", identity.Role.String()).Logger()
	tracer, err := trace.NewTracer(log, "test")
	require.NoError(t, err)
	metrics := metrics.NewNoopCollector()

	// creates state fixture and bootstrap it.
	stateFixture := CompleteStateFixture(t, log, metrics, tracer, participants)

	require.NoError(t, err)
	for _, option := range options {
		option(stateFixture.State)
	}

	return GenericNodeWithStateFixture(t, stateFixture, hub, identity, log, metrics, tracer, chainID)
}

// GenericNodeWithStateFixture is a test helper that creates a generic node with specified state fixture.
func GenericNodeWithStateFixture(t testing.TB,
	stateFixture *testmock.StateFixture,
	hub *stub.Hub,
	identity *flow.Identity,
	log zerolog.Logger,
	metrics *metrics.NoopCollector,
	tracer module.Tracer,
	chainID flow.ChainID) testmock.GenericNode {

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

	stubnet := stub.NewNetwork(stateFixture.State, me, hub)

	return testmock.GenericNode{
		Log:            log,
		Metrics:        metrics,
		Tracer:         tracer,
		DB:             stateFixture.DB,
		State:          stateFixture.State,
		Headers:        stateFixture.Storage.Headers,
		Guarantees:     stateFixture.Storage.Guarantees,
		Seals:          stateFixture.Storage.Seals,
		Payloads:       stateFixture.Storage.Payloads,
		Blocks:         stateFixture.Storage.Blocks,
		Me:             me,
		Net:            stubnet,
		DBDir:          stateFixture.DBDir,
		ChainID:        chainID,
		ProtocolEvents: stateFixture.ProtocolEvents,
	}
}

// CompleteStateFixture is a test helper that creates, bootstraps, and returns a StateFixture for sake of unit testing.
func CompleteStateFixture(t testing.TB, log zerolog.Logger, metric *metrics.NoopCollector, tracer module.Tracer,
	participants flow.IdentityList) *testmock.StateFixture {
	dbDir := unittest.TempDir(t)
	db := unittest.BadgerDB(t, dbDir)
	s := storage.InitAll(metric, db)
	consumer := events.NewNoop()

	root, result, seal := unittest.BootstrapFixture(participants)
	stateRoot, err := badgerstate.NewStateRoot(root, result, seal, 0)
	require.NoError(t, err)

	state, err := badgerstate.Bootstrap(metric, db, s.Headers, s.Seals, s.Blocks, s.Setups, s.EpochCommits, s.Statuses, stateRoot)
	require.NoError(t, err)

	mutableState, err := badgerstate.NewFullConsensusState(state, s.Index, s.Payloads, tracer, consumer, util.MockReceiptValidator(), util.MockSealValidator(s.Seals))
	require.NoError(t, err)

	return &testmock.StateFixture{
		DB:             db,
		Storage:        s,
		DBDir:          dbDir,
		ProtocolEvents: events.NewDistributor(),
		State:          mutableState,
	}
}

// CollectionNode returns a mock collection node.
func CollectionNode(t *testing.T, hub *stub.Hub, identity *flow.Identity, identities []*flow.Identity, chainID flow.ChainID, options ...func(protocol.State)) testmock.CollectionNode {

	node := GenericNode(t, hub, identity, identities, chainID, options...)

	pools := epochs.NewTransactionPools(func() mempool.Transactions { return stdmap.NewTransactions(1000) })
	transactions := storage.NewTransactions(node.Metrics, node.DB)
	collections := storage.NewCollections(node.DB, transactions)

	ingestionEngine, err := collectioningest.New(node.Log, node.Net, node.State, node.Metrics, node.Metrics, node.Me, chainID.Chain(), pools, collectioningest.DefaultConfig())
	require.NoError(t, err)

	selector := filter.HasRole(flow.RoleAccess, flow.RoleVerification)
	retrieve := func(collID flow.Identifier) (flow.Entity, error) {
		coll, err := collections.ByID(collID)
		return coll, err
	}
	providerEngine, err := provider.New(node.Log, node.Metrics, node.Net, node.Me, node.State, engine.ProvideCollections, selector, retrieve)
	require.NoError(t, err)

	pusherEngine, err := pusher.New(node.Log, node.Net, node.State, node.Metrics, node.Metrics, node.Me, collections, transactions)
	require.NoError(t, err)

	return testmock.CollectionNode{
		GenericNode:     node,
		Collections:     collections,
		Transactions:    transactions,
		IngestionEngine: ingestionEngine,
		PusherEngine:    pusherEngine,
		ProviderEngine:  providerEngine,
	}
}

// CollectionNodes returns n collection nodes connected to the given hub.
func CollectionNodes(t *testing.T, hub *stub.Hub, nNodes int, chainID flow.ChainID, options ...func(protocol.State)) []testmock.CollectionNode {
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

	resultsDB := storage.NewExecutionResults(node.Metrics, node.DB)
	receiptsDB := storage.NewExecutionReceipts(node.Metrics, node.DB, resultsDB)

	guarantees, err := stdmap.NewGuarantees(1000)
	require.NoError(t, err)

	results, err := stdmap.NewIncorporatedResults(1000)
	require.NoError(t, err)

	receipts := consensusMempools.NewExecutionTree()

	approvals, err := stdmap.NewApprovals(1000)
	require.NoError(t, err)

	seals := stdmap.NewIncorporatedResultSeals(stdmap.WithLimit(1000))
	pendingReceipts := stdmap.NewPendingReceipts(1000)

	// receive collections
	ingestionEngine, err := consensusingest.New(node.Log, node.Tracer, node.Metrics, node.Metrics, node.Metrics, node.Net, node.State,
		node.Headers, node.Me, guarantees)
	require.Nil(t, err)

	// request receipts from execution nodes
	receiptRequester, err := requester.New(node.Log, node.Metrics, node.Net, node.Me, node.State, engine.RequestReceiptsByBlockID, filter.Any, func() flow.Entity { return &flow.ExecutionReceipt{} })
	require.Nil(t, err)

	assigner, err := chunks.NewChunkAssigner(chunks.DefaultChunkAssignmentAlpha, node.State)
	require.Nil(t, err)

	signatureVerifier := signature.NewAggregationVerifier(encoding.ExecutionReceiptTag)
	validator := validation.NewReceiptValidator(node.State, node.Index, resultsDB, signatureVerifier)

	matchingEngine, err := matching.New(
		node.Log,
		node.Metrics,
		node.Tracer,
		node.Metrics,
		node.Metrics,
		node.Net,
		node.State,
		node.Me,
		receiptRequester,
		receiptsDB,
		node.Headers,
		node.Index,
		results,
		receipts,
		approvals,
		seals,
		pendingReceipts,
		assigner,
		validator,
		validation.DefaultRequiredApprovalsForSealValidation,
		matching.DefaultEmergencySealingActive)
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

func ExecutionNode(t *testing.T, hub *stub.Hub, identity *flow.Identity, identities []*flow.Identity, syncThreshold int, chainID flow.ChainID) testmock.ExecutionNode {
	node := GenericNode(t, hub, identity, identities, chainID)

	transactionsStorage := storage.NewTransactions(node.Metrics, node.DB)
	collectionsStorage := storage.NewCollections(node.DB, transactionsStorage)
	eventsStorage := storage.NewEvents(node.DB)
	serviceEventsStorage := storage.NewServiceEvents(node.DB)
	txResultStorage := storage.NewTransactionResults(node.DB)
	commitsStorage := storage.NewCommits(node.Metrics, node.DB)
	chunkDataPackStorage := storage.NewChunkDataPacks(node.DB)
	results := storage.NewExecutionResults(node.Metrics, node.DB)
	receipts := storage.NewExecutionReceipts(node.Metrics, node.DB, results)

	protoState, ok := node.State.(*badgerstate.MutableState)
	require.True(t, ok)

	followerState, err := badgerstate.NewFollowerState(protoState.State, node.Index, node.Payloads, node.Tracer, node.ProtocolEvents)
	require.NoError(t, err)

	pendingBlocks := buffer.NewPendingBlocks() // for following main chain consensus

	dbDir := unittest.TempDir(t)

	metricsCollector := &metrics.NoopCollector{}
	ls, err := completeLedger.NewLedger(dbDir, 100, metricsCollector, node.Log.With().Str("compontent", "ledger").Logger(), nil, completeLedger.DefaultPathFinderVersion)
	require.NoError(t, err)

	genesisHead, err := node.State.Final().Head()
	require.NoError(t, err)

	bootstrapper := bootstrapexec.NewBootstrapper(node.Log)
	commit, err := bootstrapper.BootstrapLedger(ls, unittest.ServiceAccountPublicKey, unittest.GenesisTokenSupply, node.ChainID.Chain())
	require.NoError(t, err)

	err = bootstrapper.BootstrapExecutionDatabase(node.DB, commit, genesisHead)
	require.NoError(t, err)

	execState := executionState.NewExecutionState(
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

	blockFinder := fvm.NewBlockFinder(node.Headers)

	vmCtx := fvm.NewContext(
		node.Log,
		fvm.WithChain(node.ChainID.Chain()),
		fvm.WithBlocks(blockFinder),
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

	computation := &testmock.ComputerWrap{
		Manager: computationEngine,
	}

	syncCore, err := chainsync.New(node.Log, chainsync.DefaultConfig())
	require.NoError(t, err)

	deltas, err := ingestion.NewDeltas(1000)
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
		serviceEventsStorage,
		txResultStorage,
		computation,
		pusherEngine,
		execState,
		node.Metrics,
		node.Tracer,
		false,
		filter.Any,
		deltas,
		syncThreshold,
		false,
	)
	require.NoError(t, err)
	requestEngine.WithHandle(ingestionEngine.OnCollection)

	node.ProtocolEvents.AddConsumer(ingestionEngine)

	followerCore, finalizer := createFollowerCore(t, &node, followerState, ingestionEngine, rootHead, rootQC)

	// initialize cleaner for DB
	cleaner := storage.NewCleaner(node.Log, node.DB, node.Metrics, flow.DefaultValueLogGCFrequency)

	followerEng, err := follower.New(node.Log, node.Net, node.Me, node.Metrics, node.Metrics, cleaner,
		node.Headers, node.Payloads, followerState, pendingBlocks, followerCore, syncCore)
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
		MutableState:    followerState,
		IngestionEngine: ingestionEngine,
		FollowerEngine:  followerEng,
		SyncEngine:      syncEngine,
		ExecutionEngine: computation,
		RequestEngine:   requestEngine,
		ReceiptsEngine:  pusherEngine,
		BadgerDB:        node.DB,
		VM:              vm,
		ExecutionState:  execState,
		Ledger:          ls,
		LevelDbDir:      dbDir,
		Collections:     collectionsStorage,
		Finalizer:       finalizer,
	}
}

func getRoot(t *testing.T, node *testmock.GenericNode) (*flow.Header, *flow.QuorumCertificate) {
	rootHead, err := node.State.Params().Root()
	require.NoError(t, err)

	signers, err := node.State.AtHeight(0).Identities(filter.HasRole(flow.RoleConsensus))
	require.NoError(t, err)

	signerIDs := signers.NodeIDs()

	rootQC := &flow.QuorumCertificate{
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

func (s *RoundRobinLeaderSelection) DKG(blockID flow.Identifier) (hotstuff.DKG, error) {
	return nil, fmt.Errorf("error")
}

func createFollowerCore(t *testing.T, node *testmock.GenericNode, followerState *badgerstate.FollowerState, notifier hotstuff.FinalizationConsumer,
	rootHead *flow.Header, rootQC *flow.QuorumCertificate) (module.HotStuffFollower, *confinalizer.Finalizer) {

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

	finalizer := confinalizer.NewFinalizer(node.DB, node.Headers, followerState)

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
	return followerCore, finalizer
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

func WithGenericNode(genericNode *testmock.GenericNode) VerificationOpt {
	return func(node *testmock.VerificationNode) {
		node.GenericNode = genericNode
	}
}

func VerificationNode(t testing.TB,
	hub *stub.Hub,
	identity *flow.Identity,
	identities []*flow.Identity,
	assigner module.ChunkAssigner,
	requestInterval time.Duration,
	processInterval time.Duration,
	receiptsLimit uint,
	chunksLimit uint,
	failureThreshold uint,
	chainID flow.ChainID,
	collector module.VerificationMetrics, // used to enable collecting metrics on happy path integration
	mempoolCollector module.MempoolMetrics, // used to enable collecting metrics on happy path integration
	opts ...VerificationOpt) testmock.VerificationNode {

	var err error
	var node testmock.VerificationNode

	for _, apply := range opts {
		apply(&node)
	}

	if node.GenericNode == nil {
		gn := GenericNode(t, hub, identity, identities, chainID)
		node.GenericNode = &gn
	}

	if node.CachedReceipts == nil {
		node.CachedReceipts, err = stdmap.NewReceiptDataPacks(receiptsLimit)
		require.Nil(t, err)
		// registers size method of backend for metrics
		err = mempoolCollector.Register(metrics.ResourceCachedReceipt, node.CachedReceipts.Size)
		require.Nil(t, err)
	}

	if node.PendingReceipts == nil {
		node.PendingReceipts, err = stdmap.NewReceiptDataPacks(receiptsLimit)
		require.Nil(t, err)

		// registers size method of backend for metrics
		err = mempoolCollector.Register(metrics.ResourcePendingReceipt, node.PendingReceipts.Size)
		require.Nil(t, err)
	}

	if node.ReadyReceipts == nil {
		node.ReadyReceipts, err = stdmap.NewReceiptDataPacks(receiptsLimit)
		require.Nil(t, err)
		// registers size method of backend for metrics
		err = mempoolCollector.Register(metrics.ResourceReceipt, node.ReadyReceipts.Size)
		require.Nil(t, err)
	}

	if node.PendingResults == nil {
		node.PendingResults = stdmap.NewResultDataPacks(receiptsLimit)
		require.Nil(t, err)

		// registers size method of backend for metrics
		err = mempoolCollector.Register(metrics.ResourcePendingResult, node.PendingResults.Size)
		require.Nil(t, err)
	}

	if node.PendingChunks == nil {
		node.PendingChunks = match.NewChunks(chunksLimit)

		// registers size method of backend for metrics
		err = mempoolCollector.Register(metrics.ResourcePendingChunk, node.PendingChunks.Size)
		require.Nil(t, err)
	}

	if node.ProcessedResultIDs == nil {
		node.ProcessedResultIDs, err = stdmap.NewIdentifiers(receiptsLimit)
		require.Nil(t, err)

		// registers size method of backend for metrics
		err = mempoolCollector.Register(metrics.ResourceProcessedResultID, node.ProcessedResultIDs.Size)
		require.Nil(t, err)
	}

	if node.DiscardedResultIDs == nil {
		node.DiscardedResultIDs, err = stdmap.NewIdentifiers(receiptsLimit)
		require.Nil(t, err)

		// registers size method of backend for metrics
		err = mempoolCollector.Register(metrics.ResourceDiscardedResultID, node.DiscardedResultIDs.Size)
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
		node.PendingReceiptIDsByBlock, err = stdmap.NewIdentifierMap(receiptsLimit)
		require.Nil(t, err)

		// registers size method of backend for metrics
		err = mempoolCollector.Register(metrics.ResourcePendingReceiptIDsByBlock, node.PendingReceiptIDsByBlock.Size)
		require.Nil(t, err)
	}

	if node.ReceiptIDsByResult == nil {
		node.ReceiptIDsByResult, err = stdmap.NewIdentifierMap(receiptsLimit)
		require.Nil(t, err)

		// registers size method of backend for metrics
		err = mempoolCollector.Register(metrics.ResourceReceiptIDsByResult, node.ReceiptIDsByResult.Size)
		require.Nil(t, err)
	}

	if node.ChunkIDsByResult == nil {
		node.ChunkIDsByResult, err = stdmap.NewIdentifierMap(chunksLimit)
		require.Nil(t, err)

		// registers size method of backend for metrics
		err = mempoolCollector.Register(metrics.ResourceChunkIDsByResult, node.ChunkIDsByResult.Size)
		require.Nil(t, err)
	}

	if node.VerifierEngine == nil {
		rt := runtime.NewInterpreterRuntime()

		vm := fvm.New(rt)

		blockFinder := fvm.NewBlockFinder(node.Headers)

		vmCtx := fvm.NewContext(
			node.Log,
			fvm.WithChain(node.ChainID.Chain()),
			fvm.WithBlocks(blockFinder),
		)

		chunkVerifier := chunks.NewChunkVerifier(vm, vmCtx)

		approvalStorage := storage.NewResultApprovals(node.Metrics, node.DB)

		node.VerifierEngine, err = verifier.New(node.Log,
			collector,
			node.Tracer,
			node.Net,
			node.State,
			node.Me,
			chunkVerifier,
			approvalStorage)
		require.Nil(t, err)
	}

	if node.MatchEngine == nil {
		node.MatchEngine, err = match.New(node.Log,
			collector,
			node.Tracer,
			node.Net,
			node.Me,
			node.PendingResults,
			node.ChunkIDsByResult,
			node.VerifierEngine,
			assigner,
			node.State,
			node.PendingChunks,
			node.Headers,
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
			node.State,
			node.MatchEngine,
			node.CachedReceipts,
			node.PendingReceipts,
			node.ReadyReceipts,
			node.Headers,
			node.ProcessedResultIDs,
			node.DiscardedResultIDs,
			node.PendingReceiptIDsByBlock,
			node.ReceiptIDsByResult,
			node.BlockIDsCache,
			processInterval)
		require.Nil(t, err)
	}

	return node
}
