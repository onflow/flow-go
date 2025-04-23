package flow

import (
	"encoding/json"

	"github.com/onflow/crypto"
)

type Spock []byte

// ExecutionReceipt is the full execution receipt, as sent by the Execution Node.
// Specifically, it contains the detailed execution result. The `ExecutorSignature`
// signs the `ExecutionReceiptBody`.
type ExecutionReceipt struct {
	ExecutionReceiptBody
	ExecutorSignature crypto.Signature
}

// ExecutionReceiptBody represents the unsigned execution receipt, whose contents the
// Execution Node testifies to be correct by its signature.
type ExecutionReceiptBody struct {
	ExecutorID Identifier
	ExecutionResult
	Spocks []crypto.Signature
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
		ExecutionReceiptMetaBody: er.ExecutionReceiptBody.Meta(),
		ExecutorSignature:        er.ExecutorSignature,
	}
}

// ID returns a hash over the data of the execution receipt.
// This is what is signed by the executor and verified by recipients.
// Necessary to override ExecutionResult.ID().
func (erb ExecutionReceiptBody) ID() Identifier {
	return erb.Meta().ID()
}

func (erb ExecutionReceiptBody) Meta() ExecutionReceiptMetaBody {
	return ExecutionReceiptMetaBody{
		ExecutorID: erb.ExecutorID,
		ResultID:   erb.ExecutionResult.ID(),
		Spocks:     erb.Spocks,
	}
}

// ExecutionReceiptMeta contains the fields from the Execution Receipts
// that vary from one executor to another (assuming they commit to the same
// result). It only contains the ID (cryptographic hash) of the execution
// result the receipt commits to. The ExecutionReceiptMeta is useful for
// storing results and receipts separately in a composable way.
type ExecutionReceiptMeta struct {
	ExecutionReceiptMetaBody
	ExecutorSignature crypto.Signature
}

// ExecutionReceiptMetaBody contains the fields of ExecutionReceiptMeta that are signed by the executor.
type ExecutionReceiptMetaBody struct {
	ExecutorID Identifier
	ResultID   Identifier
	Spocks     []crypto.Signature
}

func ExecutionReceiptFromMeta(meta ExecutionReceiptMeta, result ExecutionResult) *ExecutionReceipt {
	return &ExecutionReceipt{
		ExecutionReceiptBody: ExecutionReceiptBody{
			ExecutorID:      meta.ExecutorID,
			ExecutionResult: result,
			Spocks:          meta.Spocks,
		},
		ExecutorSignature: meta.ExecutorSignature,
	}
}

// ID returns cryptographic hash of unsigned execution receipt.
// This is what is signed by the executor and verified by recipients.
// It is identical to the ID of the full receipt's body.
func (erb ExecutionReceiptMetaBody) ID() Identifier {
	return MakeID(erb)
}

// ID returns the canonical ID of the execution receipt.
// It is identical to the ID of the full receipt.
func (er *ExecutionReceiptMeta) ID() Identifier {
	return MakeID(er)
}

func (er ExecutionReceiptMeta) MarshalJSON() ([]byte, error) {
	type Alias ExecutionReceiptMeta
	return json.Marshal(struct {
		Alias
		ID string
	}{
		Alias: Alias(er),
		ID:    er.ID().String(),
	})
}

// Checksum returns a checksum for the execution receipt including the signatures.
func (er *ExecutionReceiptMeta) Checksum() Identifier {
	return MakeID(er)
}

/*******************************************************************************
GROUPING for full ExecutionReceipts:
allows to split a list of receipts by some property
*******************************************************************************/

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

/*******************************************************************************
GROUPING for ExecutionReceiptMeta information:
allows to split a list of receipt meta information by some property
*******************************************************************************/

// ExecutionReceiptMetaList is a slice of ExecutionResultMetas with the additional
// functionality to group them by various properties
type ExecutionReceiptMetaList []*ExecutionReceiptMeta

// ExecutionReceiptMetaGroupedList is a partition of an ExecutionReceiptMetaList
type ExecutionReceiptMetaGroupedList map[Identifier]ExecutionReceiptMetaList

// ExecutionReceiptMetaGroupingFunction is a function that assigns an identifier to each receipt meta
type ExecutionReceiptMetaGroupingFunction func(*ExecutionReceiptMeta) Identifier

// GroupBy partitions the ExecutionReceiptMetaList. All receipts that are mapped
// by the grouping function to the same identifier are placed in the same group.
// Within each group, the order and multiplicity of the receipts is preserved.
func (l ExecutionReceiptMetaList) GroupBy(grouper ExecutionReceiptMetaGroupingFunction) ExecutionReceiptMetaGroupedList {
	groups := make(map[Identifier]ExecutionReceiptMetaList)
	for _, rcpt := range l {
		groupID := grouper(rcpt)
		groups[groupID] = append(groups[groupID], rcpt)
	}
	return groups
}

// GroupByExecutorID partitions the ExecutionReceiptMetaList by the receipts' ExecutorIDs.
// Within each group, the order and multiplicity of the receipts is preserved.
func (l ExecutionReceiptMetaList) GroupByExecutorID() ExecutionReceiptMetaGroupedList {
	grouper := func(receipt *ExecutionReceiptMeta) Identifier { return receipt.ExecutorID }
	return l.GroupBy(grouper)
}

// GroupByResultID partitions the ExecutionReceiptMetaList by the receipts' Result IDs.
// Within each group, the order and multiplicity of the receipts is preserved.
func (l ExecutionReceiptMetaList) GroupByResultID() ExecutionReceiptMetaGroupedList {
	grouper := func(receipt *ExecutionReceiptMeta) Identifier { return receipt.ResultID }
	return l.GroupBy(grouper)
}

// Size returns the number of receipts in the list
func (l ExecutionReceiptMetaList) Size() int {
	return len(l)
}

// GetGroup returns the receipts that were mapped to the same identifier by the
// grouping function. Returns an empty (nil) ExecutionReceiptMetaList if groupID does not exist.
func (g ExecutionReceiptMetaGroupedList) GetGroup(groupID Identifier) ExecutionReceiptMetaList {
	return g[groupID]
}

// NumberGroups returns the number of groups
func (g ExecutionReceiptMetaGroupedList) NumberGroups() int {
	return len(g)
}

// Lookup generates a map from ExecutionReceipt ID to ExecutionReceiptMeta
func (l ExecutionReceiptMetaList) Lookup() map[Identifier]*ExecutionReceiptMeta {
	receiptsByID := make(map[Identifier]*ExecutionReceiptMeta, len(l))
	for _, receipt := range l {
		receiptsByID[receipt.ID()] = receipt
	}
	return receiptsByID
}
