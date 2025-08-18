package message

import "github.com/onflow/flow-go/model/messages"

// TestMessage is used for testing the network layer.
type TestMessage struct {
	Text string
}

var _ messages.UntrustedMessage = (*TestMessage)(nil)

func (tm *TestMessage) ToInternal() (any, error) {
	// Temporary: just return the unvalidated wire struct
	return tm, nil
}
