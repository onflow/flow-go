package mock

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

	collectioningest "github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/engine/collection/provider"
	consensusingest "github.com/dapperlabs/flow-go/engine/consensus/ingestion"
	"github.com/dapperlabs/flow-go/engine/consensus/propagation"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// GenericNode implements a generic node for tests.
type GenericNode struct {
	Log   zerolog.Logger
	DB    *badger.DB
	State protocol.State
	Me    module.Local
	Net   *stub.Network
}

// CollectionNode implements a mocked collection node for tests.
type CollectionNode struct {
	GenericNode
	Pool            module.TransactionPool
	Collections     storage.Collections
	IngestionEngine *collectioningest.Engine
	ProviderEngine  *provider.Engine
}

// ConsensusNode implements a mocked consensus node for tests.
type ConsensusNode struct {
	GenericNode
	Pool              module.CollectionGuaranteePool
	IngestionEngine   *consensusingest.Engine
	PropagationEngine *propagation.Engine
}
