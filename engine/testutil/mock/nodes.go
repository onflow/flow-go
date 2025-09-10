package mock

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/collection/epochmgr"
	collectioningest "github.com/onflow/flow-go/engine/collection/ingest"
	"github.com/onflow/flow-go/engine/collection/pusher"
	followereng "github.com/onflow/flow-go/engine/common/follower"
	"github.com/onflow/flow-go/engine/common/provider"
	"github.com/onflow/flow-go/engine/common/requester"
	"github.com/onflow/flow-go/engine/common/synchronization"
	consensusingest "github.com/onflow/flow-go/engine/consensus/ingestion"
	"github.com/onflow/flow-go/engine/consensus/matching"
	"github.com/onflow/flow-go/engine/consensus/sealing"
	"github.com/onflow/flow-go/engine/execution/computation"
	executionIngest "github.com/onflow/flow-go/engine/execution/ingestion"
	executionprovider "github.com/onflow/flow-go/engine/execution/provider"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/verification/assigner"
	"github.com/onflow/flow-go/engine/verification/assigner/blockconsumer"
	"github.com/onflow/flow-go/engine/verification/fetcher"
	"github.com/onflow/flow-go/engine/verification/fetcher/chunkconsumer"
	verificationrequester "github.com/onflow/flow-go/engine/verification/requester"
	"github.com/onflow/flow-go/engine/verification/verifier"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool"
	epochpool "github.com/onflow/flow-go/module/mempool/epochs"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

// StateFixture is a test helper struct that encapsulates a flow protocol state
// as well as all of its backend dependencies.
type StateFixture struct {
	DBDir          string
	PublicDB       storage.DB
	SecretsDB      *badger.DB
	Storage        *store.All
	ProtocolEvents *events.Distributor
	State          protocol.ParticipantState
	LockManager    lockctx.Manager
}

// GenericNode implements a generic in-process node for tests.
type GenericNode struct {
	// context and cancel function used to start/stop components
	Ctx    irrecoverable.SignalerContext
	Cancel context.CancelFunc
	Errs   <-chan error

	Log                zerolog.Logger
	Metrics            *metrics.NoopCollector
	Tracer             module.Tracer
	PublicDB           storage.DB
	SecretsDB          *badger.DB
	LockManager        lockctx.Manager
	Headers            storage.Headers
	Guarantees         storage.Guarantees
	Seals              storage.Seals
	Payloads           storage.Payloads
	Blocks             storage.Blocks
	QuorumCertificates storage.QuorumCertificates
	Results            storage.ExecutionResults
	Setups             storage.EpochSetups
	EpochCommits       storage.EpochCommits
	EpochProtocolState storage.EpochProtocolStateEntries
	ProtocolKVStore    storage.ProtocolKVStore
	State              protocol.ParticipantState
	Index              storage.Index
	Me                 module.Local
	Net                *stub.Network
	DBDir              string
	ChainID            flow.ChainID
	ProtocolEvents     *events.Distributor
}

func (g *GenericNode) Done() {
	_ = g.PublicDB.Close()
	_ = os.RemoveAll(g.DBDir)

	<-g.Tracer.Done()
}

// RequireGenericNodesDoneBefore invokes the done method of all input generic nodes concurrently, and
// fails the test if any generic node's shutdown takes longer than the specified duration.
func RequireGenericNodesDoneBefore(t testing.TB, duration time.Duration, nodes ...*GenericNode) {
	wg := &sync.WaitGroup{}
	wg.Add(len(nodes))

	for _, node := range nodes {
		go func(n *GenericNode) {
			n.Done()
			wg.Done()
		}(node)
	}

	unittest.RequireReturnsBefore(t, wg.Wait, duration, "failed to shutdown all components on time")
}

// CloseDB closes the badger database of the node
func (g *GenericNode) CloseDB() error {
	return g.PublicDB.Close()
}

// CollectionNode implements an in-process collection node for tests.
type CollectionNode struct {
	GenericNode
	Collections        storage.Collections
	Transactions       storage.Transactions
	ClusterPayloads    storage.ClusterPayloads
	TxPools            *epochpool.TransactionPools
	Voter              module.ClusterRootQCVoter
	IngestionEngine    *collectioningest.Engine
	PusherEngine       *pusher.Engine
	ProviderEngine     *provider.Engine
	EpochManagerEngine *epochmgr.Engine
}

func (n CollectionNode) Start(t *testing.T) {
	go unittest.FailOnIrrecoverableError(t, n.Ctx.Done(), n.Errs)
	n.IngestionEngine.Start(n.Ctx)
	n.EpochManagerEngine.Start(n.Ctx)
	n.ProviderEngine.Start(n.Ctx)
	n.PusherEngine.Start(n.Ctx)
}

func (n CollectionNode) Ready() <-chan struct{} {
	return util.AllReady(
		n.PusherEngine,
		n.ProviderEngine,
		n.IngestionEngine,
		n.EpochManagerEngine,
	)
}

func (n CollectionNode) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		n.GenericNode.Cancel()
		<-util.AllDone(
			n.PusherEngine,
			n.ProviderEngine,
			n.IngestionEngine,
			n.EpochManagerEngine,
		)
		n.GenericNode.Done()
		close(done)
	}()
	return done
}

// ConsensusNode implements an in-process consensus node for tests.
type ConsensusNode struct {
	GenericNode
	Guarantees      mempool.Guarantees
	Receipts        mempool.ExecutionTree
	Seals           mempool.IncorporatedResultSeals
	IngestionEngine *consensusingest.Engine
	SealingEngine   *sealing.Engine
	MatchingEngine  *matching.Engine
}

func (cn ConsensusNode) Start(t *testing.T) {
	go unittest.FailOnIrrecoverableError(t, cn.Ctx.Done(), cn.Errs)
	cn.IngestionEngine.Start(cn.Ctx)
	cn.SealingEngine.Start(cn.Ctx)
}

func (cn ConsensusNode) Ready() <-chan struct{} {
	return util.AllReady(
		cn.IngestionEngine,
		cn.SealingEngine,
	)
}

func (cn ConsensusNode) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		cn.GenericNode.Cancel()
		<-util.AllDone(
			cn.IngestionEngine,
			cn.SealingEngine,
		)
		cn.GenericNode.Done()
		close(done)
	}()
	return done
}

// ExecutionNode implements a mocked execution node for tests.
type ExecutionNode struct {
	GenericNode
	FollowerState       protocol.FollowerState
	IngestionEngine     *executionIngest.Core
	ExecutionEngine     *computation.Manager
	RequestEngine       *requester.Engine
	ReceiptsEngine      *executionprovider.Engine
	FollowerCore        module.HotStuffFollower
	FollowerEngine      *followereng.ComplianceEngine
	SyncEngine          *synchronization.Engine
	Compactor           *complete.Compactor
	ProtocolDB          storage.DB
	VM                  fvm.VM
	ExecutionState      state.ExecutionState
	Ledger              ledger.Ledger
	LevelDbDir          string
	Collections         storage.Collections
	Finalizer           *consensus.Finalizer
	MyExecutionReceipts storage.MyExecutionReceipts
	StorehouseEnabled   bool
}

func (en ExecutionNode) Ready(t *testing.T, ctx context.Context) {
	// TODO: receipt engine has been migrated to the new component interface, hence
	// is using Start. Other engines' startup should be refactored once migrated to
	// new interface.
	irctx := irrecoverable.NewMockSignalerContext(t, ctx)
	en.ReceiptsEngine.Start(irctx)
	en.IngestionEngine.Start(irctx)
	en.FollowerCore.Start(irctx)
	en.FollowerEngine.Start(irctx)
	en.SyncEngine.Start(irctx)

	<-util.AllReady(
		en.Ledger,
		en.ReceiptsEngine,
		en.IngestionEngine,
		en.FollowerCore,
		en.FollowerEngine,
		en.RequestEngine,
		en.SyncEngine,
	)
}

func (en ExecutionNode) Done(cancelFunc context.CancelFunc) {
	// to stop all components running with a component manager.
	cancelFunc()

	// to stop all (deprecated) ready-done-aware
	<-util.AllDone(
		en.IngestionEngine,
		en.ReceiptsEngine,
		en.Ledger,
		en.FollowerCore,
		en.FollowerEngine,
		en.RequestEngine,
		en.SyncEngine,
		en.Compactor,
	)
	os.RemoveAll(en.LevelDbDir)
	en.GenericNode.Done()
}

func (en ExecutionNode) AssertHighestExecutedBlock(t *testing.T, header *flow.Header) {
	height, blockID, err := en.ExecutionState.GetLastExecutedBlockID(context.Background())
	require.NoError(t, err)

	require.Equal(t, header.ID(), blockID)
	require.Equal(t, header.Height, height)
}

func (en ExecutionNode) AssertBlockIsExecuted(t *testing.T, header *flow.Header) {
	executed, err := en.ExecutionState.IsBlockExecuted(header.Height, header.ID())
	require.NoError(t, err)
	require.True(t, executed)
}

func (en ExecutionNode) AssertBlockNotExecuted(t *testing.T, header *flow.Header) {
	executed, err := en.ExecutionState.IsBlockExecuted(header.Height, header.ID())
	require.NoError(t, err)
	require.False(t, executed)
}

// VerificationNode implements an in-process verification node for tests.
type VerificationNode struct {
	*GenericNode
	ChunkStatuses mempool.ChunkStatuses
	ChunkRequests mempool.ChunkRequests
	Results       storage.ExecutionResults
	Receipts      storage.ExecutionReceipts

	// chunk consumer and processor for fetcher engine
	ProcessedChunkIndex storage.ConsumerProgressInitializer
	ChunksQueue         storage.ChunksQueue
	ChunkConsumer       *chunkconsumer.ChunkConsumer

	// block consumer for chunk consumer
	ProcessedBlockHeight storage.ConsumerProgressInitializer
	BlockConsumer        *blockconsumer.BlockConsumer

	VerifierEngine  *verifier.Engine
	AssignerEngine  *assigner.Engine
	FetcherEngine   *fetcher.Engine
	RequesterEngine *verificationrequester.Engine
}
