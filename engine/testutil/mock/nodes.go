package mock

import (
	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/txpool"
	"github.com/dapperlabs/flow-go/protocol"
)

// CollectionNode implements a mocked collection node for tests
type CollectionNode struct {
	State           protocol.State
	Me              *local.Local
	IngestionEngine *ingest.Engine
	Pool            *txpool.Pool
}
