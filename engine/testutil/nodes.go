package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	mockhotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/collection/epochmgr"
	"github.com/onflow/flow-go/engine/collection/epochmgr/factories"
	collectioningest "github.com/onflow/flow-go/engine/collection/ingest"
	"github.com/onflow/flow-go/engine/collection/pusher"
	"github.com/onflow/flow-go/engine/common/follower"
	"github.com/onflow/flow-go/engine/common/provider"
	"github.com/onflow/flow-go/engine/common/requester"
	"github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/engine/consensus/approvals/tracker"
	consensusingest "github.com/onflow/flow-go/engine/consensus/ingestion"
	"github.com/onflow/flow-go/engine/consensus/matching"
	"github.com/onflow/flow-go/engine/consensus/sealing"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/ingestion"
	executionprovider "github.com/onflow/flow-go/engine/execution/provider"
	executionState "github.com/onflow/flow-go/engine/execution/state"
	bootstrapexec "github.com/onflow/flow-go/engine/execution/state/bootstrap"
	testmock "github.com/onflow/flow-go/engine/testutil/mock"
	verificationassigner "github.com/onflow/flow-go/engine/verification/assigner"
	"github.com/onflow/flow-go/engine/verification/assigner/blockconsumer"
	"github.com/onflow/flow-go/engine/verification/fetcher"
	"github.com/onflow/flow-go/engine/verification/fetcher/chunkconsumer"
	vereq "github.com/onflow/flow-go/engine/verification/requester"
	"github.com/onflow/flow-go/engine/verification/verifier"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	completeLedger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	"github.com/onflow/flow-go/module/chunks"
	confinalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/module/mempool"
	consensusMempools "github.com/onflow/flow-go/module/mempool/consensus"
	"github.com/onflow/flow-go/module/mempool/epochs"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	state_synchronization "github.com/onflow/flow-go/module/state_synchronization/mock"
	chainsync "github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/module/validation"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/state/protocol"
	badgerstate "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/state/protocol/events/gadgets"
	"github.com/onflow/flow-go/state/protocol/util"
	storage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

// GenericNodeFromParticipants is a test helper that creates and returns a generic node.
// The generic node's state is generated from the given participants, resulting in a
// root state snapshot.
//
// CAUTION: Please use GenericNode instead for most use-cases so that multiple nodes
// may share the same root state snapshot.
func GenericNodeFromParticipants(t testing.TB, hub *stub.Hub, identity *flow.Identity, participants []*flow.Identity, chainID flow.ChainID,
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
	tracer, err := trace.NewTracer(log, "test", "test", trace.SensitivityCaptureAll)
	require.NoError(t, err)
	metrics := metrics.NewNoopCollector()

	// creates state fixture and bootstrap it.
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	stateFixture := CompleteStateFixture(t, metrics, tracer, rootSnapshot)

	require.NoError(t, err)
	for _, option := range options {
		option(stateFixture.State)
	}

	return GenericNodeWithStateFixture(t, stateFixture, hub, identity, log, metrics, tracer, chainID)
}

// GenericNode returns a generic test node, containing components shared across
// all node roles. The generic node is used as the core data structure to create
// other types of flow nodes.
func GenericNode(
	t testing.TB,
	hub *stub.Hub,
	identity *flow.Identity,
	root protocol.Snapshot,
) testmock.GenericNode {

	log := unittest.Logger().With().
		Hex("node_id", identity.NodeID[:]).
		Str("role", identity.Role.String()).
		Logger()
	metrics := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()
	stateFixture := CompleteStateFixture(t, metrics, tracer, root)

	head, err := root.Head()
	require.NoError(t, err)
	chainID := head.ChainID

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

	me := LocalFixture(t, identity)
	net, err := stub.NewNetwork(stateFixture.State, me, hub)
	require.NoError(t, err)

	parentCtx, cancel := context.WithCancel(context.Background())
	ctx, _ := irrecoverable.WithSignaler(parentCtx)

	return testmock.GenericNode{
		Ctx:            ctx,
		Cancel:         cancel,
		Log:            log,
		Metrics:        metrics,
		Tracer:         tracer,
		PublicDB:       stateFixture.PublicDB,
		SecretsDB:      stateFixture.SecretsDB,
		State:          stateFixture.State,
		Headers:        stateFixture.Storage.Headers,
		Guarantees:     stateFixture.Storage.Guarantees,
		Seals:          stateFixture.Storage.Seals,
		Payloads:       stateFixture.Storage.Payloads,
		Blocks:         stateFixture.Storage.Blocks,
		Me:             me,
		Net:            net,
		DBDir:          stateFixture.DBDir,
		ChainID:        chainID,
		ProtocolEvents: stateFixture.ProtocolEvents,
	}
}

// LocalFixture creates and returns a Local module for given identity.
func LocalFixture(t testing.TB, identity *flow.Identity) module.Local {

	// Generates test signing oracle for the nodes
	// Disclaimer: it should not be used for practical applications
	//
	// uses identity of node as its seed
	seed, err := json.Marshal(identity)
	require.NoError(t, err)
	// creates signing key of the node
	sk, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed[:64])
	require.NoError(t, err)

	// sets staking public key of the node
	identity.StakingPubKey = sk.PublicKey()

	me, err := local.New(identity, sk)
	require.NoError(t, err)

	return me
}

// CompleteStateFixture is a test helper that creates, bootstraps, and returns a StateFixture for sake of unit testing.
func CompleteStateFixture(
	t testing.TB,
	metric *metrics.NoopCollector,
	tracer module.Tracer,
	rootSnapshot protocol.Snapshot,
) *testmock.StateFixture {

	dataDir := unittest.TempDir(t)
	publicDBDir := filepath.Join(dataDir, "protocol")
	secretsDBDir := filepath.Join(dataDir, "secrets")
	db := unittest.TypedBadgerDB(t, publicDBDir, storage.InitPublic)
	s := storage.InitAll(metric, db)
	secretsDB := unittest.TypedBadgerDB(t, secretsDBDir, storage.InitSecret)
	consumer := events.NewDistributor()

	state, err := badgerstate.Bootstrap(metric, db, s.Headers, s.Seals, s.Results, s.Blocks, s.Setups, s.EpochCommits, s.Statuses, rootSnapshot)
	require.NoError(t, err)

	mutableState, err := badgerstate.NewFullConsensusState(state, s.Index, s.Payloads, tracer, consumer,
		util.MockBlockTimer(), util.MockReceiptValidator(), util.MockSealValidator(s.Seals))
	require.NoError(t, err)

	return &testmock.StateFixture{
		PublicDB:       db,
		SecretsDB:      secretsDB,
		Storage:        s,
		DBDir:          dataDir,
		ProtocolEvents: consumer,
		State:          mutableState,
	}
}

// CollectionNode returns a mock collection node.
func CollectionNode(t *testing.T, hub *stub.Hub, identity bootstrap.NodeInfo, rootSnapshot protocol.Snapshot) testmock.CollectionNode {

	node := GenericNode(t, hub, identity.Identity(), rootSnapshot)
	privKeys, err := identity.PrivateKeys()
	require.NoError(t, err)
	node.Me, err = local.New(identity.Identity(), privKeys.StakingKey)
	require.NoError(t, err)

	pools := epochs.NewTransactionPools(func() mempool.Transactions { return herocache.NewTransactions(1000, node.Log) })
	transactions := storage.NewTransactions(node.Metrics, node.PublicDB)
	collections := storage.NewCollections(node.PublicDB, transactions)
	clusterPayloads := storage.NewClusterPayloads(node.Metrics, node.PublicDB)

	ingestionEngine, err := collectioningest.New(node.Log, node.Net, node.State, node.Metrics, node.Metrics, node.Metrics, node.Me, node.ChainID.Chain(), pools, collectioningest.DefaultConfig())
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

	clusterStateFactory, err := factories.NewClusterStateFactory(
		node.PublicDB,
		node.Metrics,
		node.Tracer,
	)
	require.NoError(t, err)

	builderFactory, err := factories.NewBuilderFactory(
		node.PublicDB,
		node.Headers,
		node.Tracer,
		node.Metrics,
		pusherEngine,
	)
	require.NoError(t, err)

	proposalFactory, err := factories.NewProposalEngineFactory(
		node.Log,
		node.Net,
		node.Me,
		node.Metrics, node.Metrics, node.Metrics,
		node.State,
		transactions,
	)
	require.NoError(t, err)

	syncFactory, err := factories.NewSyncEngineFactory(
		node.Log,
		node.Metrics,
		node.Net,
		node.Me,
		chainsync.DefaultConfig(),
	)
	require.NoError(t, err)

	createMetrics := func(chainID flow.ChainID) module.HotstuffMetrics {
		return metrics.NewNoopCollector()
	}
	hotstuffFactory, err := factories.NewHotStuffFactory(
		node.Log,
		node.Me,
		node.PublicDB,
		node.State,
		createMetrics,
		consensus.WithInitialTimeout(time.Second*2),
	)
	require.NoError(t, err)

	factory := factories.NewEpochComponentsFactory(
		node.Me,
		pools,
		builderFactory,
		clusterStateFactory,
		hotstuffFactory,
		proposalFactory,
		syncFactory,
	)

	rootQCVoter := new(mockmodule.ClusterRootQCVoter)
	rootQCVoter.On("Vote", mock.Anything, mock.Anything).Return(nil)

	heights := gadgets.NewHeights()
	node.ProtocolEvents.AddConsumer(heights)

	epochManager, err := epochmgr.New(
		node.Log,
		node.Me,
		node.State,
		pools,
		rootQCVoter,
		factory,
		heights,
	)
	require.NoError(t, err)

	node.ProtocolEvents.AddConsumer(epochManager)

	return testmock.CollectionNode{
		GenericNode:        node,
		Collections:        collections,
		Transactions:       transactions,
		ClusterPayloads:    clusterPayloads,
		IngestionEngine:    ingestionEngine,
		PusherEngine:       pusherEngine,
		ProviderEngine:     providerEngine,
		EpochManagerEngine: epochManager,
	}
}

func ConsensusNode(t *testing.T, hub *stub.Hub, identity *flow.Identity, identities []*flow.Identity, chainID flow.ChainID) testmock.ConsensusNode {

	node := GenericNodeFromParticipants(t, hub, identity, identities, chainID)

	resultsDB := storage.NewExecutionResults(node.Metrics, node.PublicDB)
	receiptsDB := storage.NewExecutionReceipts(node.Metrics, node.PublicDB, resultsDB, storage.DefaultCacheSize)

	guarantees, err := stdmap.NewGuarantees(1000)
	require.NoError(t, err)

	receipts := consensusMempools.NewExecutionTree()

	seals := stdmap.NewIncorporatedResultSeals(1000)
	pendingReceipts := stdmap.NewPendingReceipts(node.Headers, 1000)

	ingestionCore := consensusingest.NewCore(node.Log, node.Tracer, node.Metrics, node.State,
		node.Headers, guarantees)
	// receive collections
	ingestionEngine, err := consensusingest.New(node.Log, node.Metrics, node.Net, node.Me, ingestionCore)
	require.Nil(t, err)

	// request receipts from execution nodes
	receiptRequester, err := requester.New(node.Log, node.Metrics, node.Net, node.Me, node.State, engine.RequestReceiptsByBlockID, filter.Any, func() flow.Entity { return &flow.ExecutionReceipt{} })
	require.Nil(t, err)

	assigner, err := chunks.NewChunkAssigner(chunks.DefaultChunkAssignmentAlpha, node.State)
	require.Nil(t, err)

	receiptValidator := validation.NewReceiptValidator(node.State, node.Headers, node.Index, resultsDB, node.Seals)

	sealingConfig := sealing.DefaultConfig()

	sealingEngine, err := sealing.NewEngine(
		node.Log,
		node.Tracer,
		node.Metrics,
		node.Metrics,
		node.Metrics,
		&tracker.NoopSealingTracker{},
		node.Net,
		node.Me,
		node.Headers,
		node.Payloads,
		resultsDB,
		node.Index,
		node.State,
		node.Seals,
		assigner,
		seals,
		sealingConfig)
	require.NoError(t, err)

	matchingConfig := matching.DefaultConfig()

	matchingCore := matching.NewCore(
		node.Log,
		node.Tracer,
		node.Metrics,
		node.Metrics,
		node.State,
		node.Headers,
		receiptsDB,
		receipts,
		pendingReceipts,
		seals,
		receiptValidator,
		receiptRequester,
		matchingConfig)

	matchingEngine, err := matching.NewEngine(
		node.Log,
		node.Net,
		node.Me,
		node.Metrics,
		node.Metrics,
		node.State,
		receiptsDB,
		node.Index,
		matchingCore,
	)
	require.NoError(t, err)

	return testmock.ConsensusNode{
		GenericNode:     node,
		Guarantees:      guarantees,
		Receipts:        receipts,
		Seals:           seals,
		IngestionEngine: ingestionEngine,
		SealingEngine:   sealingEngine,
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

type CheckerMock struct {
	notifications.NoopConsumer // satisfy the FinalizationConsumer interface
}

func ExecutionNode(t *testing.T, hub *stub.Hub, identity *flow.Identity, identities []*flow.Identity, syncThreshold int, chainID flow.ChainID) testmock.ExecutionNode {
	node := GenericNodeFromParticipants(t, hub, identity, identities, chainID)

	transactionsStorage := storage.NewTransactions(node.Metrics, node.PublicDB)
	collectionsStorage := storage.NewCollections(node.PublicDB, transactionsStorage)
	eventsStorage := storage.NewEvents(node.Metrics, node.PublicDB)
	serviceEventsStorage := storage.NewServiceEvents(node.Metrics, node.PublicDB)
	txResultStorage := storage.NewTransactionResults(node.Metrics, node.PublicDB, storage.DefaultCacheSize)
	commitsStorage := storage.NewCommits(node.Metrics, node.PublicDB)
	chunkDataPackStorage := storage.NewChunkDataPacks(node.Metrics, node.PublicDB, collectionsStorage, 100)
	results := storage.NewExecutionResults(node.Metrics, node.PublicDB)
	receipts := storage.NewExecutionReceipts(node.Metrics, node.PublicDB, results, storage.DefaultCacheSize)
	myReceipts := storage.NewMyExecutionReceipts(node.Metrics, node.PublicDB, receipts)
	checkAuthorizedAtBlock := func(blockID flow.Identifier) (bool, error) {
		return protocol.IsNodeAuthorizedAt(node.State.AtBlockID(blockID), node.Me.NodeID())
	}

	protoState, ok := node.State.(*badgerstate.MutableState)
	require.True(t, ok)

	followerState, err := badgerstate.NewFollowerState(protoState.State, node.Index, node.Payloads, node.Tracer,
		node.ProtocolEvents, blocktimer.DefaultBlockTimer)
	require.NoError(t, err)

	pendingBlocks := buffer.NewPendingBlocks() // for following main chain consensus

	dbDir := unittest.TempDir(t)

	metricsCollector := &metrics.NoopCollector{}

	diskWal, err := wal.NewDiskWAL(node.Log.With().Str("subcomponent", "wal").Logger(), nil, metricsCollector, dbDir, 100, pathfinder.PathByteSize, wal.SegmentSize)
	require.NoError(t, err)

	ls, err := completeLedger.NewLedger(diskWal, 100, metricsCollector, node.Log.With().Str("compontent", "ledger").Logger(), completeLedger.DefaultPathFinderVersion)
	require.NoError(t, err)

	genesisHead, err := node.State.Final().Head()
	require.NoError(t, err)

	bootstrapper := bootstrapexec.NewBootstrapper(node.Log)
	commit, err := bootstrapper.BootstrapLedger(
		ls,
		unittest.ServiceAccountPublicKey,
		node.ChainID.Chain(),
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply))
	require.NoError(t, err)

	err = bootstrapper.BootstrapExecutionDatabase(node.PublicDB, commit, genesisHead)
	require.NoError(t, err)

	execState := executionState.NewExecutionState(
		ls, commitsStorage, node.Blocks, node.Headers, collectionsStorage, chunkDataPackStorage, results, receipts, myReceipts, eventsStorage, serviceEventsStorage, txResultStorage, node.PublicDB, node.Tracer,
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
		node.Log, node.Tracer, node.Net, node.State, node.Me, execState, metrics, checkAuthorizedAtBlock, 10, 10,
	)
	require.NoError(t, err)

	rt := fvm.NewInterpreterRuntime()

	vm := fvm.NewVirtualMachine(rt)

	blockFinder := fvm.NewBlockFinder(node.Headers)

	vmCtx := fvm.NewContext(
		node.Log,
		fvm.WithChain(node.ChainID.Chain()),
		fvm.WithBlocks(blockFinder),
	)
	committer := committer.NewLedgerViewCommitter(ls, node.Tracer)

	eds := new(state_synchronization.ExecutionDataService)
	eds.On("Add", mock.Anything, mock.Anything).Return(flow.ZeroID, nil, nil)

	edCache := new(state_synchronization.ExecutionDataCIDCache)
	edCache.On("Insert", mock.AnythingOfType("*flow.Header"), mock.AnythingOfType("BlobTree"))

	computationEngine, err := computation.New(
		node.Log,
		node.Metrics,
		node.Tracer,
		node.Me,
		node.State,
		vm,
		vmCtx,
		computation.DefaultProgramsCacheSize,
		committer,
		computation.DefaultScriptLogThreshold,
		nil,
		eds,
		edCache,
	)
	require.NoError(t, err)

	computation := &testmock.ComputerWrap{
		Manager: computationEngine,
	}

	syncCore, err := chainsync.New(node.Log, chainsync.DefaultConfig())
	require.NoError(t, err)

	deltas, err := ingestion.NewDeltas(1000)
	require.NoError(t, err)

	finalizationDistributor := pubsub.NewFinalizationDistributor()

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
		checkAuthorizedAtBlock,
		false,
	)
	require.NoError(t, err)
	requestEngine.WithHandle(ingestionEngine.OnCollection)

	node.ProtocolEvents.AddConsumer(ingestionEngine)

	followerCore, finalizer := createFollowerCore(t, &node, followerState, finalizationDistributor, rootHead, rootQC)

	// initialize cleaner for DB
	cleaner := storage.NewCleaner(node.Log, node.PublicDB, node.Metrics, flow.DefaultValueLogGCFrequency)

	followerEng, err := follower.New(node.Log, node.Net, node.Me, node.Metrics, node.Metrics, cleaner,
		node.Headers, node.Payloads, followerState, pendingBlocks, followerCore, syncCore, node.Tracer)
	require.NoError(t, err)

	finalizedHeader, err := synchronization.NewFinalizedHeaderCache(node.Log, node.State, finalizationDistributor)
	require.NoError(t, err)

	idCache, err := p2p.NewProtocolStateIDCache(node.Log, node.State, events.NewDistributor())
	require.NoError(t, err, "could not create finalized snapshot cache")
	syncEngine, err := synchronization.New(
		node.Log,
		node.Metrics,
		node.Net,
		node.Me,
		node.Blocks,
		followerEng,
		syncCore,
		finalizedHeader,
		id.NewIdentityFilterIdentifierProvider(
			filter.And(
				filter.HasRole(flow.RoleConsensus),
				filter.Not(filter.HasNodeID(node.Me.NodeID())),
			),
			idCache,
		),
		synchronization.WithPollInterval(time.Duration(0)),
	)
	require.NoError(t, err)

	return testmock.ExecutionNode{
		GenericNode:         node,
		MutableState:        followerState,
		IngestionEngine:     ingestionEngine,
		FollowerEngine:      followerEng,
		SyncEngine:          syncEngine,
		ExecutionEngine:     computation,
		RequestEngine:       requestEngine,
		ReceiptsEngine:      pusherEngine,
		BadgerDB:            node.PublicDB,
		VM:                  vm,
		ExecutionState:      execState,
		Ledger:              ls,
		LevelDbDir:          dbDir,
		Collections:         collectionsStorage,
		Finalizer:           finalizer,
		MyExecutionReceipts: myReceipts,
		DiskWAL:             diskWal,
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
	verifier.On("VerifyVote", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	verifier.On("VerifyQC", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	finalizer := confinalizer.NewFinalizer(node.PublicDB, node.Headers, followerState, trace.NewNoopTracer())

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

func WithChunkConsumer(chunkConsumer *chunkconsumer.ChunkConsumer) VerificationOpt {
	return func(node *testmock.VerificationNode) {
		node.ChunkConsumer = chunkConsumer
	}
}

func WithGenericNode(genericNode *testmock.GenericNode) VerificationOpt {
	return func(node *testmock.VerificationNode) {
		node.GenericNode = genericNode
	}
}

// VerificationNode creates a verification node with all functional engines and actual modules for purpose of
// (integration) testing.
func VerificationNode(t testing.TB,
	hub *stub.Hub,
	verIdentity *flow.Identity, // identity of this verification node.
	participants flow.IdentityList, // identity of all nodes in system including this verification node.
	assigner module.ChunkAssigner,
	chunksLimit uint,
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
		gn := GenericNodeFromParticipants(t, hub, verIdentity, participants, chainID)
		node.GenericNode = &gn
	}

	if node.ChunkStatuses == nil {
		node.ChunkStatuses = stdmap.NewChunkStatuses(chunksLimit)
		err = mempoolCollector.Register(metrics.ResourceChunkStatus, node.ChunkStatuses.Size)
		require.Nil(t, err)
	}

	if node.ChunkRequests == nil {
		node.ChunkRequests = stdmap.NewChunkRequests(chunksLimit)
		err = mempoolCollector.Register(metrics.ResourceChunkRequest, node.ChunkRequests.Size)
		require.NoError(t, err)
	}

	if node.Results == nil {
		results := storage.NewExecutionResults(node.Metrics, node.PublicDB)
		node.Results = results
		node.Receipts = storage.NewExecutionReceipts(node.Metrics, node.PublicDB, results, storage.DefaultCacheSize)
	}

	if node.ProcessedChunkIndex == nil {
		node.ProcessedChunkIndex = storage.NewConsumerProgress(node.PublicDB, module.ConsumeProgressVerificationChunkIndex)
	}

	if node.ChunksQueue == nil {
		node.ChunksQueue = storage.NewChunkQueue(node.PublicDB)
		ok, err := node.ChunksQueue.Init(chunkconsumer.DefaultJobIndex)
		require.NoError(t, err)
		require.True(t, ok)
	}

	if node.ProcessedBlockHeight == nil {
		node.ProcessedBlockHeight = storage.NewConsumerProgress(node.PublicDB, module.ConsumeProgressVerificationBlockHeight)
	}

	if node.VerifierEngine == nil {
		rt := fvm.NewInterpreterRuntime()

		vm := fvm.NewVirtualMachine(rt)

		blockFinder := fvm.NewBlockFinder(node.Headers)

		vmCtx := fvm.NewContext(
			node.Log,
			fvm.WithChain(node.ChainID.Chain()),
			fvm.WithBlocks(blockFinder),
		)

		chunkVerifier := chunks.NewChunkVerifier(vm, vmCtx, node.Log)

		approvalStorage := storage.NewResultApprovals(node.Metrics, node.PublicDB)

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

	if node.RequesterEngine == nil {
		node.RequesterEngine, err = vereq.New(node.Log,
			node.State,
			node.Net,
			node.Tracer,
			collector,
			node.ChunkRequests,
			vereq.DefaultRequestInterval,
			// requests are only qualified if their retryAfter is elapsed.
			vereq.RetryAfterQualifier,
			// exponential backoff with multiplier of 2, minimum interval of a second, and
			// maximum interval of an hour.
			mempool.ExponentialUpdater(
				vereq.DefaultBackoffMultiplier,
				vereq.DefaultBackoffMaxInterval,
				vereq.DefaultBackoffMinInterval),
			vereq.DefaultRequestTargets)

		require.NoError(t, err)
	}

	if node.FetcherEngine == nil {
		node.FetcherEngine = fetcher.New(node.Log,
			collector,
			node.Tracer,
			node.VerifierEngine,
			node.State,
			node.ChunkStatuses,
			node.Headers,
			node.Blocks,
			node.Results,
			node.Receipts,
			node.RequesterEngine,
		)
	}

	if node.ChunkConsumer == nil {
		node.ChunkConsumer = chunkconsumer.NewChunkConsumer(node.Log,
			collector,
			node.ProcessedChunkIndex,
			node.ChunksQueue,
			node.FetcherEngine,
			chunkconsumer.DefaultChunkWorkers) // defaults number of workers to 3.
		err = mempoolCollector.Register(metrics.ResourceChunkConsumer, node.ChunkConsumer.Size)
		require.NoError(t, err)
	}

	if node.AssignerEngine == nil {
		node.AssignerEngine = verificationassigner.New(node.Log,
			collector,
			node.Tracer,
			node.Me,
			node.State,
			assigner,
			node.ChunksQueue,
			node.ChunkConsumer)
	}

	if node.BlockConsumer == nil {
		node.BlockConsumer, _, err = blockconsumer.NewBlockConsumer(node.Log,
			collector,
			node.ProcessedBlockHeight,
			node.Blocks,
			node.State,
			node.AssignerEngine,
			blockconsumer.DefaultBlockWorkers)
		require.NoError(t, err)

		err = mempoolCollector.Register(metrics.ResourceBlockConsumer, node.BlockConsumer.Size)
		require.NoError(t, err)
	}

	return node
}
