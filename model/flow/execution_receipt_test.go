package flow_test

import (
	"testing"

	"github.com/onflow/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// TestNewExecutionReceipt verifies the behavior of the NewExecutionReceipt constructor.
// It ensures proper handling of both valid and invalid untrusted input fields.
//
// Test Cases:
//
// 1. Valid input:
//   - Verifies that a properly populated UntrustedExecutionReceipt results in a valid ExecutionReceipt.
//
// 2. Invalid input with invalid UnsignedExecutionReceipt:
//   - Ensures an error is returned when the UnsignedExecutionReceipt.ExecutorID is flow.ZeroID.
//
// 3. Invalid input with nil ExecutorSignature:
//   - Ensures an error is returned when the ExecutorSignature is nil.
//
// 4. Invalid input with empty ExecutorSignature:
//   - Ensures an error is returned when the ExecutorSignature is an empty byte slice.
func TestNewExecutionReceipt(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		receipt := unittest.ExecutionReceiptFixture()
		res, err := flow.NewExecutionReceipt(flow.UntrustedExecutionReceipt(*receipt))
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("invalid input with invalid unsigned execution receipt", func(t *testing.T) {
		receipt := unittest.ExecutionReceiptFixture()
		receipt.UnsignedExecutionReceipt.ExecutorID = flow.ZeroID

		res, err := flow.NewExecutionReceipt(flow.UntrustedExecutionReceipt(*receipt))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "invalid unsigned execution receipt: executor ID must not be zero")
	})

	t.Run("invalid input with nil ExecutorSignature", func(t *testing.T) {
		receipt := unittest.ExecutionReceiptFixture()
		receipt.ExecutorSignature = nil

		res, err := flow.NewExecutionReceipt(flow.UntrustedExecutionReceipt(*receipt))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "executor signature must not be empty")
	})

	t.Run("invalid input with empty ExecutorSignature", func(t *testing.T) {
		receipt := unittest.ExecutionReceiptFixture()
		receipt.ExecutorSignature = []byte{}

		res, err := flow.NewExecutionReceipt(flow.UntrustedExecutionReceipt(*receipt))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "executor signature must not be empty")
	})
}

// TestNewUnsignedExecutionReceipt verifies the behavior of the NewUnsignedExecutionReceipt constructor.
// It ensures proper handling of both valid and invalid untrusted input fields.
//
// Test Cases:
//
// 1. Valid input:
//   - Verifies that a properly populated UntrustedUnsignedExecutionReceipt results in a valid UnsignedExecutionReceipt.
//
// 2. Invalid input with zero ExecutorID:
//   - Ensures an error is returned when the ExecutorID is flow.ZeroID.
//
// 3. Invalid input with invalid ExecutionResult:
//   - Ensures an error is returned when the ExecutionResult.BlockID is flow.ZeroID.
//
// 4. Invalid input with nil Spocks:
//   - Ensures an error is returned when the Spocks field is nil.
//
// 5. Invalid input with empty Spocks:
//   - Ensures an error is returned when the Spocks field is an empty slice.
func TestNewUnsignedExecutionReceipt(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		receipt := unittest.UnsignedExecutionReceiptFixture()
		res, err := flow.NewUnsignedExecutionReceipt(flow.UntrustedUnsignedExecutionReceipt(*receipt))

		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("invalid input with zero ExecutorID", func(t *testing.T) {
		receipt := unittest.UnsignedExecutionReceiptFixture()
		receipt.ExecutorID = flow.ZeroID

		res, err := flow.NewUnsignedExecutionReceipt(flow.UntrustedUnsignedExecutionReceipt(*receipt))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "executor ID must not be zero")
	})

	t.Run("invalid input with invalid execution result", func(t *testing.T) {
		receipt := unittest.UnsignedExecutionReceiptFixture()
		receipt.ExecutionResult.BlockID = flow.ZeroID

		res, err := flow.NewUnsignedExecutionReceipt(flow.UntrustedUnsignedExecutionReceipt(*receipt))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "invalid execution result: BlockID must not be empty")
	})

	t.Run("invalid input with nil Spocks", func(t *testing.T) {
		receipt := unittest.UnsignedExecutionReceiptFixture()
		receipt.Spocks = nil

		res, err := flow.NewUnsignedExecutionReceipt(flow.UntrustedUnsignedExecutionReceipt(*receipt))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "spocks must not be empty")
	})

	t.Run("invalid input with empty Spocks", func(t *testing.T) {
		receipt := unittest.UnsignedExecutionReceiptFixture()
		receipt.Spocks = []crypto.Signature{}

		res, err := flow.NewUnsignedExecutionReceipt(flow.UntrustedUnsignedExecutionReceipt(*receipt))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "spocks must not be empty")
	})
}

// TestNewExecutionReceiptStub verifies the behavior of the NewExecutionReceiptStub constructor.
// It ensures proper handling of both valid and invalid untrusted input fields.
//
// Test Cases:
//
// 1. Valid input:
//   - Verifies that a properly populated UntrustedExecutionReceiptStub results in a valid ExecutionReceiptStub.
//
// 2. Invalid input with invalid UnsignedExecutionReceiptStub:
//   - Ensures an error is returned when the UnsignedExecutionReceiptStub.ExecutorID is flow.ZeroID.
//
// 3. Invalid input with nil ExecutorSignature:
//   - Ensures an error is returned when the ExecutorSignature is nil.
//
// 4. Invalid input with empty ExecutorSignature:
//   - Ensures an error is returned when the ExecutorSignature is an empty byte slice.
func TestNewExecutionReceiptStub(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		receipt := unittest.ExecutionReceiptFixture().Stub()
		res, err := flow.NewExecutionReceiptStub(flow.UntrustedExecutionReceiptStub(*receipt))
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("invalid input with invalid unsigned execution receipt stub", func(t *testing.T) {
		receipt := unittest.ExecutionReceiptFixture().Stub()
		receipt.UnsignedExecutionReceiptStub.ExecutorID = flow.ZeroID

		res, err := flow.NewExecutionReceiptStub(flow.UntrustedExecutionReceiptStub(*receipt))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "invalid unsigned execution receipt stub: executor ID must not be zero")
	})

	t.Run("invalid input with nil ExecutorSignature", func(t *testing.T) {
		receipt := unittest.ExecutionReceiptFixture().Stub()
		receipt.ExecutorSignature = nil

		res, err := flow.NewExecutionReceiptStub(flow.UntrustedExecutionReceiptStub(*receipt))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "executor signature must not be empty")
	})

	t.Run("invalid input with empty ExecutorSignature", func(t *testing.T) {
		receipt := unittest.ExecutionReceiptFixture().Stub()
		receipt.ExecutorSignature = []byte{}

		res, err := flow.NewExecutionReceiptStub(flow.UntrustedExecutionReceiptStub(*receipt))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "executor signature must not be empty")
	})
}

// TestNewUnsignedExecutionReceiptStub verifies the behavior of the NewUnsignedExecutionReceiptStub constructor.
// It ensures proper handling of both valid and invalid untrusted input fields.
//
// Test Cases:
//
// 1. Valid input:
//   - Verifies that a properly populated UntrustedUnsignedExecutionReceiptStub results in a valid UnsignedExecutionReceiptStub.
//
// 2. Invalid input with zero executor ID:
//   - Ensures an error is returned when the ExecutorID is flow.ZeroID.
//
// 3. Invalid input with zero result ID:
//   - Ensures an error is returned when the ResultID is flow.ZeroID.
//
// 4. Invalid input with nil spocks:
//   - Ensures an error is returned when the spocks field is nil.
//
// 5. Invalid input with empty spocks:
//   - Ensures an error is returned when the spocks field is an empty slice.
func TestNewUnsignedExecutionReceiptStub(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		receipt := unittest.UnsignedExecutionReceiptFixture().Stub()
		res, err := flow.NewUnsignedExecutionReceiptStub(flow.UntrustedUnsignedExecutionReceiptStub(*receipt))
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("invalid input with zero executor ID", func(t *testing.T) {
		receipt := unittest.UnsignedExecutionReceiptFixture().Stub()
		receipt.ExecutorID = flow.ZeroID

		res, err := flow.NewUnsignedExecutionReceiptStub(flow.UntrustedUnsignedExecutionReceiptStub(*receipt))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "executor ID must not be zero")
	})

	t.Run("invalid input with zero result ID", func(t *testing.T) {
		receipt := unittest.UnsignedExecutionReceiptFixture().Stub()
		receipt.ResultID = flow.ZeroID

		res, err := flow.NewUnsignedExecutionReceiptStub(flow.UntrustedUnsignedExecutionReceiptStub(*receipt))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "result ID must not be zero")
	})

	t.Run("invalid input with nil spocks", func(t *testing.T) {
		receipt := unittest.UnsignedExecutionReceiptFixture().Stub()
		receipt.Spocks = nil

		res, err := flow.NewUnsignedExecutionReceiptStub(flow.UntrustedUnsignedExecutionReceiptStub(*receipt))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "spocks must not be empty")
	})

	t.Run("invalid input with empty spocks", func(t *testing.T) {
		receipt := unittest.UnsignedExecutionReceiptFixture().Stub()
		receipt.Spocks = []crypto.Signature{}

		res, err := flow.NewUnsignedExecutionReceiptStub(flow.UntrustedUnsignedExecutionReceiptStub(*receipt))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "spocks must not be empty")
	})
}
