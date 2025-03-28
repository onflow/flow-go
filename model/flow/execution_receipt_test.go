package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestExecutionReceiptID_Malleability confirms that ExecutionReceipt and ExecutionReceiptMeta, which implement
// the [flow.IDEntity] interface, are resistant to tampering.
func TestExecutionReceiptID_Malleability(t *testing.T) {
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithServiceEvents(3))),
		unittest.WithSpocks(unittest.SignaturesFixture(3)),
	)
	unittest.RequireEntityNonMalleable(t, receipt.Meta())
	unittest.RequireEntityNonMalleable(t, receipt,
		unittest.WithFieldGenerator("ExecutionResult.ServiceEvents", func() []flow.ServiceEvent {
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

// TestExecutionReceiptMetaGroupBy tests the GroupBy method of ExecutionReceiptMetaList:
// * grouping should preserve order and multiplicity of elements
// * group for unknown identifier should be empty
func TestExecutionReceiptMetaGroupBy(t *testing.T) {

	er1 := unittest.ExecutionReceiptFixture().Meta()
	er2 := unittest.ExecutionReceiptFixture().Meta()
	er3 := unittest.ExecutionReceiptFixture().Meta()

	idA := unittest.IdentifierFixture()
	idB := unittest.IdentifierFixture()
	grouperFunc := func(er *flow.ExecutionReceiptMeta) flow.Identifier {
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

	groups := flow.ExecutionReceiptMetaList{er1, er2, er3, er1}.GroupBy(grouperFunc)
	assert.Equal(t, 2, groups.NumberGroups())
	assert.Equal(t, flow.ExecutionReceiptMetaList{er1, er3, er1}, groups.GetGroup(idA))
	assert.Equal(t, flow.ExecutionReceiptMetaList{er2}, groups.GetGroup(idB))

	unknown := groups.GetGroup(unittest.IdentifierFixture())
	assert.Equal(t, 0, unknown.Size())
}
