package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestIncorporatedResultGroupBy tests the GroupBy method:
// * grouping should preserve order and multiplicity of elements
// * group for unknown identifier should be empty
func TestIncorporatedResultGroupBy(t *testing.T) {
	ir1, err := flow.NewIncorporatedResult(flow.UntrustedIncorporatedResult{
		IncorporatedBlockID: unittest.IdentifierFixture(),
		Result:              unittest.ExecutionResultFixture(),
	})
	require.NoError(t, err)

	ir2, err := flow.NewIncorporatedResult(flow.UntrustedIncorporatedResult{
		IncorporatedBlockID: unittest.IdentifierFixture(),
		Result:              unittest.ExecutionResultFixture(),
	})
	require.NoError(t, err)

	ir3, err := flow.NewIncorporatedResult(flow.UntrustedIncorporatedResult{
		IncorporatedBlockID: unittest.IdentifierFixture(),
		Result:              unittest.ExecutionResultFixture(),
	})
	require.NoError(t, err)

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

// TestIncorporatedResultID_Malleability confirms that the IncorporatedResult struct, which implements
// the [flow.IDEntity] interface, is resistant to tampering.
func TestIncorporatedResultID_Malleability(t *testing.T) {
	incorporatedResult, err := flow.NewIncorporatedResult(flow.UntrustedIncorporatedResult{
		IncorporatedBlockID: unittest.IdentifierFixture(),
		Result:              unittest.ExecutionResultFixture(),
	})
	require.NoError(t, err)
	unittest.RequireEntityNonMalleable(t,
		incorporatedResult,
		unittest.WithFieldGenerator("Result.ServiceEvents", func() []flow.ServiceEvent {
			return unittest.ServiceEventsFixture(3)
		}))
}

// TestNewIncorporatedResult verifies that NewIncorporatedResult constructs a valid
// IncorporatedResult when given complete, non-zero fields, and returns an error
// if any required field is missing.
// It covers:
//   - valid incorporated result creation
//   - missing IncorporatedBlockID
//   - nil Result
func TestNewIncorporatedResult(t *testing.T) {
	t.Run("valid untrusted incorporated result", func(t *testing.T) {
		id := unittest.IdentifierFixture()
		// Use a real ExecutionResult fixture and take its address
		er := unittest.ExecutionResultFixture()
		uc := flow.UntrustedIncorporatedResult{
			IncorporatedBlockID: id,
			Result:              er,
		}

		ir, err := flow.NewIncorporatedResult(uc)
		assert.NoError(t, err)
		assert.NotNil(t, ir)
		assert.Equal(t, id, ir.IncorporatedBlockID)
		assert.Equal(t, er, ir.Result)
	})

	t.Run("missing IncorporatedBlockID", func(t *testing.T) {
		er := unittest.ExecutionResultFixture()
		uc := flow.UntrustedIncorporatedResult{
			IncorporatedBlockID: flow.ZeroID,
			Result:              er,
		}

		ir, err := flow.NewIncorporatedResult(uc)
		assert.Error(t, err)
		assert.Nil(t, ir)
		assert.Contains(t, err.Error(), "IncorporatedBlockID")
	})

	t.Run("nil Result", func(t *testing.T) {
		id := unittest.IdentifierFixture()
		uc := flow.UntrustedIncorporatedResult{
			IncorporatedBlockID: id,
			Result:              nil,
		}

		ir, err := flow.NewIncorporatedResult(uc)
		assert.Error(t, err)
		assert.Nil(t, ir)
		assert.Contains(t, err.Error(), "Result")
	})
}
