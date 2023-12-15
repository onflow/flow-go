package transaction_test

import (
	"fmt"
	"testing"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/storage/badger/transaction"
)

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
type CallMonitor struct {
	generatedTxFunctors int
	generatedCallbacks  int

	T                        *testing.T
	nextExpectedTxFunctorIdx int
	nextExpectedCallbackIdx  int
}

func (cm *CallMonitor) MakeDBUpdate(withCallbacks int) transaction.DeferredDBUpdate {
	cm.generatedTxFunctors += 1
	myFunctorIdx := cm.generatedTxFunctors       // copy into local scope. Determined when we construct functor
	callbacks := cm.MakeCallbacks(withCallbacks) // pre-generate callback functors

	return func(tx *transaction.Tx) error {
		for _, c := range callbacks {
			tx.OnSucceed(c) // schedule callback
		}
		if cm.nextExpectedTxFunctorIdx != myFunctorIdx {
			// nextExpectedTxFunctorIdx holds the Index of the Functor that was generated next. DeferredDbOps
			// should execute the functors in the order they were added, which is violated. Hence, we fail:
			cm.T.Errorf("expected next Functor Index is %d but my value is %d", cm.nextExpectedTxFunctorIdx, myFunctorIdx)
			return fmt.Errorf("expected next Functor Index is %d but my value is %d", cm.nextExpectedTxFunctorIdx, myFunctorIdx)
		}
		return nil
	}
}

func (cm *CallMonitor) MakeBadgerUpdate() transaction.DeferredBadgerUpdate {
	cm.generatedTxFunctors += 1

	myFunctorIdx := cm.generatedTxFunctors // copy into local scope. Determined when we construct functor
	return func(tx *badger.Txn) error {
		if cm.nextExpectedTxFunctorIdx == myFunctorIdx {
			return nil // happy path
		}
		// nextExpectedTxFunctorIdx holds the Index of the Functor that was generated next. DeferredDbOps
		// should execute the functors in the order they were added, which is violated. Hence, we fail:
		cm.T.Errorf("expected next Functor Index is %d but my value is %d", cm.nextExpectedTxFunctorIdx, myFunctorIdx)
		return fmt.Errorf("expected next Functor Index is %d but my value is %d", cm.nextExpectedTxFunctorIdx, myFunctorIdx)
	}
}

func (cm *CallMonitor) MakeCallback() func() {
	cm.generatedCallbacks += 1
	myFunctorIdx := cm.generatedCallbacks // copy into local scope. Determined when we construct callback

	return func() {
		if cm.nextExpectedCallbackIdx != myFunctorIdx {
			// nextExpectedCallbackIdx holds the Index of the callback that was generated next. DeferredDbOps
			// should execute the callback in the order they were scheduled, which is violated. Hence, we fail:
			cm.T.Errorf("expected next Callback Index is %d but my value is %d", cm.nextExpectedCallbackIdx, myFunctorIdx)
		}
	}
}

func (cm *CallMonitor) MakeCallbacks(numberCallbacks int) []func() {
	callbacks := make([]func(), 0, numberCallbacks)
	for ; 0 < numberCallbacks; numberCallbacks-- {
		callbacks = append(callbacks, cm.MakeCallback())
	}
	return callbacks
}
