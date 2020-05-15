package mock

import (
	"context"
	"os"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

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
	"github.com/dapperlabs/flow-go/engine/verification/ingest"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
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
	Index      storage.Index
	Payloads   storage.Payloads
	Blocks     storage.Blocks
	State      protocol.State
	Me         module.Local
	Net        *stub.Network
	DBDir      string
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
	ProviderEngine  *provider.Engine
}

// ConsensusNode implements an in-process consensus node for tests.
type ConsensusNode struct {
	GenericNode
	Guarantees        mempool.Guarantees
	Approvals         mempool.Approvals
	Receipts          mempool.Receipts
	Seals             mempool.Seals
	IngestionEngine   *consensusingest.Engine
	PropagationEngine *propagation.Engine
	MatchingEngine    *matching.Engine
}

// ExecutionNode implements a mocked execution node for tests.
type ExecutionNode struct {
	GenericNode
	IngestionEngine *ingestion.Engine
	ExecutionEngine *computation.Manager
	ReceiptsEngine  *executionprovider.Engine
	BadgerDB        *badger.DB
	VM              virtualmachine.VirtualMachine
	ExecutionState  state.ExecutionState
	Ledger          storage.Ledger
	LevelDbDir      string
	Collections     storage.Collections
}

func (en ExecutionNode) Done() {
	<-en.IngestionEngine.Done()
	<-en.ReceiptsEngine.Done()
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
	AuthReceipts          mempool.Receipts
	PendingReceipts       mempool.PendingReceipts
	AuthCollections       mempool.Collections
	PendingCollections    mempool.PendingCollections
	CollectionTrackers    mempool.CollectionTrackers
	ChunkDataPacks        mempool.ChunkDataPacks
	ChunkDataPackTrackers mempool.ChunkDataPackTrackers
	IngestedChunkIDs      mempool.Identifiers
	IngestedResultIDs     mempool.Identifiers
	IngestedCollectionIDs mempool.Identifiers
	AssignedChunkIDs      mempool.Identifiers
	IngestEngine          *ingest.Engine
	LightIngestEngine     *ingest.LightEngine // a lighter version of ingest engine
	VerifierEngine        network.Engine
}
