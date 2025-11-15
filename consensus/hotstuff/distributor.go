package hotstuff

import "github.com/onflow/flow-go/consensus/hotstuff/model"

// Distributor is an interface for distributing HotStuff consensus events to registered consumers.
// It provides a pub-sub mechanism where components can register callbacks to be notified
// when blocks are finalized or incorporated into the chain.
//
// The distributor pattern centralizes event distribution and ensures that all registered
// consumers are notified of consensus events. This prevents callers from forgetting to
// register callbacks and provides a consistent way to handle consensus notifications.
type Distributor interface {
	// AddOnBlockFinalizedConsumer registers a callback function that will be invoked
	// whenever a block is finalized. Finalized blocks have achieved consensus finality
	// and are considered immutable.
	//
	// The consumer function is called synchronously when OnFinalizedBlock is invoked
	// on the distributor. Multiple consumers can be registered, and all will be notified
	// in the order they were registered.
	AddOnBlockFinalizedConsumer(consumer func(block *model.Block))

	// AddOnBlockIncorporatedConsumer registers a callback function that will be invoked
	// whenever a block is incorporated into the chain. An incorporated block is one that
	// has been added to the chain but may not yet be finalized.
	//
	// The consumer function is called synchronously when OnBlockIncorporated is invoked
	// on the distributor. Multiple consumers can be registered, and all will be notified
	// in the order they were registered.
	AddOnBlockIncorporatedConsumer(consumer func(block *model.Block))
}
