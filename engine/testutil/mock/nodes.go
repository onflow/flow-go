package mock

import (
	collectioningest "github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/engine/collection/provider"
	consensusingest "github.com/dapperlabs/flow-go/engine/consensus/ingestion"
	"github.com/dapperlabs/flow-go/engine/consensus/propagation"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/protocol"
)

// CollectionNode implements a mocked collection node for tests.
type CollectionNode struct {
	State           protocol.State
	Me              module.Local
	Pool            module.TransactionPool
	IngestionEngine *collectioningest.Engine
	ProviderEngine  *provider.Engine
}

// ConsensusNode implements a mocked consensus node for tests.
type ConsensusNode struct {
	State             protocol.State
	Me                module.Local
	Pool              module.CollectionGuaranteePool
	IngestionEngine   *consensusingest.Engine
	PropagationEngine *propagation.Engine
}
