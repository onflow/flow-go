package flow

import (
	"github.com/onflow/flow-go/crypto"
)

type Spock []byte

// ExecutionReceipt is the full execution receipt, as sent by the Execution Node.
// Specifically, it contains the detailed execution result.
type ExecutionReceipt struct {
	ExecutorID        Identifier
	ExecutionResult   ExecutionResult
	Spocks            []crypto.Signature
	ExecutorSignature crypto.Signature
}

// ID returns the canonical ID of the execution receipt.
func (er *ExecutionReceipt) ID() Identifier {
	return er.Meta().ID()
}

// Checksum returns a checksum for the execution receipt including the signatures.
func (er *ExecutionReceipt) Checksum() Identifier {
	return MakeID(er)
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

// ExecutionReceiptMeta contains the fields from the Execution Receipts
// that vary from one executor to another (assuming they commit to the same
// result). It only contains the ID (cryptographic hash) of the execution
// result the receipt commits to. The ExecutionReceiptMeta is useful for
// storing results and receipts separately in a composable way.
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

// ID returns the canonical ID of the execution receipt.
// It is identical to the ID of the full receipt.
func (er *ExecutionReceiptMeta) ID() Identifier {
	body := struct {
		ExecutorID Identifier
		ResultID   Identifier
		Spocks     []crypto.Signature
	}{
		ExecutorID: er.ExecutorID,
		ResultID:   er.ResultID,
		Spocks:     er.Spocks,
	}
	return MakeID(body)
}

// Checksum returns a checksum for the execution receipt including the signatures.
func (er *ExecutionReceiptMeta) Checksum() Identifier {
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
	return g[groupID]
}

// NumberGroups returns the number of groups
func (g ExecutionReceiptGroupedList) NumberGroups() int {
	return len(g)
}
