package message

// TestMessage is used for testing the network layer.
type TestMessage struct {
	Text string
}

func (t *TestMessage) ToInternal() (any, error) {
	return t, nil
}
