package query

import (
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// EntropyProviderPerBlock is an abstraction for entropy providers
// that can be used in `QueryExecutor`.
//
// `EntropyProvider` is defined in `fvm/environment` and abstracts the
// distributed random source used by the protocol.
//
// For a full-protocol node implementation , `EntropyProvider` is implemented
// by the protocol `Snapshot`, while `EntropyProviderPerBlock` is implemented
// by the protocol `State`.
// For nodes answering script queries that do not participate in the protocol,
// `EntropyProvider` and `EntropyProviderPerBlock` can be implemented by other
// components that provide the source of randomness for each block.
type EntropyProviderPerBlock interface {
	// AtBlockID returns an entropy provider at the given block ID.
	AtBlockID(blockID flow.Identifier) environment.EntropyProvider
}

// protocolStateWrapper implements `EntropyProviderPerBlock`
var _ EntropyProviderPerBlock = protocolStateWrapper{}

type protocolStateWrapper struct {
	protocol.State
}

// NewProtocolStateWrapper wraps a protocol.State input as an `EntropyProviderPerBlock`
func NewProtocolStateWrapper(s protocol.State) EntropyProviderPerBlock {
	return protocolStateWrapper{s}
}

func (p protocolStateWrapper) AtBlockID(blockID flow.Identifier) environment.EntropyProvider {
	return environment.EntropyProvider(p.State.AtBlockID(blockID))
}
