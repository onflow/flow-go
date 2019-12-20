package storage

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"crypto/rand"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/verification"
	exec "github.com/dapperlabs/flow-go/model/execution"
)

type ERStoreTestSuit struct {
	suite.Suite
	mempool verification.Mempool
}

// SetupTests initiates the test setups prior to each test
func (suite *ERStoreTestSuit) SetupTest() {
	suite.mempool = New()
}

func TestERStoreTestSuite(t *testing.T) {
	suite.Run(t, new(ERStoreTestSuit))
}

// TestPuttingEmptyER tests the Put method against adding a single Execution Receipt
func (suite *ERStoreTestSuit) TestPuttingSingleER() {
	er := randomER()
	// number of results in mempool should be zero prior to any insertion
	assert.Equal(suite.T(), suite.mempool.ResultsNum(), 0)

	// adding an ER to the mempool
	suite.mempool.Put(er)

	// number of results in mempool should be one after the single ER inserted
	assert.Equal(suite.T(), suite.mempool.ResultsNum(), 1)

	// number of receipts corresponding to the specific execution result also should be
	// equal to one after the successful insertion
	assert.Equal(suite.T(), suite.mempool.ReceiptsCount(er.ExecutionResult.Hash()), 1)
}

// TestPuttingEmptyER tests the Put method against adding two distinct Execution Receipts
// with different results side
func (suite *ERStoreTestSuit) TestPuttingTwoERs() {
	// generating two random ERs
	er1 := randomER()
	er2 := randomER()

	// two random ERs require to be distinct even in their result paer
	require.NotEqual(suite.T(), er1, er2)
	require.NotEqual(suite.T(), er1.ExecutionResult, er2.ExecutionResult)

	// size of estore should be zero prior putting ER
	assert.Equal(suite.T(), suite.mempool.ResultsNum(), 0)

	// adding two ERs
	suite.mempool.Put(er1)
	suite.mempool.Put(er2)

	// adding two ERs should make number of results equal to two
	assert.Equal(suite.T(), suite.mempool.ResultsNum(), 2)

	// number of receipts corresponding to each ERs also should be
	// equal to one after the successful insertion
	assert.Equal(suite.T(), suite.mempool.ReceiptsCount(er1.ExecutionResult.Hash()), 1)
	assert.Equal(suite.T(), suite.mempool.ReceiptsCount(er2.ExecutionResult.Hash()), 1)
}

// TestPuttingTwoERsWithSharedResult tests the Put method against adding a two Execution Receipts with
// the same execution result
func (suite *ERStoreTestSuit) TestPuttingTwoERsWithSharedResult() {
	// generating two random ERs
	er1 := randomER()
	er2 := randomER()
	// overriding the result part of er1 to er2 so making both
	// of the same result
	er2.ExecutionResult = er1.ExecutionResult

	// two random ERs require to be distinct but with the same result part
	require.NotEqual(suite.T(), er1, er2)
	require.Equal(suite.T(), er1.ExecutionResult, er2.ExecutionResult)
	require.Equal(suite.T(), er1.ExecutionResult.Hash(), er2.ExecutionResult.Hash())

	// size of estore should be zero prior putting ER
	assert.Equal(suite.T(), suite.mempool.ResultsNum(), 0)

	// adding two ERs to the mempool
	suite.mempool.Put(er1)
	suite.mempool.Put(er2)

	// adding two ERs with the same result part should make number of results equal to one
	assert.Equal(suite.T(), suite.mempool.ResultsNum(), 1)

	// number of receipts corresponding to their share result also should be
	// equal to two after the successful insertion
	assert.Equal(suite.T(), suite.mempool.ReceiptsCount(er1.ExecutionResult.Hash()), 2)
}

// TestGettingResultWithNilInput tests both Getters of mempool for nil inputs
func (suite *ERStoreTestSuit) TestGettingResultWithNilInput() {
	list, err := suite.mempool.GetExecutionReceipts(nil)
	assert.Nil(suite.T(), list)
	assert.NotNil(suite.T(), err)

	result, err := suite.mempool.GetExecutionResult(nil)
	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

// TestGettingNonMembership tests both Getters of mempool for inputs that do not exist in the mempool
func (suite *ERStoreTestSuit) TestGettingNonMembership() {
	// generating a random ER that does not exist in the mempool
	er := randomER()
	hash := er.ExecutionResult.Hash()

	// no ER should be in the mempool for looking up this hash
	list, err := suite.mempool.GetExecutionReceipts(hash)
	assert.Nil(suite.T(), list)
	assert.NotNil(suite.T(), err)

	// no result should be in the mempool for looking up this hash
	result, err := suite.mempool.GetExecutionResult(hash)
	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
}

// TestGettingSingleER stores and then retrieves a single ER from the mempool
func (suite *ERStoreTestSuit) TestGettingSingleER() {
	// generating a random ER that does not exist in the mempool
	er := randomER()
	hash := er.ExecutionResult.Hash()

	// storing ER in the mempool
	suite.mempool.Put(er)

	// retrieving er and its result from mempool
	list, err := suite.mempool.GetExecutionReceipts(hash)
	assert.Equal(suite.T(), *er, list[0])
	assert.Nil(suite.T(), err)

	result, err := suite.mempool.GetExecutionResult(hash)
	require.NotNil(suite.T(), result)
	assert.Equal(suite.T(), er.ExecutionResult, *result)
	assert.Nil(suite.T(), err)
}

// TestGettingTwoERs stores and then retrieves two distinct ERs from the mempool
func (suite *ERStoreTestSuit) TestGettingTwoERs() {
	// generating two random ERs
	er1 := randomER()

	er2 := randomER()

	// two random ERs require to be distinct even in their result part
	require.NotEqual(suite.T(), er1, er2)
	require.NotEqual(suite.T(), er1.ExecutionResult, er2.ExecutionResult)

	erlist := []*exec.ExecutionReceipt{er1, er2}

	for _, er := range erlist {
		// adding two ERs to the mempool
		suite.mempool.Put(er)
	}

	for _, er := range erlist {
		hash := er.ExecutionResult.Hash()
		// retrieving the ER and its result from mempool
		list, err := suite.mempool.GetExecutionReceipts(hash)
		assert.Equal(suite.T(), *er, list[0])
		assert.Nil(suite.T(), err)

		result, err := suite.mempool.GetExecutionResult(hash)
		assert.Equal(suite.T(), er.ExecutionResult, *result)
		assert.Nil(suite.T(), err)
	}
}

// TestGettingTwoERsWithSharedResult stores and then retrieves two distinct ERs from the mempool
func (suite *ERStoreTestSuit) TestGettingTwoERsWithSharedResult() {
	// generating two random ERs
	er1 := randomER()
	er2 := randomER()
	// overriding the result part of er1 to er2 so making both
	// of the same result
	er2.ExecutionResult = er1.ExecutionResult

	// two random ERs require to be distinct but with identical result part
	require.NotEqual(suite.T(), er1, er2)
	require.Equal(suite.T(), er1.ExecutionResult, er2.ExecutionResult)

	erlist := []*exec.ExecutionReceipt{er1, er2}

	for _, er := range erlist {
		// adding two ERs to the mempool
		suite.mempool.Put(er)
	}

	for index, er := range erlist {
		hash := er.ExecutionResult.Hash()
		// retrieving the ER and its result from mempool
		list, err := suite.mempool.GetExecutionReceipts(hash)
		assert.Equal(suite.T(), *er, list[index])
		assert.Nil(suite.T(), err)

		result, err := suite.mempool.GetExecutionResult(hash)
		assert.Equal(suite.T(), er.ExecutionResult, *result)
		assert.Nil(suite.T(), err)
	}
}

// TestDeadLock tests the implementation of mempool against indefinite waits
// that may happen due to the mutex locks
// The tests makes sure that several instances of each method of mempool are
// able to be invoked, executed, and terminated within a bounded interval
func (suite *ERStoreTestSuit) TestDeadlock() {
	// generating a random ER that does not exist in the mempool
	er := randomER()
	hash := er.ExecutionResult.Hash()
	wg := sync.WaitGroup{}

	for i := 0; i < 10; i++ {
		// in each iteration, we spawn 5 goroutines, hence, we need
		// to lock the wait group on 5
		wg.Add(5)
		go func() {
			defer wg.Done()
			suite.mempool.GetExecutionResult(nil)
		}()
		go func() {
			defer wg.Done()
			suite.mempool.GetExecutionReceipts(nil)
		}()
		go func() {
			defer wg.Done()
			suite.mempool.Put(er)
		}()
		go func() {
			defer wg.Done()
			suite.mempool.ReceiptsCount(hash)
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

// randomER generates a random ExecutionReceipt
// The following fields of the generated receipt are chosen randomly
// PreviousExecutionResultHash
// BlockHash
// FinalStateCommitment
// The rest of bytes are chosen as nil
func randomER() *exec.ExecutionReceipt {
	previousER := make([]byte, 32)
	blockHash := make([]byte, 32)
	stateComm := make([]byte, 32)
	executorSignature := make([]byte, 32)
	rand.Read(previousER)
	rand.Read(blockHash)
	rand.Read(stateComm)
	rand.Read(executorSignature)
	return &exec.ExecutionReceipt{
		ExecutionResult: exec.ExecutionResult{
			PreviousExecutionResultHash: crypto.BytesToHash(previousER),
			BlockHash:                   crypto.BytesToHash(blockHash),
			FinalStateCommitment:        stateComm,
			Chunks:                      nil,
			Signatures:                  nil,
		},
		Spocks:            nil,
		ExecutorSignature: executorSignature,
	}
}
