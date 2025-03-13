package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertExecutionReceiptMeta inserts an execution receipt meta by ID.
func InsertExecutionReceiptMeta(w storage.Writer, receiptID flow.Identifier, meta *flow.ExecutionReceiptMeta) error {
	return UpsertByKey(w, MakePrefix(codeExecutionReceiptMeta, receiptID), meta)
}

// RetrieveExecutionReceiptMeta retrieves a execution receipt meta by ID.
func RetrieveExecutionReceiptMeta(r storage.Reader, receiptID flow.Identifier, meta *flow.ExecutionReceiptMeta) error {
	return RetrieveByKey(r, MakePrefix(codeExecutionReceiptMeta, receiptID), meta)
}

// IndexOwnExecutionReceipt inserts an execution receipt ID keyed by block ID
func IndexOwnExecutionReceipt(w storage.Writer, blockID flow.Identifier, receiptID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeOwnBlockReceipt, blockID), receiptID)
}

// LookupOwnExecutionReceipt finds execution receipt ID by block
func LookupOwnExecutionReceipt(r storage.Reader, blockID flow.Identifier, receiptID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeOwnBlockReceipt, blockID), receiptID)
}

// RemoveOwnExecutionReceipt removes own execution receipt index by blockID
func RemoveOwnExecutionReceipt(w storage.Writer, blockID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeOwnBlockReceipt, blockID))
}

// IndexExecutionReceipts inserts an execution receipt ID keyed by block ID and receipt ID.
// one block could have multiple receipts, even if they are from the same executor
func IndexExecutionReceipts(w storage.Writer, blockID, receiptID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeAllBlockReceipts, blockID, receiptID), receiptID)
}

// LookupExecutionReceipts finds all execution receipts by block ID
func LookupExecutionReceipts(r storage.Reader, blockID flow.Identifier, receiptIDs *[]flow.Identifier) error {
	iterationFunc := receiptIterationFunc(receiptIDs)
	return TraverseByPrefix(r, MakePrefix(codeAllBlockReceipts, blockID), iterationFunc, storage.DefaultIteratorOptions())
}

// receiptIterationFunc returns an in iteration function which returns all receipt IDs found during traversal
func receiptIterationFunc(receiptIDs *[]flow.Identifier) func() (CheckFunc, CreateFunc, HandleFunc) {
	check := func(key []byte) (bool, error) {
		return true, nil
	}

	var receiptID flow.Identifier
	create := func() interface{} {
		return &receiptID
	}
	handle := func() error {
		*receiptIDs = append(*receiptIDs, receiptID)
		return nil
	}
	return func() (CheckFunc, CreateFunc, HandleFunc) {
		return check, create, handle
	}
}
