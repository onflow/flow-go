package flow

import (
	"encoding/json"

	"github.com/onflow/crypto"
)

type Spock []byte

// ExecutionReceipt is the full execution receipt, as sent by the Execution Node.
// Specifically, it contains the detailed execution result. The `ExecutorSignature`
// signs the `UnsignedExecutionReceipt`.
type ExecutionReceipt struct {
	UnsignedExecutionReceipt
	ExecutorSignature crypto.Signature
}

// UnsignedExecutionReceipt represents the unsigned execution receipt, whose contents the
// Execution Node testifies to be correct by its signature.
type UnsignedExecutionReceipt struct {
	ExecutorID Identifier
	ExecutionResult
	Spocks []crypto.Signature
}

// ID returns the canonical ID of the execution receipt.
func (er *ExecutionReceipt) ID() Identifier {
	return er.Stub().ID()
}

// Checksum returns a checksum for the execution receipt including the signatures.
func (er *ExecutionReceipt) Checksum() Identifier {
	return MakeID(er)
}

// Stub returns a shortened version of receipt, only containing the ResultID instead of the full ExecutionResult.
func (er *ExecutionReceipt) Stub() *ExecutionReceiptStub {
	return &ExecutionReceiptStub{
		UnsignedExecutionReceiptStub: er.UnsignedExecutionReceipt.Stub(),
		ExecutorSignature:            er.ExecutorSignature,
	}
}

// ID returns a hash over the data of the execution receipt.
// This is what is signed by the executor and verified by recipients.
// Necessary to override ExecutionResult.ID().
func (erb UnsignedExecutionReceipt) ID() Identifier {
	return erb.Stub().ID()
}

func (erb UnsignedExecutionReceipt) Stub() UnsignedExecutionReceiptStub {
	return UnsignedExecutionReceiptStub{
		ExecutorID: erb.ExecutorID,
		ResultID:   erb.ExecutionResult.ID(),
		Spocks:     erb.Spocks,
	}
}

// ExecutionReceiptStub contains the fields from the Execution Receipts
// that vary from one executor to another (assuming they commit to the same
// result). It only contains the ID (cryptographic hash) of the execution
// result the receipt commits to. The ExecutionReceiptStub is useful for
// storing results and receipts separately in a composable way.
type ExecutionReceiptStub struct {
	UnsignedExecutionReceiptStub
	ExecutorSignature crypto.Signature
}

// UnsignedExecutionReceiptStub contains the fields of ExecutionReceiptStub that are signed by the executor.
type UnsignedExecutionReceiptStub struct {
	ExecutorID Identifier
	ResultID   Identifier
	Spocks     []crypto.Signature
}

func ExecutionReceiptFromStub(stub ExecutionReceiptStub, result ExecutionResult) *ExecutionReceipt {
	return &ExecutionReceipt{
		UnsignedExecutionReceipt: UnsignedExecutionReceipt{
			ExecutorID:      stub.ExecutorID,
			ExecutionResult: result,
			Spocks:          stub.Spocks,
		},
		ExecutorSignature: stub.ExecutorSignature,
	}
}

// ID returns cryptographic hash of unsigned execution receipt.
// This is what is signed by the executor and verified by recipients.
// It is identical to the ID of the full UnsignedExecutionReceipt.
func (erb UnsignedExecutionReceiptStub) ID() Identifier {
	return MakeID(erb)
}

// ID returns the canonical ID of the execution receipt.
// It is identical to the ID of the full receipt.
func (er *ExecutionReceiptStub) ID() Identifier {
	return MakeID(er)
}

func (er ExecutionReceiptStub) MarshalJSON() ([]byte, error) {
	type Alias ExecutionReceiptStub
	return json.Marshal(struct {
		Alias
		ID string
	}{
		Alias: Alias(er),
		ID:    er.ID().String(),
	})
}

// Checksum returns a checksum for the execution receipt including the signatures.
func (er *ExecutionReceiptStub) Checksum() Identifier {
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
GROUPING for ExecutionReceiptStub information:
allows to split a list of receipt stub information by some property
*******************************************************************************/

// ExecutionReceiptStubList is a slice of ExecutionResultStubs with the additional
// functionality to group them by various properties
type ExecutionReceiptStubList []*ExecutionReceiptStub

// ExecutionReceiptStubGroupedList is a partition of an ExecutionReceiptStubList
type ExecutionReceiptStubGroupedList map[Identifier]ExecutionReceiptStubList

// ExecutionReceiptStubGroupingFunction is a function that assigns an identifier to each receipt stub
type ExecutionReceiptStubGroupingFunction func(*ExecutionReceiptStub) Identifier

// GroupBy partitions the ExecutionReceiptStubList. All receipts that are mapped
// by the grouping function to the same identifier are placed in the same group.
// Within each group, the order and multiplicity of the receipts is preserved.
func (l ExecutionReceiptStubList) GroupBy(grouper ExecutionReceiptStubGroupingFunction) ExecutionReceiptStubGroupedList {
	groups := make(map[Identifier]ExecutionReceiptStubList)
	for _, rcpt := range l {
		groupID := grouper(rcpt)
		groups[groupID] = append(groups[groupID], rcpt)
	}
	return groups
}

// GroupByExecutorID partitions the ExecutionReceiptStubList by the receipts' ExecutorIDs.
// Within each group, the order and multiplicity of the receipts is preserved.
func (l ExecutionReceiptStubList) GroupByExecutorID() ExecutionReceiptStubGroupedList {
	grouper := func(receipt *ExecutionReceiptStub) Identifier { return receipt.ExecutorID }
	return l.GroupBy(grouper)
}

// GroupByResultID partitions the ExecutionReceiptStubList by the receipts' Result IDs.
// Within each group, the order and multiplicity of the receipts is preserved.
func (l ExecutionReceiptStubList) GroupByResultID() ExecutionReceiptStubGroupedList {
	grouper := func(receipt *ExecutionReceiptStub) Identifier { return receipt.ResultID }
	return l.GroupBy(grouper)
}

// Size returns the number of receipts in the list
func (l ExecutionReceiptStubList) Size() int {
	return len(l)
}

// GetGroup returns the receipts that were mapped to the same identifier by the
// grouping function. Returns an empty (nil) ExecutionReceiptStubList if groupID does not exist.
func (g ExecutionReceiptStubGroupedList) GetGroup(groupID Identifier) ExecutionReceiptStubList {
	return g[groupID]
}

// NumberGroups returns the number of groups
func (g ExecutionReceiptStubGroupedList) NumberGroups() int {
	return len(g)
}

// Lookup generates a map from ExecutionReceipt ID to ExecutionReceiptStub
func (l ExecutionReceiptStubList) Lookup() map[Identifier]*ExecutionReceiptStub {
	receiptsByID := make(map[Identifier]*ExecutionReceiptStub, len(l))
	for _, receipt := range l {
		receiptsByID[receipt.ID()] = receipt
	}
	return receiptsByID
}
