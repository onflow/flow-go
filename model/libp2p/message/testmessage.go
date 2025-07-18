package message

// TestMessage is used for testing the network layer.
//
//structwrite:immutable
type TestMessage struct {
	Text string
}

// UntrustedTestMessage is an untrusted input-only representation of a TestMessage,
// used for construction.
//
// An instance of UntrustedTestMessage should be validated and converted into
// a trusted TestMessage using NewTestMessage constructor.
type UntrustedTestMessage TestMessage

// NewTestMessage creates a new TestMessage.
//
// Parameters:
//   - untrusted: untrusted TestMessage to be validated
//
// Returns:
//   - TestMessage: the newly created instance
//   - error: any error that occurred during creation
//
// Expected Errors:
//   - TODO: add validation errors
func NewTestMessage(untrusted UntrustedTestMessage) (TestMessage, error) {
	// TODO: add validation logic
	return TestMessage{Text: untrusted.Text}, nil
}
