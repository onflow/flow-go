package mock

import (
	"context"
	"os"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	collectioningest "github.com/onflow/flow-go/engine/collection/ingest"
	"github.com/onflow/flow-go/engine/collection/pusher"
	"github.com/onflow/flow-go/engine/common/provider"
	"github.com/onflow/flow-go/engine/common/synchronization"
	consensusingest "github.com/onflow/flow-go/engine/consensus/ingestion"
	"github.com/onflow/flow-go/engine/consensus/matching"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/ingestion"
	executionprovider "github.com/onflow/flow-go/engine/execution/provider"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/verification/finder"
	"github.com/onflow/flow-go/engine/verification/match"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// GenericNode implements a generic in-process node for tests.
type GenericNode struct {
	Log        zerolog.Logger
	Metrics    *metrics.NoopCollector
	Tracer     module.Tracer
	DB         *badger.DB
	Headers    storage.Headers
	Identities storage.Identities
	Guarantees storage.Guarantees
	Seals      storage.Seals
	Payloads   storage.Payloads
	Blocks     storage.Blocks
	State      protocol.State
	Index      storage.Index
	Me         module.Local
	Net        *stub.Network
	DBDir      string
	ChainID    flow.ChainID
}

func (g *GenericNode) Done() {
	_ = g.DB.Close()
	_ = os.RemoveAll(g.DBDir)

	<-g.Tracer.Done()
}

// Closes closes the badger database of the node
func (g *GenericNode) CloseDB() error {
	return g.DB.Close()
}

// CollectionNode implements an in-process collection node for tests.
type CollectionNode struct {
	GenericNode
	Pool            mempool.Transactions
	Collections     storage.Collections
	Transactions    storage.Transactions
	IngestionEngine *collectioningest.Engine
	PusherEngine    *pusher.Engine
	ProviderEngine  *provider.Engine
}

// ConsensusNode implements an in-process consensus node for tests.
type ConsensusNode struct {
	GenericNode
	Guarantees      mempool.Guarantees
	Approvals       mempool.Approvals
	Receipts        mempool.Receipts
	Seals           mempool.Seals
	IngestionEngine *consensusingest.Engine
	MatchingEngine  *matching.Engine
}

// ExecutionNode implements a mocked execution node for tests.
type ExecutionNode struct {
	GenericNode
	IngestionEngine *ingestion.Engine
	ExecutionEngine *computation.Manager
	ReceiptsEngine  *executionprovider.Engine
	SyncEngine      *synchronization.Engine
	BadgerDB        *badger.DB
	VM              *fvm.VirtualMachine
	ExecutionState  state.ExecutionState
	Ledger          storage.Ledger
	LevelDbDir      string
	Collections     storage.Collections
}

func (en ExecutionNode) Ready() {
	<-en.Ledger.Ready()
	<-en.ReceiptsEngine.Ready()
	<-en.IngestionEngine.Ready()
	<-en.SyncEngine.Ready()
}

func (en ExecutionNode) Done() {
	<-en.IngestionEngine.Done()
	<-en.ReceiptsEngine.Done()
	<-en.SyncEngine.Done()
	<-en.Ledger.Done()
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
	GenericNode
	CachedReceipts           mempool.ReceiptDataPacks
	ReadyReceipts            mempool.ReceiptDataPacks
	PendingReceipts          mempool.ReceiptDataPacks
	PendingResults           mempool.ResultDataPacks
	ProcessedResultIDs       mempool.Identifiers
	BlockIDsCache            mempool.Identifiers
	PendingReceiptIDsByBlock mempool.IdentifierMap
	ReceiptIDsByResult       mempool.IdentifierMap
	ChunkIDsByResult         mempool.IdentifierMap
	PendingChunks            *match.Chunks
	HeaderStorage            storage.Headers
	VerifierEngine           network.Engine
	FinderEngine             *finder.Engine
	MatchEngine              network.Engine
}
