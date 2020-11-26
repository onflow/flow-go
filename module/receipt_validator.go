package module

import "github.com/onflow/flow-go/model/flow"

type ReceiptValidator interface {
	Validate(receipt *flow.ExecutionReceipt) error
}
