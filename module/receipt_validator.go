package module

import "github.com/onflow/flow-go/model/flow"

// ReceiptValidator is an interface which is used for validating
// receipts with respect to current protocol state.
// Returns the following errors:
// * nil - in case of success
// * sentinel engine.InvalidInputError when one receipt is not valid due to
// conflicting protocol state. This can happen when invalid receipt has been passed or
// there is not enough data to validate state.
// * exception in case of any other error, usually this is not expected.
type ReceiptValidator interface {
	Validate(receipts []*flow.ExecutionReceipt) error
}
