package flow

import (
	"github.com/onflow/flow-go/crypto"
)

type Spock []byte

// ExecutionReceiptMeta contains the metadata the distinguishes an execution
// receipt from an execution result. This is used for storing results and
// receipts separately in a composable way.
type ExecutionReceiptMeta struct {
	ExecutorID        Identifier
	ResultID          Identifier
	Spocks            []crypto.Signature
	ExecutorSignature crypto.Signature
}

func ExecutionReceiptFromMeta(meta ExecutionReceiptMeta, result ExecutionResult) *ExecutionReceipt {
	return &ExecutionReceipt{
		ExecutorID:        meta.ExecutorID,
		ExecutionResult:   result,
		Spocks:            meta.Spocks,
		ExecutorSignature: meta.ExecutorSignature,
	}
}

type ExecutionReceipt struct {
	ExecutorID        Identifier
	ExecutionResult   ExecutionResult
	Spocks            []crypto.Signature
	ExecutorSignature crypto.Signature
}

// Meta returns the receipt metadata for the receipt.
func (er *ExecutionReceipt) Meta() *ExecutionReceiptMeta {
	return &ExecutionReceiptMeta{
		ExecutorID:        er.ExecutorID,
		ResultID:          er.ExecutionResult.ID(),
		Spocks:            er.Spocks,
		ExecutorSignature: er.ExecutorSignature,
	}
}

// ID returns the canonical ID of the execution receipt.
func (er *ExecutionReceipt) ID() Identifier {
	body := struct {
		ExecutorID      Identifier
		ExecutionResult ExecutionResult
		Spocks          []crypto.Signature
	}{
		ExecutorID:      er.ExecutorID,
		ExecutionResult: er.ExecutionResult,
		Spocks:          er.Spocks,
	}
	return MakeID(body)
}

// Checksum returns a checksum for the execution receipt including the signatures.
func (er *ExecutionReceipt) Checksum() Identifier {
	return MakeID(er)
}

/* GROUPING allows to split a list or map of receipts by some property */

// ExecutionReceiptList is a slice of ExecutionReceipts with the additional
// functionality to group receipts by various properties
type ExecutionReceiptList []*ExecutionReceipt

// ExecutionReceiptGroupedList is a partition of an ExecutionReceiptList
type ExecutionReceiptGroupedList map[Identifier]ExecutionReceiptList

// ExecutionReceiptGroupingFunction is a function that assigns an identifier to each receipt
type ExecutionReceiptGroupingFunction func(*ExecutionReceipt) Identifier

// GroupBy partitions the ExecutionReceiptList. All receipts that are mapped
// by the grouping function to the same identifier are placed in the same group.
// Within each group, the order and multiplicity of the receipts is preserved.
func (l ExecutionReceiptList) GroupBy(grouper ExecutionReceiptGroupingFunction) ExecutionReceiptGroupedList {
	groups := make(map[Identifier]ExecutionReceiptList)
	for _, rcpt := range l {
		groupID := grouper(rcpt)
		groups[groupID] = append(groups[groupID], rcpt)
	}
	return groups
}

// GroupByExecutorID partitions the ExecutionReceiptList by the receipts' ExecutorIDs.
// Within each group, the order and multiplicity of the receipts is preserved.
func (l ExecutionReceiptList) GroupByExecutorID() ExecutionReceiptGroupedList {
	grouper := func(receipt *ExecutionReceipt) Identifier { return receipt.ExecutorID }
	return l.GroupBy(grouper)
}

// GroupByResultID partitions the ExecutionReceiptList by the receipts' Result IDs.
// Within each group, the order and multiplicity of the receipts is preserved.
func (l ExecutionReceiptList) GroupByResultID() ExecutionReceiptGroupedList {
	grouper := func(receipt *ExecutionReceipt) Identifier { return receipt.ExecutionResult.ID() }
	return l.GroupBy(grouper)
}

// Size returns the number of receipts in the list
func (l ExecutionReceiptList) Size() int {
	return len(l)
}

// GetGroup returns the receipts that were mapped to the same identifier by the
// grouping function. Returns an empty (nil) ExecutionReceiptList if groupID does not exist.
func (g ExecutionReceiptGroupedList) GetGroup(groupID Identifier) ExecutionReceiptList {
	//if g == nil {
	//	return nil
	//}
	return g[groupID]
}

// NumberGroups returns the number of groups
func (g ExecutionReceiptGroupedList) NumberGroups() int {
	return len(g)
}
