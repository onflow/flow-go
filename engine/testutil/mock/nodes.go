package mock

import (
	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/protocol"
)

// CollectionNode implements a mocked collection node for tests
type CollectionNode struct {
	State           protocol.State
	Me              module.Local
	Pool            module.TransactionPool
	IngestionEngine *ingest.Engine
}
