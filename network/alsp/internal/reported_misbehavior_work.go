package internal

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
)

const NonceSize = 8

// ReportedMisbehaviorWork is an internal data structure for "temporarily" storing misbehavior reports in the queue
// till they are processed by the worker.
type ReportedMisbehaviorWork struct {
	// Channel is the channel that the misbehavior report is about.
	Channel channels.Channel

	// OriginId is the ID of the peer that the misbehavior report is about.
	OriginId flow.Identifier

	// Reason is the reason of the misbehavior.
	Reason network.Misbehavior

	// Nonce is a random nonce value that is used to make the key of the struct unique in the queue even when
	// the same misbehavior report is reported multiple times. This is needed as we expect the same misbehavior report
	// to be reported multiple times when an attack persists for a while. We don't want to deduplicate the misbehavior
	// reports in the queue as we want to penalize the misbehaving node for each report.
	Nonce [NonceSize]byte

	// Penalty is the penalty value of the misbehavior.
	// We use `rlp:"-"` to ignore this field when serializing the struct to RLP to determine the key of this struct
	// when storing in the queue. Hence, the penalty value does "not" contribute to the key for storing in the queue.
	// As RLP encoding does not support float64, we cannot use this field as the key of the
	// struct. As we use a random nonce value for the key of the struct, we can be sure that we will not have a collision
	// in the queue, and duplicate reports will be accepted with unique keys.
	Penalty float64 `rlp:"-"`
}
