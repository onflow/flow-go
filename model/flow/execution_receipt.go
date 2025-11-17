package flow

import (
	"encoding/json"
	"fmt"

	"github.com/onflow/crypto"
)

type Spock []byte

// ExecutionReceipt is the full execution receipt, as sent by the Execution Node.
// Specifically, it contains the detailed execution result. The `ExecutorSignature`
// signs the `UnsignedExecutionReceipt`.
//
//structwrite:immutable - mutations allowed only within the constructor
type ExecutionReceipt struct {
	UnsignedExecutionReceipt
	ExecutorSignature crypto.Signature
}

// UntrustedExecutionReceipt is an untrusted input-only representation of a ExecutionReceipt,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedExecutionReceipt should be validated and converted into
// a trusted ExecutionReceipt using NewExecutionReceipt constructor.
type UntrustedExecutionReceipt ExecutionReceipt

// NewExecutionReceipt creates a new instance of ExecutionReceipt.
// Construction ExecutionReceipt allowed only within the constructor.
//
// All errors indicate a valid ExecutionReceipt cannot be constructed from the input.
func NewExecutionReceipt(untrusted UntrustedExecutionReceipt) (*ExecutionReceipt, error) {
	unsignedExecutionReceipt, err := NewUnsignedExecutionReceipt(UntrustedUnsignedExecutionReceipt(untrusted.UnsignedExecutionReceipt))
	if err != nil {
		return nil, fmt.Errorf("invalid unsigned execution receipt: %w", err)
	}
	if len(untrusted.ExecutorSignature) == 0 {
		return nil, fmt.Errorf("executor signature must not be empty")
	}
	return &ExecutionReceipt{
		UnsignedExecutionReceipt: *unsignedExecutionReceipt,
		ExecutorSignature:        untrusted.ExecutorSignature,
	}, nil
}

// ID returns the canonical ID of the execution receipt.
func (er *ExecutionReceipt) ID() Identifier {
	return er.Stub().ID()
}

// Stub returns a stub of the full ExecutionReceipt, where the ExecutionResult is replaced by its cryptographic hash.
func (er *ExecutionReceipt) Stub() *ExecutionReceiptStub {
	// Constructor is skipped since we're using an already-valid ExecutionReceipt object.
	//nolint:structwrite
	return &ExecutionReceiptStub{
		UnsignedExecutionReceiptStub: *er.UnsignedExecutionReceipt.Stub(),
		ExecutorSignature:            er.ExecutorSignature,
	}
}

// UnsignedExecutionReceipt represents the unsigned execution receipt, whose contents the
// Execution Node testifies to be correct by its signature.
//
//structwrite:immutable - mutations allowed only within the constructor
type UnsignedExecutionReceipt struct {
	ExecutorID Identifier
	ExecutionResult
	Spocks []crypto.Signature
}

// UntrustedUnsignedExecutionReceipt is an untrusted input-only representation of a UnsignedExecutionReceipt,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedUnsignedExecutionReceipt should be validated and converted into
// a trusted UnsignedExecutionReceipt using NewUnsignedExecutionReceipt constructor.
type UntrustedUnsignedExecutionReceipt UnsignedExecutionReceipt

// NewUnsignedExecutionReceipt creates a new instance of UnsignedExecutionReceipt.
// Construction UnsignedExecutionReceipt allowed only within the constructor.
//
// All errors indicate a valid UnsignedExecutionReceipt cannot be constructed from the input.
func NewUnsignedExecutionReceipt(untrusted UntrustedUnsignedExecutionReceipt) (*UnsignedExecutionReceipt, error) {
	if untrusted.ExecutorID == ZeroID {
		return nil, fmt.Errorf("executor ID must not be zero")
	}
	executionResult, err := NewExecutionResult(UntrustedExecutionResult(untrusted.ExecutionResult))
	if err != nil {
		return nil, fmt.Errorf("invalid execution result: %w", err)
	}
	if len(untrusted.Spocks) == 0 {
		return nil, fmt.Errorf("spocks must not be empty")
	}
	return &UnsignedExecutionReceipt{
		ExecutorID:      untrusted.ExecutorID,
		ExecutionResult: *executionResult,
		Spocks:          untrusted.Spocks,
	}, nil
}

// ID returns a hash over the data of the execution receipt.
// This is what is signed by the executor and verified by recipients.
// Necessary to override ExecutionResult.ID().
func (erb UnsignedExecutionReceipt) ID() Identifier {
	return erb.Stub().ID()
}

// Stub returns a stub of the UnsignedExecutionReceipt, where the ExecutionResult is replaced by its cryptographic hash.
func (erb UnsignedExecutionReceipt) Stub() *UnsignedExecutionReceiptStub {
	// Constructor is skipped since we're using an already-valid UnsignedExecutionReceipt object.
	//nolint:structwrite
	return &UnsignedExecutionReceiptStub{
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
//
//structwrite:immutable - mutations allowed only within the constructor
type ExecutionReceiptStub struct {
	UnsignedExecutionReceiptStub
	ExecutorSignature crypto.Signature
}

// UntrustedExecutionReceiptStub is an untrusted input-only representation of a ExecutionReceiptStub,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedExecutionReceiptStub should be validated and converted into
// a trusted ExecutionReceiptStub using NewExecutionReceiptStub constructor.
type UntrustedExecutionReceiptStub ExecutionReceiptStub

// NewExecutionReceiptStub creates a new instance of ExecutionReceiptStub.
// Construction ExecutionReceiptStub allowed only within the constructor.
//
// All errors indicate a valid ExecutionReceiptStub cannot be constructed from the input.
func NewExecutionReceiptStub(untrusted UntrustedExecutionReceiptStub) (*ExecutionReceiptStub, error) {
	unsignedExecutionReceiptStub, err := NewUnsignedExecutionReceiptStub(UntrustedUnsignedExecutionReceiptStub(untrusted.UnsignedExecutionReceiptStub))
	if err != nil {
		return nil, fmt.Errorf("invalid unsigned execution receipt stub: %w", err)
	}
	if len(untrusted.ExecutorSignature) == 0 {
		return nil, fmt.Errorf("executor signature must not be empty")
	}
	return &ExecutionReceiptStub{
		UnsignedExecutionReceiptStub: *unsignedExecutionReceiptStub,
		ExecutorSignature:            untrusted.ExecutorSignature,
	}, nil
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

// ExecutionReceiptFromStub creates ExecutionReceipt from execution result and ExecutionReceiptStub.
// No errors are expected during normal operation.
func ExecutionReceiptFromStub(stub ExecutionReceiptStub, result ExecutionResult) (*ExecutionReceipt, error) {
	unsignedExecutionReceipt, err := NewUnsignedExecutionReceipt(
		UntrustedUnsignedExecutionReceipt{
			ExecutorID:      stub.ExecutorID,
			ExecutionResult: result,
			Spocks:          stub.Spocks,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("could not construct unsigned execution receipt: %w", err)
	}

	executionReceipt, err := NewExecutionReceipt(
		UntrustedExecutionReceipt{
			UnsignedExecutionReceipt: *unsignedExecutionReceipt,
			ExecutorSignature:        stub.ExecutorSignature,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("could not construct execution receipt: %w", err)
	}

	return executionReceipt, nil
}

// UnsignedExecutionReceiptStub contains the fields of ExecutionReceiptStub that are signed by the executor.
//
//structwrite:immutable - mutations allowed only within the constructor
type UnsignedExecutionReceiptStub struct {
	ExecutorID Identifier
	ResultID   Identifier
	Spocks     []crypto.Signature
}

// UntrustedUnsignedExecutionReceiptStub is an untrusted input-only representation of a UnsignedExecutionReceiptStub,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedUnsignedExecutionReceiptStub should be validated and converted into
// a trusted UnsignedExecutionReceiptStub using NewUnsignedExecutionReceiptStub constructor.
type UntrustedUnsignedExecutionReceiptStub UnsignedExecutionReceiptStub

// NewUnsignedExecutionReceiptStub creates a new instance of UnsignedExecutionReceiptStub.
// Construction UnsignedExecutionReceiptStub allowed only within the constructor.
//
// All errors indicate a valid UnsignedExecutionReceiptStub cannot be constructed from the input.
func NewUnsignedExecutionReceiptStub(untrusted UntrustedUnsignedExecutionReceiptStub) (*UnsignedExecutionReceiptStub, error) {
	if untrusted.ExecutorID == ZeroID {
		return nil, fmt.Errorf("executor ID must not be zero")
	}
	if untrusted.ResultID == ZeroID {
		return nil, fmt.Errorf("result ID must not be zero")
	}
	if len(untrusted.Spocks) == 0 {
		return nil, fmt.Errorf("spocks must not be empty")
	}
	return &UnsignedExecutionReceiptStub{
		ExecutorID: untrusted.ExecutorID,
		ResultID:   untrusted.ResultID,
		Spocks:     untrusted.Spocks,
	}, nil
}

// ID returns cryptographic hash of unsigned execution receipt.
// This is what is signed by the executor and verified by recipients.
// It is identical to the ID of the full UnsignedExecutionReceipt.
func (erb UnsignedExecutionReceiptStub) ID() Identifier {
	return MakeID(erb)
}

/*******************************************************************************
GROUPING for full ExecutionReceipts:
allows to split a list of receipts by some property
*******************************************************************************/

// ExecutionReceiptList is a slice of ExecutionReceipts with the additional
// functionality to group receipts by various properties
type ExecutionReceiptList []*ExecutionReceipt

// ExecutionReceiptGroupedList is a partition of an ExecutionReceiptList.
// The map key is a generic Identifier whose meaning is defined by the grouping
// function used to build the groups. For example:
//   - With GroupByExecutorID, the key is the receipt's ExecutorID (node ID).
//   - With GroupByResultID, the key is the Execution Result ID.
//
// Custom groupers may choose other identifiers. Within each group, order and
// multiplicity of receipts are preserved.
//
// Note: This type itself does not prescribe what the key represents; it is
// solely determined by the ExecutionReceiptGroupingFunction.
//
// Example:
//
//	groups := list.GroupByExecutorID()
//	receiptsFromExecutor := groups[executorID]
//
// Use GetGroup to safely obtain a group's receipts if the key may be absent.
//
// ExecutionReceiptGroupedList maps a group Identifier to the corresponding
// receipts for that group.
type ExecutionReceiptGroupedList map[Identifier]ExecutionReceiptList

// ExecutionReceiptGroupingFunction is a function that assigns an identifier to each receipt
type ExecutionReceiptGroupingFunction func(*ExecutionReceipt) Identifier

// GroupBy partitions the ExecutionReceiptList. All receipts that are mapped
// by the provided grouping function to the same Identifier (the map key) are
// placed in the same group. The meaning of the key is entirely determined by
// the grouper (e.g., ExecutorID or Execution Result ID). Within each group, the
// order and multiplicity of the receipts is preserved.
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

// Stubs converts the ExecutionReceiptList to an ExecutionReceiptStubList
func (l ExecutionReceiptList) Stubs() ExecutionReceiptStubList {
	stubs := make(ExecutionReceiptStubList, len(l))
	for i, receipt := range l {
		stubs[i] = receipt.Stub()
	}
	return stubs
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
