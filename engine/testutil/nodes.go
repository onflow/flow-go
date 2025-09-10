package testutil

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/coreos/go-semver/semver"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/cmd/build"
	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	mockhotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/collection/epochmgr"
	"github.com/onflow/flow-go/engine/collection/epochmgr/factories"
	"github.com/onflow/flow-go/engine/collection/ingest"
	collectioningest "github.com/onflow/flow-go/engine/collection/ingest"
	mockcollection "github.com/onflow/flow-go/engine/collection/mock"
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
	"github.com/onflow/flow-go/engine/execution/computation/query"
	"github.com/onflow/flow-go/engine/execution/ingestion"
	exeFetcher "github.com/onflow/flow-go/engine/execution/ingestion/fetcher"
	"github.com/onflow/flow-go/engine/execution/ingestion/stop"
	"github.com/onflow/flow-go/engine/execution/ingestion/uploader"
	executionprovider "github.com/onflow/flow-go/engine/execution/provider"
	executionState "github.com/onflow/flow-go/engine/execution/state"
	bootstrapexec "github.com/onflow/flow-go/engine/execution/state/bootstrap"
	esbootstrap "github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	testmock "github.com/onflow/flow-go/engine/testutil/mock"
	verificationassigner "github.com/onflow/flow-go/engine/verification/assigner"
	"github.com/onflow/flow-go/engine/verification/assigner/blockconsumer"
	"github.com/onflow/flow-go/engine/verification/fetcher"
	"github.com/onflow/flow-go/engine/verification/fetcher/chunkconsumer"
	vereq "github.com/onflow/flow-go/engine/verification/requester"
	"github.com/onflow/flow-go/engine/verification/verifier"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	completeLedger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/chainsync"
	"github.com/onflow/flow-go/module/chunks"
	"github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	exedataprovider "github.com/onflow/flow-go/module/executiondatasync/provider"
	mocktracker "github.com/onflow/flow-go/module/executiondatasync/tracker/mock"
	"github.com/onflow/flow-go/module/finalizedreader"
	confinalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/module/mempool"
	consensusMempools "github.com/onflow/flow-go/module/mempool/consensus"
	"github.com/onflow/flow-go/module/mempool/epochs"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/signature"
	requesterunit "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/module/updatable_configs"
	"github.com/onflow/flow-go/module/validation"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p/cache"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/state/protocol"
	badgerstate "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/state/protocol/events/gadgets"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/storage"
	storagebadger "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	storagepebble "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

// GenericNodeFromParticipants is a test helper that creates and returns a generic node.
// The generic node's state is generated from the given participants, resulting in a
// root state snapshot.
//
// CAUTION: Please use GenericNode instead for most use-cases so that multiple nodes
// may share the same root state snapshot.
func GenericNodeFromParticipants(t testing.TB, hub *stub.Hub, identity bootstrap.NodeInfo, participants []*flow.Identity, chainID flow.ChainID,
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
	rootSnapshot := unittest.RootSnapshotFixtureWithChainID(participants, chainID)
	stateFixture := CompleteStateFixture(t, log, metrics, tracer, rootSnapshot)

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
	identity bootstrap.NodeInfo,
	root protocol.Snapshot,
) testmock.GenericNode {

	log := unittest.Logger().With().
		Hex("node_id", identity.NodeID[:]).
		Str("role", identity.Role.String()).
		Logger()
	metrics := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()
	stateFixture := CompleteStateFixture(t, log, metrics, tracer, root)

	head, err := root.Head()
	require.NoError(t, err)
	chainID := head.ChainID

	return GenericNodeWithStateFixture(t, stateFixture, hub, identity, log, metrics, tracer, chainID)
}

// GenericNodeWithStateFixture is a test helper that creates a generic node with specified state fixture.
func GenericNodeWithStateFixture(t testing.TB,
	stateFixture *testmock.StateFixture,
	hub *stub.Hub,
	bootstrapInfo bootstrap.NodeInfo,
	log zerolog.Logger,
	metrics *metrics.NoopCollector,
	tracer module.Tracer,
	chainID flow.ChainID) testmock.GenericNode {

	identity := bootstrapInfo.Identity()
	privateKeys, err := bootstrapInfo.PrivateKeys()
	require.NoError(t, err)
	me, err := local.New(identity.IdentitySkeleton, privateKeys.StakingKey)
	require.NoError(t, err)
	net := stub.NewNetwork(t, identity.NodeID, hub)

	parentCtx, cancel := context.WithCancel(context.Background())
	ctx, errs := irrecoverable.WithSignaler(parentCtx)

	return testmock.GenericNode{
		Ctx:                ctx,
		Cancel:             cancel,
		Errs:               errs,
		Log:                log,
		Metrics:            metrics,
		Tracer:             tracer,
		PublicDB:           stateFixture.PublicDB,
		SecretsDB:          stateFixture.SecretsDB,
		LockManager:        stateFixture.LockManager,
		Headers:            stateFixture.Storage.Headers,
		Guarantees:         stateFixture.Storage.Guarantees,
		Seals:              stateFixture.Storage.Seals,
		Payloads:           stateFixture.Storage.Payloads,
		Blocks:             stateFixture.Storage.Blocks,
		QuorumCertificates: stateFixture.Storage.QuorumCertificates,
		Results:            stateFixture.Storage.Results,
		Setups:             stateFixture.Storage.EpochSetups,
		EpochCommits:       stateFixture.Storage.EpochCommits,
		EpochProtocolState: stateFixture.Storage.EpochProtocolStateEntries,
		ProtocolKVStore:    stateFixture.Storage.ProtocolKVStore,
		State:              stateFixture.State,
		Index:              stateFixture.Storage.Index,
		Me:                 me,
		Net:                net,
		DBDir:              stateFixture.DBDir,
		ChainID:            chainID,
		ProtocolEvents:     stateFixture.ProtocolEvents,
	}
}

// CompleteStateFixture is a test helper that creates, bootstraps, and returns a StateFixture for sake of unit testing.
func CompleteStateFixture(
	t testing.TB,
	log zerolog.Logger,
	metric *metrics.NoopCollector,
	tracer module.Tracer,
	rootSnapshot protocol.Snapshot,
) *testmock.StateFixture {

	dataDir := unittest.TempDir(t)
	publicDBDir := filepath.Join(dataDir, "protocol")
	secretsDBDir := filepath.Join(dataDir, "secrets")
	pdb := unittest.TypedPebbleDB(t, publicDBDir, pebble.Open)
	db := pebbleimpl.ToDB(pdb)
	lockManager := storage.NewTestingLockManager()
	s := store.InitAll(metric, db)
	secretsDB := unittest.TypedBadgerDB(t, secretsDBDir, storagebadger.InitSecret)
	consumer := events.NewDistributor()

	state, err := badgerstate.Bootstrap(
		metric,
		db,
		lockManager,
		s.Headers,
		s.Seals,
		s.Results,
		s.Blocks,
		s.QuorumCertificates,
		s.EpochSetups,
		s.EpochCommits,
		s.EpochProtocolStateEntries,
		s.ProtocolKVStore,
		s.VersionBeacons,
		rootSnapshot,
	)
	require.NoError(t, err)

	mutableState, err := badgerstate.NewFullConsensusState(
		log,
		tracer,
		consumer,
		state,
		s.Index,
		s.Payloads,
		util.MockBlockTimer(),
		util.MockReceiptValidator(),
		util.MockSealValidator(s.Seals),
	)
	require.NoError(t, err)

	return &testmock.StateFixture{
		PublicDB:       db,
		SecretsDB:      secretsDB,
		Storage:        s,
		DBDir:          dataDir,
		ProtocolEvents: consumer,
		State:          mutableState,
		LockManager:    lockManager,
	}
}

// CollectionNode returns a mock collection node.
func CollectionNode(t *testing.T, hub *stub.Hub, identity bootstrap.NodeInfo, rootSnapshot protocol.Snapshot) testmock.CollectionNode {
	node := GenericNode(t, hub, identity, rootSnapshot)
	privKeys, err := identity.PrivateKeys()
	require.NoError(t, err)
	node.Me, err = local.New(identity.Identity().IdentitySkeleton, privKeys.StakingKey)
	require.NoError(t, err)

	pools := epochs.NewTransactionPools(
		func(_ uint64) mempool.Transactions {
			return herocache.NewTransactions(1000, node.Log, metrics.NewNoopCollector())
		})

	db := node.PublicDB
	transactions := store.NewTransactions(node.Metrics, db)
	collections := store.NewCollections(db, transactions)
	clusterPayloads := store.NewClusterPayloads(node.Metrics, db)

	ingestionEngine, err := collectioningest.New(node.Log, node.Net, node.State, node.Metrics, node.Metrics, node.Metrics, node.Me, node.ChainID.Chain(), pools, collectioningest.DefaultConfig(),
		ingest.NewAddressRateLimiter(rate.Limit(1), 10)) // 10 tps
	require.NoError(t, err)

	selector := filter.HasRole[flow.Identity](flow.RoleAccess, flow.RoleVerification)
	retrieve := func(collID flow.Identifier) (flow.Entity, error) {
		coll, err := collections.ByID(collID)
		return coll, err
	}
	providerEngine, err := provider.New(
		node.Log,
		node.Metrics,
		node.Net,
		node.Me,
		node.State,
		queue.NewHeroStore(uint32(1000), unittest.Logger(), metrics.NewNoopCollector()),
		uint(1000),
		channels.ProvideCollections,
		selector,
		retrieve)
	require.NoError(t, err)

	pusherEngine, err := pusher.New(node.Log, node.Net, node.State, node.Metrics, node.Metrics, node.Me)
	require.NoError(t, err)

	clusterStateFactory, err := factories.NewClusterStateFactory(
		db,
		node.LockManager,
		node.Metrics,
		node.Tracer,
	)
	require.NoError(t, err)

	builderFactory, err := factories.NewBuilderFactory(
		db,
		node.State,
		node.LockManager,
		node.Headers,
		node.Tracer,
		node.Metrics,
		pusherEngine,
		node.Log,
		updatable_configs.DefaultBySealingLagRateLimiterConfigs(),
	)
	require.NoError(t, err)

	complianceEngineFactory, err := factories.NewComplianceEngineFactory(
		node.Log,
		node.Net,
		node.Me,
		node.Metrics, node.Metrics, node.Metrics,
		node.State,
		compliance.DefaultConfig(),
	)
	require.NoError(t, err)

	syncCoreFactory, err := factories.NewSyncCoreFactory(node.Log, chainsync.DefaultConfig())
	require.NoError(t, err)

	syncFactory, err := factories.NewSyncEngineFactory(
		node.Log,
		node.Metrics,
		node.Net,
		node.Me,
	)
	require.NoError(t, err)

	createMetrics := func(chainID flow.ChainID) module.HotstuffMetrics {
		return metrics.NewNoopCollector()
	}

	hotstuffFactory, err := factories.NewHotStuffFactory(
		node.Log,
		node.Me,
		db,
		node.State,
		node.Metrics,
		node.Metrics,
		createMetrics,
	)
	require.NoError(t, err)

	messageHubFactory := factories.NewMessageHubFactory(
		node.Log,
		node.Net,
		node.Me,
		node.Metrics,
		node.State,
	)

	factory := factories.NewEpochComponentsFactory(
		node.Me,
		pools,
		builderFactory,
		clusterStateFactory,
		hotstuffFactory,
		complianceEngineFactory,
		syncCoreFactory,
		syncFactory,
		messageHubFactory,
	)

	rootQCVoter := new(mockmodule.ClusterRootQCVoter)
	rootQCVoter.On("Vote", mock.Anything, mock.Anything).Return(nil)

	engineEventsDistributor := mockcollection.NewEngineEvents(t)
	engineEventsDistributor.On("ActiveClustersChanged", mock.AnythingOfType("flow.ChainIDList")).Maybe()
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
		engineEventsDistributor,
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

func ConsensusNode(t *testing.T, hub *stub.Hub, identity bootstrap.NodeInfo, identities []*flow.Identity, chainID flow.ChainID) testmock.ConsensusNode {

	node := GenericNodeFromParticipants(t, hub, identity, identities, chainID)

	db := node.PublicDB
	resultsDB := store.NewExecutionResults(node.Metrics, db)
	receiptsDB := store.NewExecutionReceipts(node.Metrics, db, resultsDB, storagebadger.DefaultCacheSize)

	guarantees := stdmap.NewGuarantees(1000)

	receipts := consensusMempools.NewExecutionTree()

	seals := stdmap.NewIncorporatedResultSeals(1000)
	pendingReceipts := stdmap.NewPendingReceipts(node.Headers, 1000)

	ingestionCore := consensusingest.NewCore(node.Log, node.Tracer, node.Metrics, node.State,
		node.Headers, guarantees)
	// receive collections
	ingestionEngine, err := consensusingest.New(node.Log, node.Metrics, node.Net, node.Me, ingestionCore)
	require.NoError(t, err)

	// request receipts from execution nodes
	receiptRequester, err := requester.New(node.Log.With().Str("entity", "receipt").Logger(), node.Metrics, node.Net, node.Me, node.State, channels.RequestReceiptsByBlockID, filter.Any, func() flow.Entity { return new(flow.ExecutionReceipt) })
	require.NoError(t, err)

	assigner, err := chunks.NewChunkAssigner(flow.DefaultChunkAssignmentAlpha, node.State)
	require.NoError(t, err)

	receiptValidator := validation.NewReceiptValidator(
		node.State,
		node.Headers,
		node.Index,
		resultsDB,
		node.Seals,
	)

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
		unittest.NewSealingConfigs(flow.DefaultRequiredApprovalsForSealConstruction),
	)
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

type CheckerMock struct {
	notifications.NoopConsumer // satisfy the FinalizationConsumer interface
}

func ExecutionNode(t *testing.T, hub *stub.Hub, identity bootstrap.NodeInfo, identities []*flow.Identity, syncThreshold int, chainID flow.ChainID) testmock.ExecutionNode {
	node := GenericNodeFromParticipants(t, hub, identity, identities, chainID)

	db := node.PublicDB
	transactionsStorage := store.NewTransactions(node.Metrics, db)
	collectionsStorage := store.NewCollections(db, transactionsStorage)
	eventsStorage := store.NewEvents(node.Metrics, db)
	serviceEventsStorage := store.NewServiceEvents(node.Metrics, db)
	txResultStorage := store.NewTransactionResults(node.Metrics, db, storagebadger.DefaultCacheSize)
	commitsStorage := store.NewCommits(node.Metrics, db)
	chunkDataPackStorage := store.NewChunkDataPacks(node.Metrics, db, collectionsStorage, 100)
	results := store.NewExecutionResults(node.Metrics, db)
	receipts := store.NewExecutionReceipts(node.Metrics, db, results, storagebadger.DefaultCacheSize)
	myReceipts := store.NewMyExecutionReceipts(node.Metrics, db, receipts)
	versionBeacons := store.NewVersionBeacons(db)
	headersStorage := store.NewHeaders(node.Metrics, db)

	checkAuthorizedAtBlock := func(blockID flow.Identifier) (bool, error) {
		return protocol.IsNodeAuthorizedAt(node.State.AtBlockID(blockID), node.Me.NodeID())
	}

	protoState, ok := node.State.(*badgerstate.ParticipantState)
	require.True(t, ok)

	followerState, err := badgerstate.NewFollowerState(
		node.Log,
		node.Tracer,
		node.ProtocolEvents,
		protoState.State,
		node.Index,
		node.Payloads,
		blocktimer.DefaultBlockTimer,
	)
	require.NoError(t, err)

	dbDir := unittest.TempDir(t)

	metricsCollector := &metrics.NoopCollector{}

	const (
		capacity           = 100
		checkpointDistance = math.MaxInt // A large number to prevent checkpoint creation.
		checkpointsToKeep  = 1
	)
	diskWal, err := wal.NewDiskWAL(node.Log.With().Str("subcomponent", "wal").Logger(), nil, metricsCollector, dbDir, capacity, pathfinder.PathByteSize, wal.SegmentSize)
	require.NoError(t, err)

	ls, err := completeLedger.NewLedger(diskWal, capacity, metricsCollector, node.Log.With().Str("component", "ledger").Logger(), completeLedger.DefaultPathFinderVersion)
	require.NoError(t, err)

	compactor, err := completeLedger.NewCompactor(ls, diskWal, zerolog.Nop(), capacity, checkpointDistance, checkpointsToKeep, atomic.NewBool(false), metricsCollector)
	require.NoError(t, err)

	<-compactor.Ready() // Need to start compactor here because BootstrapLedger() updates ledger state.

	genesisHead, err := node.State.Final().Head()
	require.NoError(t, err)

	bootstrapper := bootstrapexec.NewBootstrapper(node.Log)
	commit, err := bootstrapper.BootstrapLedger(
		ls,
		unittest.ServiceAccountPublicKey,
		node.ChainID.Chain(),
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply))
	require.NoError(t, err)

	matchTrie, err := ls.FindTrieByStateCommit(commit)
	require.NoError(t, err)
	require.NotNil(t, matchTrie)

	const bootstrapCheckpointFile = "bootstrap-checkpoint"
	checkpointFile := filepath.Join(dbDir, bootstrapCheckpointFile)
	err = wal.StoreCheckpointV6([]*trie.MTrie{matchTrie}, dbDir, bootstrapCheckpointFile, zerolog.Nop(), 1)
	require.NoError(t, err)

	rootResult, rootSeal, err := protoState.Sealed().SealedResult()
	require.NoError(t, err)

	require.Equal(t, fmt.Sprint(rootSeal.FinalState), fmt.Sprint(commit))
	require.Equal(t, rootSeal.ResultID, rootResult.ID())

	err = bootstrapper.BootstrapExecutionDatabase(node.LockManager, db, rootSeal)
	require.NoError(t, err)

	registerDir := unittest.TempPebblePath(t)
	pebbledb, err := storagepebble.OpenRegisterPebbleDB(node.Log.With().Str("pebbledb", "registers").Logger(), registerDir)
	require.NoError(t, err)

	checkpointHeight := uint64(0)
	require.NoError(t, esbootstrap.ImportRegistersFromCheckpoint(node.Log, checkpointFile, checkpointHeight, matchTrie.RootHash(), pebbledb, 2))

	diskStore, err := storagepebble.NewRegisters(pebbledb, storagepebble.PruningDisabled)
	require.NoError(t, err)

	reader := finalizedreader.NewFinalizedReader(headersStorage, checkpointHeight)
	registerStore, err := storehouse.NewRegisterStore(
		diskStore,
		nil, // TOOD(leo): replace with real WAL
		reader,
		node.Log,
		storehouse.NewNoopNotifier(),
	)
	require.NoError(t, err)

	storehouseEnabled := true
	getLatestFinalized := func() (uint64, error) {
		final, err := protoState.Final().Head()
		if err != nil {
			return 0, err
		}
		return final.Height, nil
	}

	execState := executionState.NewExecutionState(
		ls, commitsStorage, node.Blocks, node.Headers, chunkDataPackStorage, results, myReceipts, eventsStorage, serviceEventsStorage, txResultStorage, db, getLatestFinalized, node.Tracer,
		// TODO: test with register store
		registerStore,
		storehouseEnabled,
		node.LockManager,
	)

	requestEngine, err := requester.New(
		node.Log.With().Str("entity", "collection").Logger(), node.Metrics, node.Net, node.Me, node.State,
		channels.RequestCollections,
		filter.HasRole[flow.Identity](flow.RoleCollection),
		func() flow.Entity { return new(flow.Collection) },
	)
	require.NoError(t, err)

	pusherEngine, err := executionprovider.New(
		node.Log,
		node.Tracer,
		node.Net,
		node.State,
		execState,
		metricsCollector,
		checkAuthorizedAtBlock,
		queue.NewHeroStore(uint32(1000), unittest.Logger(), metrics.NewNoopCollector()),
		executionprovider.DefaultChunkDataPackRequestWorker,
		executionprovider.DefaultChunkDataPackQueryTimeout,
		executionprovider.DefaultChunkDataPackDeliveryTimeout,
	)
	require.NoError(t, err)

	blockFinder := environment.NewBlockFinder(node.Headers)

	vmCtx := fvm.NewContext(
		fvm.WithLogger(node.Log),
		fvm.WithChain(node.ChainID.Chain()),
		fvm.WithBlocks(blockFinder),
	)
	committer := committer.NewLedgerViewCommitter(ls, node.Tracer)

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := mocktracker.NewMockStorage()

	prov := exedataprovider.NewProvider(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		execution_data.DefaultSerializer,
		bservice,
		trackerStorage,
	)

	computationEngine, err := computation.New(
		node.Log,
		node.Metrics,
		node.Tracer,
		node.Me,
		computation.NewProtocolStateWrapper(node.State),
		vmCtx,
		committer,
		prov,
		computation.ComputationConfig{
			QueryConfig:          query.NewDefaultConfig(),
			DerivedDataCacheSize: derived.DefaultDerivedDataCacheSize,
			MaxConcurrency:       1,
		},
	)
	require.NoError(t, err)

	syncCore, err := chainsync.New(node.Log, chainsync.DefaultConfig(), metrics.NewChainSyncCollector(genesisHead.ChainID), genesisHead.ChainID)
	require.NoError(t, err)

	followerDistributor := pubsub.NewFollowerDistributor()
	require.NoError(t, err)

	// disabled by default
	uploader := uploader.NewManager(node.Tracer)

	_, err = build.Semver()
	require.ErrorIs(t, err, build.UndefinedVersionError)
	ver := semver.New("0.0.0")

	latestFinalizedBlock, err := node.State.Final().Head()
	require.NoError(t, err)

	unit := engine.NewUnit()
	stopControl := stop.NewStopControl(
		unit,
		time.Second,
		node.Log,
		execState,
		node.Headers,
		versionBeacons,
		ver,
		latestFinalizedBlock,
		false,
		true,
	)

	fetcher := exeFetcher.NewCollectionFetcher(node.Log, requestEngine, node.State, false)
	rootHead, rootQC := getRoot(t, &node)
	_, ingestionCore, err := ingestion.NewMachine(
		node.Log,
		node.ProtocolEvents,
		requestEngine,
		fetcher,
		node.Headers,
		node.Blocks,
		collectionsStorage,
		execState,
		node.State,
		node.Metrics,
		computationEngine,
		pusherEngine,
		uploader,
		stopControl,
	)
	require.NoError(t, err)
	node.ProtocolEvents.AddConsumer(stopControl)

	followerCore, finalizer := createFollowerCore(t, &node, followerState, followerDistributor, rootHead, rootQC)
	// mock out hotstuff validator
	validator := new(mockhotstuff.Validator)
	validator.On("ValidateProposal", mock.Anything).Return(nil)

	core, err := follower.NewComplianceCore(
		node.Log,
		node.Metrics,
		node.Metrics,
		followerDistributor,
		followerState,
		followerCore,
		validator,
		syncCore,
		node.Tracer,
	)
	require.NoError(t, err)

	finalizedHeader, err := protoState.Final().Head()
	require.NoError(t, err)
	followerEng, err := follower.NewComplianceLayer(
		node.Log,
		node.Net,
		node.Me,
		node.Metrics,
		node.Headers,
		finalizedHeader,
		core,
		compliance.DefaultConfig(),
	)
	require.NoError(t, err)

	idCache, err := cache.NewProtocolStateIDCache(node.Log, node.State, events.NewDistributor())
	require.NoError(t, err, "could not create finalized snapshot cache")
	spamConfig, err := synchronization.NewSpamDetectionConfig()
	require.NoError(t, err, "could not initialize spam detection config")
	syncEngine, err := synchronization.New(
		node.Log,
		node.Metrics,
		node.Net,
		node.Me,
		node.State,
		node.Blocks,
		followerEng,
		syncCore,
		id.NewIdentityFilterIdentifierProvider(
			filter.And(
				filter.HasRole[flow.Identity](flow.RoleConsensus),
				filter.Not(filter.HasNodeID[flow.Identity](node.Me.NodeID())),
			),
			idCache,
		),
		spamConfig,
		synchronization.WithPollInterval(time.Duration(0)),
	)
	require.NoError(t, err)
	followerDistributor.AddFinalizationConsumer(syncEngine)

	return testmock.ExecutionNode{
		GenericNode:         node,
		FollowerState:       followerState,
		IngestionEngine:     ingestionCore,
		FollowerCore:        followerCore,
		FollowerEngine:      followerEng,
		SyncEngine:          syncEngine,
		ExecutionEngine:     computationEngine,
		RequestEngine:       requestEngine,
		ReceiptsEngine:      pusherEngine,
		ProtocolDB:          node.PublicDB,
		VM:                  computationEngine.VM(),
		ExecutionState:      execState,
		Ledger:              ls,
		LevelDbDir:          dbDir,
		Collections:         collectionsStorage,
		Finalizer:           finalizer,
		MyExecutionReceipts: myReceipts,
		Compactor:           compactor,
		StorehouseEnabled:   storehouseEnabled,
	}
}

func getRoot(t *testing.T, node *testmock.GenericNode) (*flow.Header, *flow.QuorumCertificate) {
	rootHead := node.State.Params().FinalizedRoot()

	signers, err := node.State.AtHeight(0).Identities(filter.HasRole[flow.Identity](flow.RoleConsensus))
	require.NoError(t, err)

	signerIDs := signers.NodeIDs()
	signerIndices, err := signature.EncodeSignersToIndices(signerIDs, signerIDs)
	require.NoError(t, err)

	rootQC, err := flow.NewQuorumCertificate(flow.UntrustedQuorumCertificate{
		View:          rootHead.View,
		BlockID:       rootHead.ID(),
		SignerIndices: signerIndices,
		SigData:       unittest.SignatureFixture(),
	})
	require.NoError(t, err)

	return rootHead, rootQC
}

type RoundRobinLeaderSelection struct {
	identities flow.IdentityList
	me         flow.Identifier
}

var _ hotstuff.Replicas = (*RoundRobinLeaderSelection)(nil)
var _ hotstuff.DynamicCommittee = (*RoundRobinLeaderSelection)(nil)

func (s *RoundRobinLeaderSelection) IdentitiesByBlock(_ flow.Identifier) (flow.IdentityList, error) {
	return s.identities, nil
}

func (s *RoundRobinLeaderSelection) IdentityByBlock(_ flow.Identifier, participantID flow.Identifier) (*flow.Identity, error) {
	id, found := s.identities.ByNodeID(participantID)
	if !found {
		return nil, model.NewInvalidSignerErrorf("unknown participant %x", participantID)
	}

	return id, nil
}

func (s *RoundRobinLeaderSelection) IdentitiesByEpoch(view uint64) (flow.IdentitySkeletonList, error) {
	return s.identities.ToSkeleton(), nil
}

func (s *RoundRobinLeaderSelection) IdentityByEpoch(view uint64, participantID flow.Identifier) (*flow.IdentitySkeleton, error) {
	id, found := s.identities.ByNodeID(participantID)
	if !found {
		return nil, model.NewInvalidSignerErrorf("unknown participant %x", participantID)
	}
	return &id.IdentitySkeleton, nil
}

func (s *RoundRobinLeaderSelection) LeaderForView(view uint64) (flow.Identifier, error) {
	return s.identities[int(view)%len(s.identities)].NodeID, nil
}

func (s *RoundRobinLeaderSelection) QuorumThresholdForView(_ uint64) (uint64, error) {
	return committees.WeightThresholdToBuildQC(s.identities.ToSkeleton().TotalWeight()), nil
}

func (s *RoundRobinLeaderSelection) TimeoutThresholdForView(_ uint64) (uint64, error) {
	return committees.WeightThresholdToTimeout(s.identities.ToSkeleton().TotalWeight()), nil
}

func (s *RoundRobinLeaderSelection) Self() flow.Identifier {
	return s.me
}

func (s *RoundRobinLeaderSelection) DKG(_ uint64) (hotstuff.DKG, error) {
	return nil, fmt.Errorf("error")
}

func createFollowerCore(
	t *testing.T,
	node *testmock.GenericNode,
	followerState *badgerstate.FollowerState,
	notifier hotstuff.FollowerConsumer,
	rootHead *flow.Header,
	rootQC *flow.QuorumCertificate,
) (module.HotStuffFollower, *confinalizer.Finalizer) {
	finalizer := confinalizer.NewFinalizer(node.PublicDB.Reader(), node.Headers, followerState, trace.NewNoopTracer())

	pending := make([]*flow.ProposalHeader, 0)

	// creates a consensus follower with noop consumer as the notifier
	followerCore, err := consensus.NewFollower(
		node.Log,
		node.Metrics,
		node.Headers,
		finalizer,
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
	verIdentity bootstrap.NodeInfo, // identity of this verification node.
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
		require.NoError(t, err)
	}

	if node.ChunkRequests == nil {
		node.ChunkRequests = stdmap.NewChunkRequests(chunksLimit)
		err = mempoolCollector.Register(metrics.ResourceChunkRequest, node.ChunkRequests.Size)
		require.NoError(t, err)
	}

	if node.Results == nil {
		db := node.PublicDB
		results := store.NewExecutionResults(node.Metrics, db)
		node.Results = results
		node.Receipts = store.NewExecutionReceipts(node.Metrics, db, results, storagebadger.DefaultCacheSize)
	}

	if node.ProcessedChunkIndex == nil {
		node.ProcessedChunkIndex = store.NewConsumerProgress(node.PublicDB, module.ConsumeProgressVerificationChunkIndex)
	}

	if node.ChunksQueue == nil {
		cq := store.NewChunkQueue(node.Metrics, node.PublicDB)
		ok, err := cq.Init(chunkconsumer.DefaultJobIndex)
		require.NoError(t, err)
		require.True(t, ok)
		node.ChunksQueue = cq
	}

	if node.ProcessedBlockHeight == nil {
		node.ProcessedBlockHeight = store.NewConsumerProgress(node.PublicDB, module.ConsumeProgressVerificationBlockHeight)
	}
	if node.VerifierEngine == nil {
		vm := fvm.NewVirtualMachine()

		blockFinder := environment.NewBlockFinder(node.Headers)

		vmCtx := fvm.NewContext(
			fvm.WithLogger(node.Log),
			fvm.WithChain(node.ChainID.Chain()),
			fvm.WithBlocks(blockFinder),
		)

		chunkVerifier := chunks.NewChunkVerifier(vm, vmCtx, node.Log)

		approvalStorage := store.NewResultApprovals(node.Metrics, node.PublicDB, node.LockManager)

		node.VerifierEngine, err = verifier.New(node.Log,
			collector,
			node.Tracer,
			node.Net,
			node.State,
			node.Me,
			chunkVerifier,
			approvalStorage,
			node.LockManager,
		)
		require.NoError(t, err)
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
			0,
		)
	}

	if node.ChunkConsumer == nil {
		node.ChunkConsumer, err = chunkconsumer.NewChunkConsumer(node.Log,
			collector,
			node.ProcessedChunkIndex,
			node.ChunksQueue,
			node.FetcherEngine,
			chunkconsumer.DefaultChunkWorkers) // defaults number of workers to 3.
		require.NoError(t, err)
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
			node.ChunkConsumer,
			0)
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
