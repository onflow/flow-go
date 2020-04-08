package mock

import (
	"os"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

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
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// GenericNode implements a generic in-process node for tests.
type GenericNode struct {
	Log    zerolog.Logger
	Tracer trace.Tracer
	DB     *badger.DB
	State  protocol.State
	Me     module.Local
	Net    *stub.Network
	DBDir  string
}

func (g *GenericNode) Done() {
	_ = g.DB.Close()
	_ = os.RemoveAll(g.DBDir)
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
	State           state.ExecutionState
	Ledger          storage.Ledger
	LevelDbDir      string
}

func (en ExecutionNode) Done() {
	<-en.IngestionEngine.Done()
	<-en.ReceiptsEngine.Done()
	<-en.Ledger.Done()
	os.RemoveAll(en.LevelDbDir)
	en.GenericNode.Done()
}

// VerificationNode implements an in-process verification node for tests.
type VerificationNode struct {
	GenericNode
	AuthReceipts          mempool.Receipts
	PendingReceipts       mempool.PendingReceipts
	BlockStorage          storage.Blocks
	AuthCollections       mempool.Collections
	PendingCollections    mempool.PendingCollections
	CollectionTrackers    mempool.CollectionTrackers
	ChunkDataPacks        mempool.ChunkDataPacks
	ChunkDataPackTrackers mempool.ChunkDataPackTrackers
	IngestEngine          *ingest.Engine
	VerifierEngine        network.Engine
}
