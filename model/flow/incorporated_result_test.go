package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestIncorporatedResultGroupBy tests the GroupBy method:
// * grouping should preserve order and multiplicity of elements
// * group for unknown identifier should be empty
func TestIncorporatedResultGroupBy(t *testing.T) {

	ir1 := flow.NewIncorporatedResult(unittest.IdentifierFixture(), unittest.ExecutionResultFixture())
	ir2 := flow.NewIncorporatedResult(unittest.IdentifierFixture(), unittest.ExecutionResultFixture())
	ir3 := flow.NewIncorporatedResult(unittest.IdentifierFixture(), unittest.ExecutionResultFixture())

	idA := unittest.IdentifierFixture()
	idB := unittest.IdentifierFixture()
	grouperFunc := func(ir *flow.IncorporatedResult) flow.Identifier {
		switch ir.ID() {
		case ir1.ID():
			return idA
		case ir2.ID():
			return idB
		case ir3.ID():
			return idA
		default:
			panic("unexpected IncorporatedResult")
		}
	}

	groups := flow.IncorporatedResultList{ir1, ir2, ir3, ir1}.GroupBy(grouperFunc)
	assert.Equal(t, 2, groups.NumberGroups())
	assert.Equal(t, flow.IncorporatedResultList{ir1, ir3, ir1}, groups.GetGroup(idA))
	assert.Equal(t, flow.IncorporatedResultList{ir2}, groups.GetGroup(idB))

	unknown := groups.GetGroup(unittest.IdentifierFixture())
	assert.Equal(t, 0, unknown.Size())
}
