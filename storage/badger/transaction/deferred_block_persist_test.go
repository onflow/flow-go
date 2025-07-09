package transaction_test

import (
	"fmt"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestDeferredBlockPersist(t *testing.T) {
	suite.Run(t, new(DeferredBlockPersistSuite))
}

type DeferredBlockPersistSuite struct {
	suite.Suite
}

// TestEmpty verifies that DeferredBlockPersist behaves like a no-op if nothing is scheduled
func (s *DeferredBlockPersistSuite) TestEmpty() {
	deferredPersistOps := transaction.NewDeferredBlockPersist()
	require.True(s.T(), deferredPersistOps.IsEmpty())

	// NewDeferredBlockPersist.Pending() should be a no-op and therefore not care that transaction.Tx is nil
	err := deferredPersistOps.Pending()(unittest.IdentifierFixture(), nil)
	require.NoError(s.T(), err)
}

// Test_AddBadgerOp adds 1 or 2 DeferredBadgerUpdate(s) and verifies that they are executed in the expected order
func (s *DeferredBlockPersistSuite) Test_AddBadgerOp() {
	blockID := unittest.IdentifierFixture()
	unittest.RunWithBadgerDB(s.T(), func(db *badger.DB) {
		s.Run("single DeferredBadgerUpdate", func() {
			m := NewBlockPersistCallMonitor(s.T())
			deferredPersistOps := transaction.NewDeferredBlockPersist().AddBadgerOp(m.MakeBadgerUpdate())
			require.False(s.T(), deferredPersistOps.IsEmpty())
			err := transaction.Update(db, deferredPersistOps.Pending().WithBlock(blockID))
			require.NoError(s.T(), err)
		})

		s.Run("two DeferredBadgerUpdates added individually", func() {
			m := NewBlockPersistCallMonitor(s.T())
			deferredPersistOps := transaction.NewDeferredBlockPersist().
				AddBadgerOp(m.MakeBadgerUpdate()).
				AddBadgerOp(m.MakeBadgerUpdate())
			require.False(s.T(), deferredPersistOps.IsEmpty())
			err := transaction.Update(db, deferredPersistOps.Pending().WithBlock(blockID))
			require.NoError(s.T(), err)
		})

		s.Run("two DeferredBadgerUpdates added as a sequence", func() {
			m := NewBlockPersistCallMonitor(s.T())
			deferredPersistOps := transaction.NewDeferredBlockPersist().AddBadgerOps(
				m.MakeBadgerUpdate(),
				m.MakeBadgerUpdate())
			require.False(s.T(), deferredPersistOps.IsEmpty())
			err := transaction.Update(db, deferredPersistOps.Pending().WithBlock(blockID))
			require.NoError(s.T(), err)
		})
	})
}

// TestDbOp adds 1 or 2 DeferredDBUpdate(s) and verifies that they are executed in the expected order
func (s *DeferredBlockPersistSuite) Test_AddDbOp() {
	blockID := unittest.IdentifierFixture()
	unittest.RunWithBadgerDB(s.T(), func(db *badger.DB) {
		s.Run("single DeferredDBUpdate without callback", func() {
			m := NewBlockPersistCallMonitor(s.T())
			deferredPersistOps := transaction.NewDeferredBlockPersist().
				AddDbOp(m.MakeDBUpdate(0))
			require.False(s.T(), deferredPersistOps.IsEmpty())
			err := transaction.Update(db, deferredPersistOps.Pending().WithBlock(blockID))
			require.NoError(s.T(), err)
		})

		s.Run("single DeferredDBUpdate with one callback", func() {
			m := NewBlockPersistCallMonitor(s.T())
			deferredPersistOps := transaction.NewDeferredBlockPersist().
				AddDbOp(m.MakeDBUpdate(1))
			require.False(s.T(), deferredPersistOps.IsEmpty())
			err := transaction.Update(db, deferredPersistOps.Pending().WithBlock(blockID))
			require.NoError(s.T(), err)
		})

		s.Run("single DeferredDBUpdate with multiple callbacks", func() {
			m := NewBlockPersistCallMonitor(s.T())
			deferredPersistOps := transaction.NewDeferredBlockPersist().
				AddDbOp(m.MakeDBUpdate(21))
			require.False(s.T(), deferredPersistOps.IsEmpty())
			err := transaction.Update(db, deferredPersistOps.Pending().WithBlock(blockID))
			require.NoError(s.T(), err)
		})

		s.Run("two DeferredDBUpdates added individually", func() {
			m := NewBlockPersistCallMonitor(s.T())
			deferredPersistOps := transaction.NewDeferredBlockPersist().
				AddDbOp(m.MakeDBUpdate(17)).
				AddDbOp(m.MakeDBUpdate(0))
			require.False(s.T(), deferredPersistOps.IsEmpty())
			err := transaction.Update(db, deferredPersistOps.Pending().WithBlock(blockID))
			require.NoError(s.T(), err)
		})

		s.Run("two DeferredDBUpdates added as a sequence", func() {
			m := NewBlockPersistCallMonitor(s.T())
			deferredPersistOps := transaction.NewDeferredBlockPersist()
			deferredPersistOps.AddDbOps(
				m.MakeDBUpdate(0),
				m.MakeDBUpdate(17))
			require.False(s.T(), deferredPersistOps.IsEmpty())
			err := transaction.Update(db, deferredPersistOps.Pending().WithBlock(blockID))
			require.NoError(s.T(), err)
		})
	})
}

// Test_AddIndexingOp adds 1 or 2 DeferredBlockPersistOp(s) and verifies that they are executed in the expected order
func (s *DeferredBlockPersistSuite) Test_AddIndexingOp() {
	blockID := unittest.IdentifierFixture()
	unittest.RunWithBadgerDB(s.T(), func(db *badger.DB) {
		s.Run("single DeferredBlockPersistOp without callback", func() {
			m := NewBlockPersistCallMonitor(s.T())
			deferredPersistOps := transaction.NewDeferredBlockPersist().
				AddIndexingOp(m.MakeIndexingOp(blockID, 0))
			require.False(s.T(), deferredPersistOps.IsEmpty())
			err := transaction.Update(db, deferredPersistOps.Pending().WithBlock(blockID))
			require.NoError(s.T(), err)
		})

		s.Run("single DeferredBlockPersistOp with one callback", func() {
			m := NewBlockPersistCallMonitor(s.T())
			deferredPersistOps := transaction.NewDeferredBlockPersist().
				AddIndexingOp(m.MakeIndexingOp(blockID, 1))
			require.False(s.T(), deferredPersistOps.IsEmpty())
			err := transaction.Update(db, deferredPersistOps.Pending().WithBlock(blockID))
			require.NoError(s.T(), err)
		})

		s.Run("single DeferredBlockPersistOp with multiple callbacks", func() {
			m := NewBlockPersistCallMonitor(s.T())
			deferredPersistOps := transaction.NewDeferredBlockPersist().
				AddIndexingOp(m.MakeIndexingOp(blockID, 21))
			require.False(s.T(), deferredPersistOps.IsEmpty())
			err := transaction.Update(db, deferredPersistOps.Pending().WithBlock(blockID))
			require.NoError(s.T(), err)
		})

		s.Run("two DeferredBlockPersistOp added individually", func() {
			m := NewBlockPersistCallMonitor(s.T())
			deferredPersistOps := transaction.NewDeferredBlockPersist().
				AddIndexingOp(m.MakeIndexingOp(blockID, 17)).
				AddIndexingOp(m.MakeIndexingOp(blockID, 0))
			require.False(s.T(), deferredPersistOps.IsEmpty())
			err := transaction.Update(db, deferredPersistOps.Pending().WithBlock(blockID))
			require.NoError(s.T(), err)
		})

		s.Run("two DeferredBlockPersistOp added as a sequence", func() {
			m := NewBlockPersistCallMonitor(s.T())
			deferredPersistOps := transaction.NewDeferredBlockPersist()
			deferredPersistOps.AddIndexingOps(
				m.MakeIndexingOp(blockID, 0),
				m.MakeIndexingOp(blockID, 17))
			require.False(s.T(), deferredPersistOps.IsEmpty())
			err := transaction.Update(db, deferredPersistOps.Pending().WithBlock(blockID))
			require.NoError(s.T(), err)
		})
	})
}

// Test_AddOnSucceedCallback adds 1 or 2 callback(s) and verifies that they are executed in the expected order
func (s *DeferredBlockPersistSuite) Test_AddOnSucceedCallback() {
	blockID := unittest.IdentifierFixture()
	unittest.RunWithBadgerDB(s.T(), func(db *badger.DB) {
		s.Run("single callback", func() {
			m := NewBlockPersistCallMonitor(s.T())
			deferredPersistOps := transaction.NewDeferredBlockPersist().
				OnSucceed(m.MakeCallback())
			require.False(s.T(), deferredPersistOps.IsEmpty())
			err := transaction.Update(db, deferredPersistOps.Pending().WithBlock(blockID))
			require.NoError(s.T(), err)
		})

		s.Run("two callbacks added individually", func() {
			m := NewBlockPersistCallMonitor(s.T())
			deferredPersistOps := transaction.NewDeferredBlockPersist().
				OnSucceed(m.MakeCallback()).
				OnSucceed(m.MakeCallback())
			require.False(s.T(), deferredPersistOps.IsEmpty())
			err := transaction.Update(db, deferredPersistOps.Pending().WithBlock(blockID))
			require.NoError(s.T(), err)
		})

		s.Run("many callbacks added as a sequence", func() {
			m := NewBlockPersistCallMonitor(s.T())
			deferredPersistOps := transaction.NewDeferredBlockPersist().
				OnSucceeds(m.MakeCallbacks(11)...)
			require.False(s.T(), deferredPersistOps.IsEmpty())
			err := transaction.Update(db, deferredPersistOps.Pending().WithBlock(blockID))
			require.NoError(s.T(), err)
		})
	})
}

// Test_EverythingMixed uses all ways to add functors in combination and verifies that they are executed in the expected order
func (s *DeferredBlockPersistSuite) Test_EverythingMixed() {
	blockID := unittest.IdentifierFixture()
	unittest.RunWithBadgerDB(s.T(), func(db *badger.DB) {
		m := NewBlockPersistCallMonitor(s.T())
		deferredPersistOps := transaction.NewDeferredBlockPersist().
			OnSucceed(m.MakeCallback()).
			AddDbOp(m.MakeDBUpdate(1)).
			AddBadgerOp(m.MakeBadgerUpdate()).
			AddIndexingOp(m.MakeIndexingOp(blockID, 2)).
			OnSucceeds(m.MakeCallbacks(3)...).
			AddDbOp(m.MakeDBUpdate(0)).
			AddBadgerOps(
				m.MakeBadgerUpdate(),
				m.MakeBadgerUpdate(),
				m.MakeBadgerUpdate()).
			AddIndexingOps(
				m.MakeIndexingOp(blockID, 7),
				m.MakeIndexingOp(blockID, 0)).
			OnSucceeds(
				m.MakeCallback(),
				m.MakeCallback()).
			AddDbOps(
				m.MakeDBUpdate(7),
				m.MakeDBUpdate(0),
				m.MakeDBUpdate(1)).
			OnSucceed(m.MakeCallback())
		require.False(s.T(), deferredPersistOps.IsEmpty())
		err := transaction.Update(db, deferredPersistOps.Pending().WithBlock(blockID))
		require.NoError(s.T(), err)
	})
}

/* ***************************************** Testing Utility BlockPersistCallMonitor ***************************************** */

// BlockPersistCallMonitor is a utility for testing that DeferredBlockPersist calls its input functors and callbacks
// in the correct order. DeferredBlockPersist is expected to proceed as follows:
//
//  0. Record functors added via `AddBadgerOp`, `AddDbOp`, `AddIndexingOp`, `OnSucceed` ...
//  1. Execute the functors in the order they were added
//  2. During each functor's execution:
//     - some functor's may schedule callbacks (depending on their type)
//     - record those callbacks in the order they are scheduled (no execution yet)
//     `OnSucceed` schedules its callback during its execution at this step as well
//  3. If and only if the underlying database transaction _successfully_ completed, run the callbacks
//
// To verify the correct order of calls, the BlockPersistCallMonitor generates functors. Each functor has a
// dedicated index value. When the functor is called, it checks that its index matches the functor index
// that the BlockPersistCallMonitor expects to be executed next. For callbacks, we proceed analogously.
//
// Usage note:
// The call BlockPersistCallMonitor assumes that functors are added to DeferredBlockPersist exactly in the order that
// BlockPersistCallMonitor generates them. This works very intuitively, when the tests proceed as in the following example:
//
//	m := NewBlockPersistCallMonitor(t)
//	deferredPersistOps :=  transaction.NewDeferredBlockPersist()
//	deferredPersistOps.AddBadgerOp(m.MakeBadgerUpdate()) // here, we add the functor right when it is generated
//	transaction.Update(db, deferredPersistOps.Pending())
type BlockPersistCallMonitor struct {
	generatedTxFunctors int
	generatedCallbacks  int

	T                        *testing.T
	nextExpectedTxFunctorIdx int
	nextExpectedCallbackIdx  int
}

func NewBlockPersistCallMonitor(t *testing.T) *BlockPersistCallMonitor {
	return &BlockPersistCallMonitor{T: t}
}

func (cm *BlockPersistCallMonitor) MakeIndexingOp(expectedBlockID flow.Identifier, withCallbacks int) transaction.DeferredBlockPersistOp {
	myFunctorIdx := cm.generatedTxFunctors       // copy into local scope. Determined when we construct functor
	callbacks := cm.MakeCallbacks(withCallbacks) // pre-generate callback functors
	functor := func(blockID flow.Identifier, tx *transaction.Tx) error {
		if expectedBlockID != blockID {
			cm.T.Errorf("expected block ID %v but got %v", expectedBlockID, blockID)
			return fmt.Errorf("expected block ID %v but got %v", expectedBlockID, blockID)
		}
		for _, c := range callbacks {
			tx.OnSucceed(c) // schedule callback
		}
		if cm.nextExpectedTxFunctorIdx != myFunctorIdx {
			// nextExpectedTxFunctorIdx holds the Index of the Functor that was generated next. DeferredBlockPersist
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

func (cm *BlockPersistCallMonitor) MakeDBUpdate(withCallbacks int) transaction.DeferredDBUpdate {
	myFunctorIdx := cm.generatedTxFunctors       // copy into local scope. Determined when we construct functor
	callbacks := cm.MakeCallbacks(withCallbacks) // pre-generate callback functors
	functor := func(tx *transaction.Tx) error {
		for _, c := range callbacks {
			tx.OnSucceed(c) // schedule callback
		}
		if cm.nextExpectedTxFunctorIdx != myFunctorIdx {
			// nextExpectedTxFunctorIdx holds the Index of the Functor that was generated next. DeferredBlockPersist
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

func (cm *BlockPersistCallMonitor) MakeBadgerUpdate() transaction.DeferredBadgerUpdate {
	myFunctorIdx := cm.generatedTxFunctors // copy into local scope. Determined when we construct functor
	functor := func(tx *badger.Txn) error {
		if cm.nextExpectedTxFunctorIdx != myFunctorIdx {
			// nextExpectedTxFunctorIdx holds the Index of the Functor that was generated next. DeferredBlockPersist
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

func (cm *BlockPersistCallMonitor) MakeCallback() func() {
	myFunctorIdx := cm.generatedCallbacks // copy into local scope. Determined when we construct callback
	functor := func() {
		if cm.nextExpectedCallbackIdx != myFunctorIdx {
			// nextExpectedCallbackIdx holds the Index of the callback that was generated next. DeferredBlockPersist
			// should execute the callback in the order they were scheduled, which is violated. Hence, we fail:
			cm.T.Errorf("expected next Callback Index is %d but my value is %d", cm.nextExpectedCallbackIdx, myFunctorIdx)
		}
		cm.nextExpectedCallbackIdx += 1 // happy path
	}

	cm.generatedCallbacks += 1
	return functor
}

func (cm *BlockPersistCallMonitor) MakeCallbacks(numberCallbacks int) []func() {
	callbacks := make([]func(), 0, numberCallbacks)
	for ; 0 < numberCallbacks; numberCallbacks-- {
		callbacks = append(callbacks, cm.MakeCallback())
	}
	return callbacks
}
