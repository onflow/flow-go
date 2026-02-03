package hotstuff

import "github.com/onflow/flow-go/consensus/hotstuff/model"

type Distributor interface {
	AddOnBlockFinalizedConsumer(consumer func(block *model.Block))
	AddOnBlockIncorporatedConsumer(consumer func(block *model.Block))
}
