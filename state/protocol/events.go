package protocol

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Consumer defines a set of events that occur within the protocol state, that
// can be propagated to other components via an implementation of this interface.
// Consumer implementations must be non-blocking.
//
// TODO: this is only a striped-down version of Jordan's interface
// https://github.com/dapperlabs/flow-go/blob/706e05f6bea069f0aeb3d14e3607b1471b1cc16e/state/protocol/events.go
// Will need to consolidate this with Jordan's work
type Consumer interface {

	// BlockFinalized is called when a block is finalized.
	// Formally, this callback is informationally idempotent. I.e. the consumer
	// of this callback must handle repeated calls with the same header.
	BlockFinalized(block *flow.Header)

	// CorrectBlock is called when a correct block is encountered that is ready
	// to be processed (i.e. it is connected to the finalized chain and its
	// source of randomness is available).
	// Formally, this callback is informationally idempotent. I.e. the consumer
	// of this callback must handle repeated calls with the same header.
	CorrectBlock(block *flow.Header)
}
