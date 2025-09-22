package message

import "github.com/onflow/flow-go/model/flow"

// TestMessage is the untrusted network-level representation of a test message.
//
// This type exists purely for exercising the network layer in tests and carries
// no guarantees about structural validity. It implements messages.UntrustedMessage interface so
// it can be transmitted and decoded like any other message type.
//
// Use ToInternal to convert this to the internal flow.TestMessage before
// consuming it inside the node.
type TestMessage flow.TestMessage

// ToInternal converts the untrusted TestMessage into its trusted internal
// representation.
//
// Since TestMessage is only used for testing, the conversion is effectively
// a passthrough that wraps the network-level data in a flow.TestMessage.
// In production message types, this method would enforce structural validity
// through flow constructors.
func (t *TestMessage) ToInternal() (any, error) {
	return &flow.TestMessage{
		Text: t.Text,
	}, nil
}
