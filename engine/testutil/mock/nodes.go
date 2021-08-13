package mock

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
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
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/ingestion"
	executionprovider "github.com/onflow/flow-go/engine/execution/provider"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/verification/assigner"
	"github.com/onflow/flow-go/engine/verification/assigner/blockconsumer"
	"github.com/onflow/flow-go/engine/verification/fetcher"
	"github.com/onflow/flow-go/engine/verification/fetcher/chunkconsumer"
	verificationrequester "github.com/onflow/flow-go/engine/verification/requester"
	"github.com/onflow/flow-go/engine/verification/verifier"
	"github.com/onflow/flow-go/fvm"
	fvmState "github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/mempool/entity"
	epochpool "github.com/onflow/flow-go/module/mempool/epochs"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

// StateFixture is a test helper struct that encapsulates a flow protocol state
// as well as all of its backend dependencies.
type StateFixture struct {
	DB             *badger.DB
	Storage        *storage.All
	DBDir          string
	ProtocolEvents *events.Distributor
	State          protocol.MutableState
}

// GenericNode implements a generic in-process node for tests.
type GenericNode struct {
	Log            zerolog.Logger
	Metrics        *metrics.NoopCollector
	Tracer         module.Tracer
	DB             *badger.DB
	Headers        storage.Headers
	Identities     storage.Identities
	Guarantees     storage.Guarantees
	Seals          storage.Seals
	Payloads       storage.Payloads
	Blocks         storage.Blocks
	State          protocol.MutableState
	Index          storage.Index
	Me             module.Local
	Net            *stub.Network
	DBDir          string
	ChainID        flow.ChainID
	ProtocolEvents *events.Distributor
}

func (g *GenericNode) Done() {
	_ = g.DB.Close()
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
	return g.DB.Close()
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

func (n CollectionNode) Ready() <-chan struct{} {
	return lifecycle.AllReady(
		n.PusherEngine,
		n.ProviderEngine,
		n.IngestionEngine,
		n.EpochManagerEngine,
	)
}

func (n CollectionNode) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		<-lifecycle.AllDone(
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

func (cn ConsensusNode) Ready() {
	<-cn.IngestionEngine.Ready()
	<-cn.SealingEngine.Ready()
}

func (cn ConsensusNode) Done() {
	<-cn.IngestionEngine.Done()
	<-cn.SealingEngine.Done()
}

type ComputerWrap struct {
	*computation.Manager
	OnComputeBlock func(ctx context.Context, block *entity.ExecutableBlock, view fvmState.View)
}

func (c *ComputerWrap) ComputeBlock(
	ctx context.Context,
	block *entity.ExecutableBlock,
	view fvmState.View,
) (*execution.ComputationResult, error) {
	if c.OnComputeBlock != nil {
		c.OnComputeBlock(ctx, block, view)
	}
	return c.Manager.ComputeBlock(ctx, block, view)
}

// ExecutionNode implements a mocked execution node for tests.
type ExecutionNode struct {
	GenericNode
	MutableState        protocol.MutableState
	IngestionEngine     *ingestion.Engine
	ExecutionEngine     *ComputerWrap
	RequestEngine       *requester.Engine
	ReceiptsEngine      *executionprovider.Engine
	FollowerEngine      *followereng.Engine
	SyncEngine          *synchronization.Engine
	DiskWAL             *wal.DiskWAL
	BadgerDB            *badger.DB
	VM                  *fvm.VirtualMachine
	ExecutionState      state.ExecutionState
	Ledger              ledger.Ledger
	LevelDbDir          string
	Collections         storage.Collections
	Finalizer           *consensus.Finalizer
	MyExecutionReceipts storage.MyExecutionReceipts
}

func (en ExecutionNode) Ready() {
	<-lifecycle.AllReady(
		en.Ledger,
		en.ReceiptsEngine,
		en.IngestionEngine,
		en.FollowerEngine,
		en.RequestEngine,
		en.SyncEngine,
		en.DiskWAL,
	)
}

func (en ExecutionNode) Done() {
	lifecycle.AllDone(
		en.IngestionEngine,
		en.IngestionEngine,
		en.ReceiptsEngine,
		en.Ledger,
		en.FollowerEngine,
		en.RequestEngine,
		en.SyncEngine,
		en.DiskWAL,
	)
	os.RemoveAll(en.LevelDbDir)
	en.GenericNode.Done()
}

func (en ExecutionNode) AssertHighestExecutedBlock(t *testing.T, header *flow.Header) {

	height, blockID, err := en.ExecutionState.GetHighestExecutedBlockID(context.Background())
	require.NoError(t, err)

	require.Equal(t, header.ID(), blockID)
	require.Equal(t, header.Height, height)
}

// VerificationNode implements an in-process verification node for tests.
type VerificationNode struct {
	*GenericNode
	ChunkStatuses mempool.ChunkStatuses
	ChunkRequests mempool.ChunkRequests
	Results       storage.ExecutionResults
	Receipts      storage.ExecutionReceipts

	// chunk consumer and processor for fetcher engine
	ProcessedChunkIndex storage.ConsumerProgress
	ChunksQueue         *bstorage.ChunksQueue
	ChunkConsumer       *chunkconsumer.ChunkConsumer

	// block consumer for chunk consumer
	ProcessedBlockHeight storage.ConsumerProgress
	BlockConsumer        *blockconsumer.BlockConsumer

	VerifierEngine  *verifier.Engine
	AssignerEngine  *assigner.Engine
	FetcherEngine   *fetcher.Engine
	RequesterEngine *verificationrequester.Engine
}
