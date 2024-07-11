package transaction_test

import (
	"fmt"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage/badger/transaction"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestEmpty verifies that DeferredDbOps behaves like a no-op if nothing is scheduled
func TestEmpty(t *testing.T) {
	deferredDbOps := transaction.NewDeferredDbOps()
	// deferredDbOps.Pending() should be a no-op and therefore not care that transaction.Tx is nil
	err := deferredDbOps.Pending()(nil)
	require.NoError(t, err)
}

// TestAddBaderOp adds 1 or 2 DeferredBadgerUpdate(s) and verifies that they are executed in the expected order
func Test_AddBaderOp(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		t.Run("single DeferredBadgerUpdate", func(t *testing.T) {
			m := NewCallMonitor(t)
			deferredDbOps := transaction.NewDeferredDbOps().
				AddBadgerOp(m.MakeBadgerUpdate())
			err := transaction.Update(db, deferredDbOps.Pending())
			require.NoError(t, err)
		})

		t.Run("two DeferredBadgerUpdates added individually", func(t *testing.T) {
			m := NewCallMonitor(t)
			deferredDbOps := transaction.NewDeferredDbOps().
				AddBadgerOp(m.MakeBadgerUpdate()).
				AddBadgerOp(m.MakeBadgerUpdate())
			err := transaction.Update(db, deferredDbOps.Pending())
			require.NoError(t, err)
		})

		t.Run("two DeferredBadgerUpdates added as a sequence", func(t *testing.T) {
			m := NewCallMonitor(t)
			deferredDbOps := transaction.NewDeferredDbOps()
			deferredDbOps.AddBadgerOps(
				m.MakeBadgerUpdate(),
				m.MakeBadgerUpdate())
			err := transaction.Update(db, deferredDbOps.Pending())
			require.NoError(t, err)
		})
	})
}

// TestDbOp adds 1 or 2 DeferredDBUpdate(s) and verifies that they are executed in the expected order
func Test_AddDbOp(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		t.Run("single DeferredDBUpdate without callback", func(t *testing.T) {
			m := NewCallMonitor(t)
			deferredDbOps := transaction.NewDeferredDbOps().
				AddDbOp(m.MakeDBUpdate(0))
			err := transaction.Update(db, deferredDbOps.Pending())
			require.NoError(t, err)
		})

		t.Run("single DeferredDBUpdate with one callback", func(t *testing.T) {
			m := NewCallMonitor(t)
			deferredDbOps := transaction.NewDeferredDbOps().
				AddDbOp(m.MakeDBUpdate(1))
			err := transaction.Update(db, deferredDbOps.Pending())
			require.NoError(t, err)
		})

		t.Run("single DeferredDBUpdate with multiple callbacks", func(t *testing.T) {
			m := NewCallMonitor(t)
			deferredDbOps := transaction.NewDeferredDbOps().
				AddDbOp(m.MakeDBUpdate(21))
			err := transaction.Update(db, deferredDbOps.Pending())
			require.NoError(t, err)
		})

		t.Run("two DeferredDBUpdates added individually", func(t *testing.T) {
			m := NewCallMonitor(t)
			deferredDbOps := transaction.NewDeferredDbOps().
				AddDbOp(m.MakeDBUpdate(17)).
				AddDbOp(m.MakeDBUpdate(0))
			err := transaction.Update(db, deferredDbOps.Pending())
			require.NoError(t, err)
		})

		t.Run("two DeferredDBUpdates added as a sequence", func(t *testing.T) {
			m := NewCallMonitor(t)
			deferredDbOps := transaction.NewDeferredDbOps()
			deferredDbOps.AddDbOps(
				m.MakeDBUpdate(0),
				m.MakeDBUpdate(17))
			err := transaction.Update(db, deferredDbOps.Pending())
			require.NoError(t, err)
		})
	})
}

// Test_AddOnSucceedCallback adds 1 or 2 callback(s) and verifies that they are executed in the expected order
func Test_AddOnSucceedCallback(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		t.Run("single callback", func(t *testing.T) {
			m := NewCallMonitor(t)
			deferredDbOps := transaction.NewDeferredDbOps().
				OnSucceed(m.MakeCallback())
			err := transaction.Update(db, deferredDbOps.Pending())
			require.NoError(t, err)
		})

		t.Run("two callbacks added individually", func(t *testing.T) {
			m := NewCallMonitor(t)
			deferredDbOps := transaction.NewDeferredDbOps().
				OnSucceed(m.MakeCallback()).
				OnSucceed(m.MakeCallback())
			err := transaction.Update(db, deferredDbOps.Pending())
			require.NoError(t, err)
		})

		t.Run("many callbacks added as a sequence", func(t *testing.T) {
			m := NewCallMonitor(t)
			deferredDbOps := transaction.NewDeferredDbOps().
				OnSucceeds(m.MakeCallbacks(11)...)
			err := transaction.Update(db, deferredDbOps.Pending())
			require.NoError(t, err)
		})
	})
}

// Test_EverythingMixed uses all ways to add functors in combination and verifies that they are executed in the expected order
func Test_EverythingMixed(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		m := NewCallMonitor(t)
		deferredDbOps := transaction.NewDeferredDbOps().
			OnSucceed(m.MakeCallback()).
			AddDbOp(m.MakeDBUpdate(1)).
			AddBadgerOp(m.MakeBadgerUpdate()).
			OnSucceeds(m.MakeCallbacks(3)...).
			AddDbOp(m.MakeDBUpdate(0)).
			AddBadgerOps(
				m.MakeBadgerUpdate(),
				m.MakeBadgerUpdate(),
				m.MakeBadgerUpdate()).
			OnSucceeds(
				m.MakeCallback(),
				m.MakeCallback()).
			AddDbOps(
				m.MakeDBUpdate(7),
				m.MakeDBUpdate(0),
				m.MakeDBUpdate(1)).
			OnSucceed(m.MakeCallback())
		err := transaction.Update(db, deferredDbOps.Pending())
		require.NoError(t, err)
	})
}

/* ***************************************** Testing Utility CallMonitor ***************************************** */

// CallMonitor is a utility for testing that DeferredDbOps calls its input functors and callbacks
// in the correct order. DeferredDbOps is expected to proceed as follows:
//
//  0. Record functors added via `AddBadgerOp`, `AddDbOp`, `OnSucceed` ...
//  1. Execute the functors in the order they were added
//  2. During each functor's execution:
//     - some functor's may schedule callbacks (depending on their type)
//     - record those callbacks in the order they are scheduled (no execution yet)
//     `OnSucceed` schedules its callback during its execution at this step as well
//  3. If and only if the underlying database transaction _successfully_ completed, run the callbacks
//
// To verify the correct order of calls, the CallMonitor generates functors. Each functor has a
// dedicated index value. When the functor is called, it checks that its index matches the functor index
// that the CallMonitor expects to be executed next. For callbacks, we proceed analogously.
//
// Usage note:
// The call CallMonitor assumes that functors are added to DeferredDbOps exactly in the order that
// CallMonitor generates them. This works very intuitively, when the tests proceed as in the following example:
//
//	m := NewCallMonitor(t)
//	deferredDbOps := transaction.NewDeferredDbOps()
//	deferredDbOps.AddBadgerOp(m.MakeBadgerUpdate()) // here, we add the functor right when it is generated
//	transaction.Update(db, deferredDbOps.Pending())
type CallMonitor struct {
	generatedTxFunctors int
	generatedCallbacks  int

	T                        *testing.T
	nextExpectedTxFunctorIdx int
	nextExpectedCallbackIdx  int
}

func NewCallMonitor(t *testing.T) *CallMonitor {
	return &CallMonitor{T: t}
}

func (cm *CallMonitor) MakeDBUpdate(withCallbacks int) transaction.DeferredDBUpdate {
	myFunctorIdx := cm.generatedTxFunctors       // copy into local scope. Determined when we construct functor
	callbacks := cm.MakeCallbacks(withCallbacks) // pre-generate callback functors
	functor := func(tx *transaction.Tx) error {
		for _, c := range callbacks {
			tx.OnSucceed(c) // schedule callback
		}
		if cm.nextExpectedTxFunctorIdx != myFunctorIdx {
			// nextExpectedTxFunctorIdx holds the Index of the Functor that was generated next. DeferredDbOps
			// should execute the functors in the order they were added, which is violated. Hence, we fail:
			cm.T.Errorf("expected next Functor Index is %d but my value is %d", cm.nextExpectedTxFunctorIdx, myFunctorIdx)
			return fmt.Errorf("expected next Functor Index is %d but my value is %d", cm.nextExpectedTxFunctorIdx, myFunctorIdx)
		}

		// happy path:
		cm.nextExpectedTxFunctorIdx += 1
		return nil
	}

	cm.generatedTxFunctors += 1
	return functor
}

func (cm *CallMonitor) MakeBadgerUpdate() transaction.DeferredBadgerUpdate {
	myFunctorIdx := cm.generatedTxFunctors // copy into local scope. Determined when we construct functor
	functor := func(tx *badger.Txn) error {
		if cm.nextExpectedTxFunctorIdx != myFunctorIdx {
			// nextExpectedTxFunctorIdx holds the Index of the Functor that was generated next. DeferredDbOps
			// should execute the functors in the order they were added, which is violated. Hence, we fail:
			cm.T.Errorf("expected next Functor Index is %d but my value is %d", cm.nextExpectedTxFunctorIdx, myFunctorIdx)
			return fmt.Errorf("expected next Functor Index is %d but my value is %d", cm.nextExpectedTxFunctorIdx, myFunctorIdx)
		}

		// happy path:
		cm.nextExpectedTxFunctorIdx += 1
		return nil
	}

	cm.generatedTxFunctors += 1
	return functor
}

func (cm *CallMonitor) MakeCallback() func() {
	myFunctorIdx := cm.generatedCallbacks // copy into local scope. Determined when we construct callback
	functor := func() {
		if cm.nextExpectedCallbackIdx != myFunctorIdx {
			// nextExpectedCallbackIdx holds the Index of the callback that was generated next. DeferredDbOps
			// should execute the callback in the order they were scheduled, which is violated. Hence, we fail:
			cm.T.Errorf("expected next Callback Index is %d but my value is %d", cm.nextExpectedCallbackIdx, myFunctorIdx)
		}
		cm.nextExpectedCallbackIdx += 1 // happy path
	}

	cm.generatedCallbacks += 1
	return functor
}

func (cm *CallMonitor) MakeCallbacks(numberCallbacks int) []func() {
	callbacks := make([]func(), 0, numberCallbacks)
	for ; 0 < numberCallbacks; numberCallbacks-- {
		callbacks = append(callbacks, cm.MakeCallback())
	}
	return callbacks
}
