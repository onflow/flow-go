package finder_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/engine/testutil/mock"
	"github.com/onflow/flow-go/engine/verification/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/utils/unittest"
)

// ConcurrencyTestSuite encapsulates tests for happy and unhappy paths of concurrently sending several receipts to finder engine.
type ConcurrencyTestSuite struct {
	suite.Suite
	verID        *flow.Identity
	exeID        *flow.Identity
	participants flow.IdentityList
	log          zerolog.Logger
	tracer       module.Tracer
	collector    *metrics.NoopCollector
	stateFixture *mock.StateFixture
}

// TestFinderEngine executes all FinderEngineTestSuite tests.
func TestConcurrencyTestSuite(t *testing.T) {
	suite.Run(t, new(ConcurrencyTestSuite))
}

// SetupTest is executed before each test in this test suite.
func (suite *ConcurrencyTestSuite) SetupTest() {
	tracer, err := trace.NewTracer(suite.log, "test")
	require.NoError(suite.T(), err)
	suite.tracer = tracer
	suite.collector = metrics.NewNoopCollector()
}

// TestConcurrency evaluates behavior of finder engine against:
// - finder engine receives concurrent receipts from different sources
// - in a staked verification node:
// -- for each distinct result with an available block finder engine emits it to the match engine.
// - in an unstaked verification node:
// -- marks results of receipts as discarded, and does not emit any result to match engine.
// - it does a correct resource clean up of the pipeline after handling all incoming receipts
// Each test case is tried with a scenario where block goes first then receipt, and vice versa.
func (suite *ConcurrencyTestSuite) TestConcurrency() {
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
	}

	// runs each test case in block-first and receipt-first mode.
	for _, blockFirst := range []bool{true, false} {
		// runs each test case in staked and unstaked verification node.
		for _, staked := range []bool{true, false} {
			for _, tc := range testcases {
				suite.T().Run(fmt.Sprintf("%d-ers/%d-senders/%d-chunks/%t-block-first/%t-staked",
					tc.erCount, tc.senderCount, tc.chunksNum, blockFirst, staked), func(t *testing.T) {
					mu.Lock()
					defer mu.Unlock()

					suite.testConcurrency(tc.erCount, tc.senderCount, tc.chunksNum, blockFirst, staked)
				})
			}
		}
	}
}

// testConcurrency sends `receiptCount`-many execution receipts each with `chunkCount`-many chunks, concurrently by
// `senderCount`-many senders to the verification node.
//
// If blockFirst is true, the block arrives at verification node earlier than the receipt.
// Otherwise, the block arrives after the receipt.
//
// If staked is true, the verification node is staked for the current epoch, otherwise not.
//
// In case of staked verification node, this test successfully passes if each unique execution result is passed "only once" to
// match engine by the finder engine in verification node. It also checks the result is marked as processed,
// and the receipts with process results are cleaned up.
//
// In case of an unstaked verification node, this test successfully passes if no execution result is passed from finder to match engine, no
// result is marked as processed, and rather all results are marked as discarded.
//
// In both cases of staked and unstaked tests, it also evaluates that the cached-pending-ready pipeline of finder engine is
// cleaned up completely.
func (suite *ConcurrencyTestSuite) testConcurrency(receiptCount, senderCount, chunkCount int, blockFirst bool, staked bool) {
	// to demarcate the logs
	suite.T().Logf("TestConcurrencyStarted: %d-receipts/%d-senders/%d-chunks", receiptCount, senderCount, chunkCount)
	suite.log.Debug().
		Int("execution_receipt_count", receiptCount).
		Int("sender_count", senderCount).
		Int("chunks_num", chunkCount).
		Msg("TestConcurrency started")

	// bootstraps the system, creates a generic node, and a verification node out of the generic node.
	suite.bootstrapSystem(staked)

	hub := stub.NewNetworkHub()
	chainID := flow.Testnet
	genericNode := testutil.GenericNodeWithStateFixture(suite.T(),
		suite.stateFixture,
		hub,
		suite.verID,
		suite.log,
		suite.collector,
		suite.tracer,
		chainID)

	matchEng := &mocknetwork.Engine{}
	verNode := testutil.VerificationNode(suite.T(),
		hub,
		suite.verID,
		suite.participants,
		utils.NewMockAssigner(suite.verID.NodeID, func(index uint64) bool { return false }), // no assignment is needed.
		1*time.Second,
		1*time.Second,
		uint(receiptCount),
		uint(receiptCount*chunkCount),
		uint(2),
		chainID,
		suite.collector,
		suite.collector,
		testutil.WithGenericNode(&genericNode),
		testutil.WithMatchEngine(matchEng))

	// create `receiptCount` execution receipt fixtures that will be concurrently delivered to finder engine.
	// all receipts are children of `parent` block.
	parent, err := verNode.State.Final().Head()
	require.NoError(suite.T(), err)

	receipts := make([]*utils.CompleteExecutionReceipt, receiptCount)
	results := make([]flow.ExecutionResult, receiptCount)
	for i := 0; i < receiptCount; i++ {
		completeER := utils.CompleteExecutionReceiptFixture(suite.T(), chunkCount, chainID.Chain(), parent)
		receipts[i] = completeER
		results[i] = completeER.Receipts[0].ExecutionResult
	}

	// sets up mock match engine that asserts:
	// - each result is submitted exactly once, if verification node is staked.
	// - no result is submitted, if verification node is unstaked.
	matchEngWG := SetupMockMatchEng(suite.T(), matchEng, suite.exeID, results, staked)

	// starts finder engine of verification node, the rest are not involved in this test.
	<-verNode.FinderEngine.Ready()

	// the wait group tracks goroutines for each execution receipt sent to finder engine
	var senderWG sync.WaitGroup
	senderWG.Add(receiptCount * senderCount)

	// mutatorLock is needed to provide a concurrency-safe imitation of consensus follower engine.
	var mutatorLock sync.Mutex
	for _, completeER := range receipts {
		// spins up `senderCount` sender goroutines to mimic receiving concurrent execution receipts of same copies
		for i := 0; i < senderCount; i++ {
			go func(j int, id flow.Identifier, block *flow.Block, receipt *flow.ExecutionReceipt) {
				// sendBlock makes the block associated with the receipt available to the follower engine of the verification node.
				sendBlock := func() {
					// Note: this is done by the follower.
					mutatorLock.Lock()
					if _, err := verNode.Blocks.ByID(block.ID()); err != nil {
						err = verNode.State.Extend(block)
						require.NoError(suite.T(), err)
					}
					mutatorLock.Unlock()

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

				// sendReceipt sends the execution receipt to the finder engine of verification node.
				sendReceipt := func() {
					err := verNode.FinderEngine.Process(suite.exeID.NodeID, receipt)
					require.NoError(suite.T(), err)
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
			}(i,
				completeER.Receipts[0].ExecutionResult.ID(),
				completeER.ReceiptsData[0].ReferenceBlock,
				completeER.Receipts[0])
		}
	}

	// waits for all receipts to be sent to verification node
	unittest.RequireReturnsBefore(suite.T(), senderWG.Wait, time.Duration(senderCount*chunkCount*receiptCount*5)*time.Second,
		"finder engine process")

	// staked verification node should pass each execution result only once to match engine.
	// waits for all distinct execution results sent to matching engine of verification node.
	//
	// Note: in unstaked mode, matchEngWG wait group has zero counter, so this should pass immediately.
	unittest.RequireReturnsBefore(suite.T(), matchEngWG.Wait, time.Duration(senderCount*chunkCount*receiptCount*5)*time.Second,
		"match engine process")

	// sleeps to make sure that the cleaning of processed execution receipts
	// happens. This sleep is necessary since we are evaluating cleanup right after the sleep.
	time.Sleep(2 * time.Second)

	// stops finder engine of verification node
	<-verNode.FinderEngine.Done()

	if staked {
		// staked verification node should mark all distinct execution results as processed
		for _, result := range results {
			assert.True(suite.T(), verNode.ProcessedResultIDs.Has(result.ID()))
		}
	}

	// evaluates proper resource cleanup
	//
	// no execution receipt should reside in cached, pending, or ready mempools of finder engine
	require.True(suite.T(), verNode.CachedReceipts.Size() == 0)
	require.True(suite.T(), verNode.PendingReceipts.Size() == 0)
	require.True(suite.T(), verNode.ReadyReceipts.Size() == 0)

	// no execution receipt should be pending for a block, and no block should remain cached.
	require.True(suite.T(), verNode.PendingReceiptIDsByBlock.Size() == 0)
	require.True(suite.T(), verNode.ReceiptIDsByResult.Size() == 0)
	require.True(suite.T(), verNode.CachedReceipts.Size() == 0)
	require.True(suite.T(), verNode.BlockIDsCache.Size() == 0)

	if staked {
		// staked finder engine should not discard any result
		require.True(suite.T(), verNode.DiscardedResultIDs.Size() == 0)
	} else {
		// unstaked finder engine should discard all results
		require.True(suite.T(), verNode.DiscardedResultIDs.Size() == uint(len(receipts)))
	}

	verNode.Done()

	// to demarcate the logs
	suite.log.Debug().
		Int("execution_receipt_count", receiptCount).
		Int("sender_count", senderCount).
		Int("chunks_num", chunkCount).
		Msg("TestConcurrency finished")
}

// SetupMockMatchEng sets up a mock match engine that asserts the followings:
// - in a staked verification node:
// -- that a set of execution results are delivered to it.
// -- that each execution result is delivered only once.
// - in an unstaked verification node:
// -- no result is passed to it.
// SetupMockMatchEng returns the mock engine and a wait group that unblocks when all results are received.
func SetupMockMatchEng(t testing.TB,
	eng *mocknetwork.Engine,
	exeID *flow.Identity,
	results []flow.ExecutionResult,
	staked bool) *sync.WaitGroup {
	// keeps track of which execution results it has received
	receivedResults := make(map[flow.Identifier]struct{})
	var (
		// decrements the wait group per distinct execution result received
		wg sync.WaitGroup
		// serializes processing received execution receipts
		mu sync.Mutex
	)

	if staked {
		// in staked mode, it expects `len(result)` many distinct execution results
		wg.Add(len(results))
	}

	eng.On("Process", testifymock.Anything, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			mu.Lock()
			defer mu.Unlock()

			// match engine should only receive a result if the verification node is
			// staked.
			require.True(t, staked, "unstaked match engine received result")

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
			for _, result := range results {
				if resultID == result.ID() {
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

	return &wg
}

// bootstrapSystem bootstraps a flow system with one node of each main roles.
// If staked set to true, it bootstraps verification node as an staked one.
// Otherwise, it bootstraps the verification node as unstaked in current epoch.
func (suite *ConcurrencyTestSuite) bootstrapSystem(staked bool) {
	// creates identities to bootstrap system with
	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	conID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	identities := flow.IdentityList{colID, conID, exeID, verID}

	// bootstraps the system
	stateFixture := testutil.CompleteStateFixture(suite.T(), suite.collector, suite.tracer, identities)

	if !staked {
		// creates a new verification node identity that is unstaked for this epoch
		verID = unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
		identities = identities.Union(flow.IdentityList{verID})

		epochBuilder := unittest.NewEpochBuilder(suite.T(), stateFixture.State)
		epochBuilder.
			UsingSetupOpts(unittest.WithParticipants(identities)).
			BuildEpoch()
	}

	suite.verID = verID
	suite.exeID = exeID
	suite.participants = identities
	suite.stateFixture = stateFixture
}
