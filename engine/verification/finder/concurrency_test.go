package finder_test

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/engine/verification/utils"
	"github.com/dapperlabs/flow-go/model/flow"
	network "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// TestConcurrency evaluates behavior of finder engine against:
// - finder engine receives concurrent receipts from different sources
// - for each distinct receipt with an available block finder engine emits it to the matching engine
// Each test case is tried with a scenario where block goes first then receipt, and vice versa.
func TestConcurrency(t *testing.T) {
	var mu sync.Mutex
	testcases := []struct {
		erCount, // number of execution receipts
		senderCount, // number of (concurrent) senders for each execution receipt
		chunksNum int // number of chunks in each execution receipt
	}{
		{
			erCount:     1,
			senderCount: 1,
			chunksNum:   2,
		},
		{
			erCount:     1,
			senderCount: 5,
			chunksNum:   2,
		},
		{
			erCount:     5,
			senderCount: 1,
			chunksNum:   2,
		},
		{
			erCount:     5,
			senderCount: 5,
			chunksNum:   2,
		},
		{
			erCount:     1,
			senderCount: 1,
			chunksNum:   10,
		},
		{
			erCount:     2,
			senderCount: 5,
			chunksNum:   4,
		},
		{
			erCount:     1,
			senderCount: 1,
			chunksNum:   2,
		},
	}

	for _, blockFirst := range []bool{true, false} {
		for _, tc := range testcases {
			t.Run(fmt.Sprintf("%d-ers/%d-senders/%d-chunks",
				tc.erCount, tc.senderCount, tc.chunksNum), func(t *testing.T) {
				mu.Lock()
				defer mu.Unlock()
				testConcurrency(t, tc.erCount, tc.senderCount, tc.chunksNum, blockFirst)
			})
		}
	}
}

// testConcurrency sends `erCount` many execution receipts each with `chunkNum` many chunks, concurrently by
// `senderCount` sender to the verification node.
// If blockFirst is true, the block arrives at verification node earlier than the receipt.
// Otherwise, the block arrives after the receipt.
// This test successfully is passed if each unique execution result is passed to Match engine by the Finder engine
// in verification node. It also checks the result is marked as processed, and the receipts with process results are
// cleaned up.
func testConcurrency(t *testing.T, erCount, senderCount, chunksNum int, blockFirst bool) {
	log := zerolog.New(os.Stderr).Level(zerolog.DebugLevel)
	// to demarcate the logs
	t.Logf("TestConcurrencyStarted: %d-receipts/%d-senders/%d-chunks", erCount, senderCount, chunksNum)
	log.Debug().
		Int("execution_receipt_count", erCount).
		Int("sender_count", senderCount).
		Int("chunks_num", chunksNum).
		Msg("TestConcurrency started")
	hub := stub.NewNetworkHub()

	// creates test id for each role
	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	conID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	identities := flow.IdentityList{colID, conID, exeID, verID}

	// create `erCount` execution receipt fixtures that will be concurrently delivered
	ers := make([]utils.CompleteExecutionResult, erCount)
	results := make([]flow.ExecutionResult, erCount)
	for i := 0; i < erCount; i++ {
		er := utils.LightExecutionResultFixture(chunksNum)
		ers[i] = er
		results[i] = er.Receipt.ExecutionResult
	}

	// set up mock matching engine that asserts each receipt is submitted exactly once.
	requestInterval := uint(1000)
	failureThreshold := uint(2)
	matchEng, matchEngWG := SetupMockMatchEng(t, exeID, results)
	assigner := utils.NewMockAssigner(verID.NodeID, IsAssigned)

	// creates a verification node with a real finder engine, and mock matching engine.
	verNode := testutil.VerificationNode(t, hub, verID, identities,
		assigner, requestInterval, failureThreshold, testutil.WithMatchEngine(matchEng))

	// the wait group tracks goroutines for each Execution Receipt sent to finder engine
	var senderWG sync.WaitGroup
	senderWG.Add(erCount * senderCount)

	// blockStorageLock is needed to provide a concurrency-safe imitation of consensus follower engine
	var blockStorageLock sync.Mutex

	for _, completeER := range ers {
		// spins up `senderCount` sender goroutines to mimic receiving concurrent execution receipts of same copies
		for i := 0; i < senderCount; i++ {
			go func(j int, id flow.Identifier, block *flow.Block, receipt *flow.ExecutionReceipt) {

				// sendBlock makes the block associated with the receipt available to the
				// follower engine of the verification node
				sendBlock := func() {
					// adds the block to the storage of the node
					// Note: this is done by the follower
					// this block should be done in a thread-safe way
					blockStorageLock.Lock()
					// we don't check for error as it definitely returns error when we
					// have duplicate blocks, however, this is not the concern for this test
					_ = verNode.Blocks.Store(block)
					blockStorageLock.Unlock()

					// casts block into a Hotstuff block for notifier
					hotstuffBlock := &model.Block{
						BlockID:     block.ID(),
						View:        block.Header.View,
						ProposerID:  block.Header.ProposerID,
						QC:          nil,
						PayloadHash: block.Header.PayloadHash,
						Timestamp:   block.Header.Timestamp,
					}
					verNode.FinderEngine.OnFinalizedBlock(hotstuffBlock)
				}

				// sendReceipt sends the execution receipt to the finder engine of verification node
				sendReceipt := func() {
					err := verNode.FinderEngine.Process(exeID.NodeID, receipt)
					require.NoError(t, err)
				}

				if blockFirst {
					// block then receipt
					sendBlock()
					// allows another goroutine to run before sending receipt
					time.Sleep(time.Nanosecond)
					sendReceipt()
				} else {
					// receipt then block
					sendReceipt()
					// allows another goroutine to run before sending block
					time.Sleep(time.Nanosecond)
					sendBlock()
				}

				senderWG.Done()
			}(i, completeER.Receipt.ExecutionResult.ID(), completeER.Block, completeER.Receipt)
		}
	}

	// waits for all receipts to be sent to verification node
	unittest.RequireReturnsBefore(t, senderWG.Wait, time.Duration(senderCount*chunksNum*erCount*5)*time.Second)
	// waits for all distinct execution results sent to matching engine of verification node
	unittest.RequireReturnsBefore(t, matchEngWG.Wait, time.Duration(senderCount*chunksNum*erCount*5)*time.Second)

	// sleeps to make sure that the cleaning of processed execution receipts
	// happens. This sleep is necessary since we are evaluating cleanup right after the sleep.
	time.Sleep(1 * time.Second)

	for _, er := range ers {
		// all distinct execution results should be marked as processed by finder engine
		assert.True(t, verNode.ProcessedResultIDs.Has(er.Receipt.ExecutionResult.ID()))

		// no execution receipt should reside in mempool of finder engine
		assert.False(t, verNode.PendingReceipts.Has(er.Receipt.ID()))
	}

	verNode.Done()

	// to demarcate the logs
	log.Debug().
		Int("execution_receipt_count", erCount).
		Int("sender_count", senderCount).
		Int("chunks_num", chunksNum).
		Msg("TestConcurrency finished")
}

// SetupMockMatchEng sets up a mock match engine that asserts the followings:
// - that a set of execution results are delivered to it.
// - that each execution result is delivered only once.
// SetupMockMatchEng returns the mock engine and a wait group that unblocks when all results are received.
func SetupMockMatchEng(t testing.TB, exeID *flow.Identity, ers []flow.ExecutionResult) (*network.Engine, *sync.WaitGroup) {
	eng := new(network.Engine)

	// keeps track of which execution results it has received
	receivedResults := make(map[flow.Identifier]struct{})
	var (
		// decrements the wait group per distinct execution result received
		wg sync.WaitGroup
		// serializes processing received execution receipts
		mu sync.Mutex
	)

	// expects `len(er)` many distinct execution results
	wg.Add(len(ers))

	eng.On("Process", testifymock.Anything, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			mu.Lock()
			defer mu.Unlock()

			// origin ID of event should be exection node
			originID, ok := args[0].(flow.Identifier)
			assert.True(t, ok)
			assert.Equal(t, originID, exeID.NodeID)

			// the received entity should be an execution result
			result, ok := args[1].(*flow.ExecutionResult)
			assert.True(t, ok)

			resultID := result.ID()

			// verifies that it has not seen this result
			_, alreadySeen := receivedResults[resultID]
			if alreadySeen {
				t.Logf("match engine received duplicate ER (id=%s)", resultID)
				t.Fail()
				return
			}

			// ensures the received result matches one we expect
			for _, er := range ers {
				if resultID == er.ID() {
					// mark it as seen and decrement the waitgroup
					receivedResults[resultID] = struct{}{}
					wg.Done()
					return
				}
			}

			// the received result doesn't match any expected result
			t.Logf("match engine received unexpected results (id=%s)", resultID)
			t.Fail()
		}).
		Return(nil)

	return eng, &wg
}

// IsAssigned is a helper function that returns true for the even indices in [0, chunkNum-1]
func IsAssigned(index uint64) bool {
	return index%2 == 0
}
