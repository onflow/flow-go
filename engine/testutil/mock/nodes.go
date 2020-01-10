package mock

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

	collectioningest "github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/engine/collection/provider"
	consensusingest "github.com/dapperlabs/flow-go/engine/consensus/ingestion"
	"github.com/dapperlabs/flow-go/engine/consensus/propagation"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/engine/verification/receiver"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// GenericNode implements a generic in-process node for tests.
type GenericNode struct {
	Log   zerolog.Logger
	DB    *badger.DB
	State protocol.State
	Me    module.Local
	Net   *stub.Network
}

// CollectionNode implements an in-process collection node for tests.
type CollectionNode struct {
	GenericNode
	Pool            module.TransactionPool
	Collections     storage.Collections
	IngestionEngine *collectioningest.Engine
	ProviderEngine  *provider.Engine
}

// ConsensusNode implements an in-process consensus node for tests.
type ConsensusNode struct {
	GenericNode
	Pool              module.CollectionGuaranteePool
	IngestionEngine   *consensusingest.Engine
	PropagationEngine *propagation.Engine
}

// VerificationNode implements an in-process verification node for tests.
type VerificationNode struct {
	GenericNode
	Pool                     verification.Mempool
	CollectionReceiverEngine *receiver.Engine
	ExecutionReceiverEngine  *receiver.Engine
	VerifierEngine           network.Engine
}
