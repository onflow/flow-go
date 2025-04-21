package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestExecutionReceiptID_Malleability confirms that ExecutionReceipt and ExecutionReceiptStub, which implement
// the [flow.IDEntity] interface, are resistant to tampering.
func TestExecutionReceiptID_Malleability(t *testing.T) {
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithServiceEvents(3))),
		unittest.WithSpocks(unittest.SignaturesFixture(3)),
	)
	receiptMeta := receipt.Stub()
	// check ID of body used for signature
	unittest.RequireEntityNonMalleable(t, &receiptMeta.UnsignedExecutionReceiptStub)
	// check full ID used for indexing
	unittest.RequireEntityNonMalleable(t, receiptMeta)
	unittest.RequireEntityNonMalleable(t, receipt,
		unittest.WithFieldGenerator("UnsignedExecutionReceipt.ExecutionResult.ServiceEvents", func() []flow.ServiceEvent {
			return unittest.ServiceEventsFixture(3)
		}))
}

// TestExecutionReceiptGroupBy tests the GroupBy method of ExecutionReceiptList:
// * grouping should preserve order and multiplicity of elements
// * group for unknown identifier should be empty
func TestExecutionReceiptGroupBy(t *testing.T) {

	er1 := unittest.ExecutionReceiptFixture()
	er2 := unittest.ExecutionReceiptFixture()
	er3 := unittest.ExecutionReceiptFixture()

	idA := unittest.IdentifierFixture()
	idB := unittest.IdentifierFixture()
	grouperFunc := func(er *flow.ExecutionReceipt) flow.Identifier {
		switch er.ID() {
		case er1.ID():
			return idA
		case er2.ID():
			return idB
		case er3.ID():
			return idA
		default:
			panic("unexpected ExecutionReceipt")
		}
	}

	groups := flow.ExecutionReceiptList{er1, er2, er3, er1}.GroupBy(grouperFunc)
	assert.Equal(t, 2, groups.NumberGroups())
	assert.Equal(t, flow.ExecutionReceiptList{er1, er3, er1}, groups.GetGroup(idA))
	assert.Equal(t, flow.ExecutionReceiptList{er2}, groups.GetGroup(idB))

	unknown := groups.GetGroup(unittest.IdentifierFixture())
	assert.Equal(t, 0, unknown.Size())
}

// TestExecutionReceiptStubGroupBy tests the GroupBy method of ExecutionReceiptStubList:
// * grouping should preserve order and multiplicity of elements
// * group for unknown identifier should be empty
func TestExecutionReceiptStubGroupBy(t *testing.T) {

	er1 := unittest.ExecutionReceiptFixture().Stub()
	er2 := unittest.ExecutionReceiptFixture().Stub()
	er3 := unittest.ExecutionReceiptFixture().Stub()

	idA := unittest.IdentifierFixture()
	idB := unittest.IdentifierFixture()
	grouperFunc := func(er *flow.ExecutionReceiptStub) flow.Identifier {
		switch er.ID() {
		case er1.ID():
			return idA
		case er2.ID():
			return idB
		case er3.ID():
			return idA
		default:
			panic("unexpected ExecutionReceipt")
		}
	}

	groups := flow.ExecutionReceiptStubList{er1, er2, er3, er1}.GroupBy(grouperFunc)
	assert.Equal(t, 2, groups.NumberGroups())
	assert.Equal(t, flow.ExecutionReceiptStubList{er1, er3, er1}, groups.GetGroup(idA))
	assert.Equal(t, flow.ExecutionReceiptStubList{er2}, groups.GetGroup(idB))

	unknown := groups.GetGroup(unittest.IdentifierFixture())
	assert.Equal(t, 0, unknown.Size())
}
