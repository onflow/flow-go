package storage

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/model/flow"
)

type ERMempoolTestSuit struct {
	suite.Suite
	mempool verification.Mempool
}

// SetupTests initiates the test setups prior to each test
func (suite *ERMempoolTestSuit) SetupTest() {
	suite.mempool = New()
}

func TestERMempoolTestSuite(t *testing.T) {
	suite.Run(t, new(ERMempoolTestSuit))
}

// TestPuttingEmptyER tests the Put method against adding a single Execution Receipt
func (suite *ERMempoolTestSuit) TestPuttingSingleER() {
	er := verification.RandomERGen()
	// number of results in mempool should be zero prior to any insertion
	assert.Equal(suite.T(), uint(0), suite.mempool.ResultsNum())

	// adding an ER to the mempool
	suite.mempool.Put(er)

	// number of results in mempool should be one after the single ER inserted
	assert.Equal(suite.T(), uint(1), suite.mempool.ResultsNum())

	// number of receipts corresponding to the specific execution result also should be
	// equal to one after the successful insertion
	assert.Equal(suite.T(), uint(1), suite.mempool.ReceiptsCount(er.ExecutionResult.ID()))
}

// TestPuttingEmptyER tests the Put method against adding two distinct Execution Receipts
// with different results side
func (suite *ERMempoolTestSuit) TestPuttingTwoERs() {
	// generating two random ERs
	er1 := verification.RandomERGen()
	er2 := verification.RandomERGen()

	// two random ERs require to be distinct even in their result paer
	require.NotEqual(suite.T(), er1, er2)
	require.NotEqual(suite.T(), er1.ExecutionResult, er2.ExecutionResult)

	// size of mempool should be zero prior putting ER
	assert.Equal(suite.T(), uint(0), suite.mempool.ResultsNum())

	// adding two ERs
	suite.mempool.Put(er1)
	suite.mempool.Put(er2)

	// adding two ERs should make number of results equal to two
	assert.Equal(suite.T(), uint(2), suite.mempool.ResultsNum())

	// number of receipts corresponding to each ERs also should be
	// equal to one after the successful insertion
	assert.Equal(suite.T(), uint(1), suite.mempool.ReceiptsCount(er1.ExecutionResult.ID()))
	assert.Equal(suite.T(), uint(1), suite.mempool.ReceiptsCount(er2.ExecutionResult.ID()))
}

// TestPuttingTwoERsWithSharedResult tests the Put method against adding a two Execution Receipts with
// the same execution result
func (suite *ERMempoolTestSuit) TestPuttingTwoERsWithSharedResult() {
	// generating two random ERs
	er1 := verification.RandomERGen()
	er2 := verification.RandomERGen()

	// overriding the result part of er1 to er2 so making both
	// of the same result
	er2.ExecutionResult = er1.ExecutionResult

	// two random ERs require to be distinct but with the same result part
	require.NotEqual(suite.T(), er1, er2)
	require.Equal(suite.T(), er1.ExecutionResult, er2.ExecutionResult)
	require.Equal(suite.T(), er1.ExecutionResult.ID(), er2.ExecutionResult.ID())

	// size of mempool should be zero prior putting ER
	assert.Equal(suite.T(), uint(0), suite.mempool.ResultsNum())

	// adding two ERs to the mempool
	suite.mempool.Put(er1)
	suite.mempool.Put(er2)

	// adding two ERs with the same result part should make number of results equal to one
	assert.Equal(suite.T(), uint(1), suite.mempool.ResultsNum())

	// number of receipts corresponding to their share result also should be
	// equal to two after the successful insertion
	assert.Equal(suite.T(), uint(2), suite.mempool.ReceiptsCount(er1.ExecutionResult.ID()))
}

// TestGettingResultWithNilInput tests both Getters of mempool for nil inputs
func (suite *ERMempoolTestSuit) TestGettingResultWithNilInput() {
	list, err := suite.mempool.GetExecutionReceipts(flow.ZeroID)
	assert.Nil(suite.T(), list)
	assert.NotNil(suite.T(), err)

	result, err := suite.mempool.GetExecutionResult(flow.ZeroID)
	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

// TestGettingNonMembership tests both Getters of mempool for inputs that do not exist in it
func (suite *ERMempoolTestSuit) TestGettingNonMembership() {
	// generating a random ER that does not exist in the mempool
	er := verification.RandomERGen()
	resultID := er.ExecutionResult.ID()

	// no ER should be in the mempool for looking up this hash
	list, err := suite.mempool.GetExecutionReceipts(resultID)
	assert.Nil(suite.T(), list)
	assert.NotNil(suite.T(), err)

	// no result should be in the mempool for looking up this hash
	result, err := suite.mempool.GetExecutionResult(resultID)
	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

// TestGettingSingleER stores and then retrieves a single ER from the mempool
func (suite *ERMempoolTestSuit) TestGettingSingleER() {
	// generating a random ER that does not exist in the mempool
	er := verification.RandomERGen()
	resultID := er.ExecutionResult.ID()

	// storing ER in the mempool
	suite.mempool.Put(er)

	// retrieving er and its result from mempool
	list, err := suite.mempool.GetExecutionReceipts(resultID)
	assert.Equal(suite.T(), er, list[er.ID()])
	assert.Nil(suite.T(), err)

	result, err := suite.mempool.GetExecutionResult(resultID)
	require.NotNil(suite.T(), result)
	assert.Equal(suite.T(), er.ExecutionResult, *result)
	assert.Nil(suite.T(), err)
}

// TestGettingTwoERs stores and then retrieves two distinct ERs from the mempool
func (suite *ERMempoolTestSuit) TestGettingTwoERs() {
	// generating two random ERs
	er1 := verification.RandomERGen()
	er2 := verification.RandomERGen()

	// two random ERs require to be distinct even in their result part
	require.NotEqual(suite.T(), er1, er2)
	require.NotEqual(suite.T(), er1.ExecutionResult, er2.ExecutionResult)

	erlist := []*flow.ExecutionReceipt{er1, er2}

	for _, er := range erlist {
		// adding two ERs to the mempool
		suite.mempool.Put(er)
	}

	for _, er := range erlist {
		hash := er.ExecutionResult.ID()
		// retrieving the ER and its result from mempool
		list, err := suite.mempool.GetExecutionReceipts(hash)
		assert.Equal(suite.T(), er, list[er.ID()])
		assert.Nil(suite.T(), err)

		result, err := suite.mempool.GetExecutionResult(hash)
		assert.Equal(suite.T(), er.ExecutionResult, *result)
		assert.Nil(suite.T(), err)
	}
}

// TestGettingTwoERsWithSharedResult stores and then retrieves two distinct ERs from the mempool
func (suite *ERMempoolTestSuit) TestGettingTwoERsWithSharedResult() {
	// generating two random ERs
	er1 := verification.RandomERGen()
	er2 := verification.RandomERGen()
	// overriding the result part of er1 to er2 so making both
	// of the same result
	er2.ExecutionResult = er1.ExecutionResult

	// two random ERs require to be distinct but with identical result part
	require.NotEqual(suite.T(), er1, er2)
	require.Equal(suite.T(), er1.ExecutionResult, er2.ExecutionResult)

	erlist := []*flow.ExecutionReceipt{er1, er2}

	for _, er := range erlist {
		// adding two ERs to the mempool
		suite.mempool.Put(er)
	}

	for _, er := range erlist {
		resultID := er.ExecutionResult.ID()
		// retrieving the ER and its result from mempool
		list, err := suite.mempool.GetExecutionReceipts(resultID)
		assert.Equal(suite.T(), er, list[er.ID()])
		assert.Nil(suite.T(), err)

		result, err := suite.mempool.GetExecutionResult(resultID)
		assert.Equal(suite.T(), er.ExecutionResult, *result)
		assert.Nil(suite.T(), err)
	}
}

// TestDuplicateER tests the Put method against adding two identical Execution Receipts sequentially
// The second ER is expected not to be processed
func (suite *ERMempoolTestSuit) TestDuplicateER() {
	// generating a random ER
	er := verification.RandomERGen()

	// size of mempool should be zero prior putting ER
	assert.Equal(suite.T(), uint(0), suite.mempool.ResultsNum())

	// adding the same ER twice to the mempool
	suite.mempool.Put(er)
	suite.mempool.Put(er)

	// adding two ERs with the same result part should make number of results equal to one
	assert.Equal(suite.T(), uint(1), suite.mempool.ResultsNum())

	// number of receipts corresponding to their share result should be 1, in other word,
	// the second put operation should not take an effect
	assert.Equal(suite.T(), uint(1), suite.mempool.ReceiptsCount(er.ExecutionResult.ID()))
}

// TestDeadLock tests the implementation of mempool against indefinite waits
// that may happen due to the mutex locks
// The tests makes sure that several instances of each method of mempool are
// able to be invoked, executed, and terminated within a bounded interval
func (suite *ERMempoolTestSuit) TestDeadlock() {
	// generating a random ER that does not exist in the mempool
	er := verification.RandomERGen()
	resultID := er.ExecutionResult.ID()
	wg := sync.WaitGroup{}

	for i := 0; i < 10; i++ {
		// in each iteration, we spawn 5 goroutines, hence, we need
		// to lock the wait group on 5
		wg.Add(5)
		go func() {
			defer wg.Done()
			_, _ = suite.mempool.GetExecutionResult(flow.ZeroID)
		}()
		go func() {
			defer wg.Done()
			_, _ = suite.mempool.GetExecutionReceipts(flow.ZeroID)
		}()
		go func() {
			defer wg.Done()
			suite.mempool.Put(er)
		}()
		go func() {
			defer wg.Done()
			suite.mempool.ReceiptsCount(resultID)
		}()
		go func() {
			defer wg.Done()
			suite.mempool.ResultsNum()
		}()
	}

	// Giving gossip calls time to finish propagating
	waitChannel := make(chan struct{})
	testPassed := false
	go func() {
		wg.Wait()
		close(waitChannel)
	}()
	select {
	case <-waitChannel:
		testPassed = true
	case <-time.After(500 * time.Millisecond):
		suite.T().Logf("Test timed out, possible deadlock")
	}

	assert.True(suite.T(), testPassed)
}
