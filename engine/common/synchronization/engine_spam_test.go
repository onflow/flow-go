package synchronization

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/chainsync"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/rand"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestLoad_Process_SyncRequest_HigherThanReceiver_OutsideTolerance_AlwaysReportSpam is a load test that ensures that
// a misbehavior report is generated every time when the probability factor is set to 1.0.
// It checks that a sync request that's higher than the receiver's height doesn't trigger a response, even if outside tolerance.
func (ss *SyncSuite) TestLoad_Process_SyncRequest_HigherThanReceiver_OutsideTolerance_AlwaysReportSpam() {
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(ss.T(), context.Background())
	ss.e.Start(ctx)
	unittest.AssertClosesBefore(ss.T(), ss.e.Ready(), time.Second)
	// Stop the engine and wait for its worker routines to exit before the test returns. Otherwise,
	// leaked workers would access suite fields, racing with the next test's SetupTest.
	defer func() {
		cancel()
		unittest.AssertClosesBefore(ss.T(), ss.e.Done(), time.Second)
	}()

	load := 1000

	// reset misbehavior report counter for each subtest
	misbehaviorsCounter := 0

	// if request height is higher than local finalized, we should not respond
	reqHeight := ss.head.Height + 1

	// Register loop-invariant mock expectations once, before the load loop. Registering them per
	// iteration accumulates thousands of expected-call entries, which makes the mock's call matching
	// quadratic in the load and slows this test down drastically.
	ss.core.On("HandleHeight", ss.head, reqHeight)
	ss.core.On("WithinTolerance", ss.head, reqHeight).Return(false)

	// maybe function calls that might or might not occur over the course of the load test
	ss.core.On("ScanPending", ss.head).Return([]chainsync.Range{}, []chainsync.Batch{}).Maybe()
	ss.con.On("Multicast", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	// count misbehavior reports over the course of a load test
	ss.con.On("ReportMisbehavior", mock.Anything).Return(mock.Anything).Run(
		func(args mock.Arguments) {
			misbehaviorsCounter++
		},
	)

	// force creating misbehavior report by setting syncRequestProb to 1.0 (i.e. report misbehavior 100% of the time)
	ss.e.spamDetectionConfig.syncRequestProb = 1.0

	ss.metrics.On("MessageReceived", metrics.EngineSynchronization, metrics.MessageSyncRequest).Times(load)

	for i := 0; i < load; i++ {
		// generate origin and request message
		originID := unittest.IdentifierFixture()

		nonce, err := rand.Uint64()
		require.NoError(ss.T(), err, "should generate nonce")

		req := &flow.SyncRequest{
			Nonce:  nonce,
			Height: reqHeight,
		}

		require.NoError(ss.T(), ss.e.Process(channels.SyncCommittee, originID, req))
	}

	ss.con.AssertNotCalled(ss.T(), "Unicast", mock.Anything, mock.Anything)
	ss.core.AssertExpectations(ss.T())
	ss.con.AssertExpectations(ss.T())
	ss.metrics.AssertExpectations(ss.T())
	assert.Equal(ss.T(), misbehaviorsCounter, load) // should generate misbehavior report every time
}

// TestLoad_Process_SyncRequest_HigherThanReceiver_OutsideTolerance_SometimesReportSpam is a load test that ensures that a
// misbehavior report is generated an appropriate range of times when the probability factor is set to different values.
// It checks that a sync request that's higher than the receiver's height doesn't trigger a response, even if
// outside tolerance.
func (ss *SyncSuite) TestLoad_Process_SyncRequest_HigherThanReceiver_OutsideTolerance_SometimesReportSpam() {
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(ss.T(), context.Background())
	ss.e.Start(ctx)
	unittest.AssertClosesBefore(ss.T(), ss.e.Ready(), time.Second)
	// Stop the engine and wait for its worker routines to exit before the test returns. Otherwise,
	// leaked workers would access suite fields, racing with the next test's SetupTest.
	defer func() {
		cancel()
		unittest.AssertClosesBefore(ss.T(), ss.e.Done(), time.Second)
	}()

	load := 1000

	// each load test is a load group that contains a set of factors with unique values to test how many misbehavior reports are generated
	// Due to the probabilistic nature of how misbehavior reports are generated, we use an expected lower and
	// upper range of expected misbehaviors to determine if the load test passed or failed. As long as the number of misbehavior reports
	// falls within the expected range, the load test passes.
	type loadGroup struct {
		syncRequestProbabilityFactor float32 // probability factor that will be used to generate misbehavior reports
		expectedMisbehaviorsLower    int     // lower range of expected misbehavior reports
		expectedMisbehaviorsUpper    int     // upper range of expected misbehavior reports
	}

	loadGroups := []loadGroup{}

	// These load tests are wiring smoke tests: the exact decision boundary of the probabilistic
	// reporting is unit-tested deterministically in `TestShouldReportProbabilistically`, so we only
	// keep the groups with discriminating power here. The bounds are the exact quantiles of the
	// Binomial(1000, p) distribution of the misbehavior report count, such that the probability of
	// the count falling outside the bounds is at most 1e-9 per tail. This keeps the test meaningful
	// while making failures of a correct implementation practically impossible (previous, tighter
	// bounds caused flakiness; low-probability groups were removed because their lower bound of 0
	// could not discriminate at all).

	// expect to never get misbehavior report
	loadGroups = append(loadGroups, loadGroup{0.0, 0, 0})

	// expect to get misbehavior report about 50% of the time
	loadGroups = append(loadGroups, loadGroup{0.5, 405, 595})

	// reset misbehavior report counter for each subtest
	misbehaviorsCounter := 0

	for _, loadGroup := range loadGroups {
		ss.T().Run(fmt.Sprintf("load test; pfactor=%f lower=%d upper=%d", loadGroup.syncRequestProbabilityFactor, loadGroup.expectedMisbehaviorsLower, loadGroup.expectedMisbehaviorsUpper), func(t *testing.T) {
			// if request height is higher than local finalized, we should not respond
			reqHeight := ss.head.Height + 1

			// Register loop-invariant mock expectations once, before the load loop. Registering them per
			// iteration accumulates thousands of expected-call entries, which makes the mock's call matching
			// quadratic in the load and slows this test down drastically.
			ss.core.On("HandleHeight", ss.head, reqHeight)
			ss.core.On("WithinTolerance", ss.head, reqHeight).Return(false)

			// maybe function calls that might or might not occur over the course of the load test
			ss.core.On("ScanPending", ss.head).Return([]chainsync.Range{}, []chainsync.Batch{}).Maybe()
			ss.con.On("Multicast", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

			// count misbehavior reports over the course of a load test
			ss.con.On("ReportMisbehavior", mock.Anything).Return(mock.Anything).Maybe().Run(
				func(args mock.Arguments) {
					misbehaviorsCounter++
				},
			)
			ss.e.spamDetectionConfig.syncRequestProb = loadGroup.syncRequestProbabilityFactor
			ss.metrics.On("MessageSent", metrics.EngineSynchronization, metrics.MessageSyncRequest).Maybe()
			ss.metrics.On("MessageReceived", metrics.EngineSynchronization, metrics.MessageSyncRequest).Times(load)

			for i := 0; i < load; i++ {
				nonce, err := rand.Uint64()
				require.NoError(ss.T(), err, "should generate nonce")

				// generate origin and request message
				originID := unittest.IdentifierFixture()
				req := &flow.SyncRequest{
					Nonce:  nonce,
					Height: reqHeight,
				}

				require.NoError(ss.T(), ss.e.Process(channels.SyncCommittee, originID, req))
			}

			ss.con.AssertNotCalled(ss.T(), "Unicast", mock.Anything, mock.Anything)

			// check function call expectations at the end of the load test; otherwise, load test would take much longer
			ss.core.AssertExpectations(ss.T())
			ss.con.AssertExpectations(ss.T())
			ss.metrics.AssertExpectations(ss.T())

			// check that correct range of misbehavior reports were generated
			// since we're using a probabilistic approach to generate misbehavior reports, we can't guarantee the exact number,
			// so we check that it's within an expected range
			ss.T().Logf("misbehaviors counter after load test: %d (expected lower bound: %d expected upper bound: %d)", misbehaviorsCounter, loadGroup.expectedMisbehaviorsLower, loadGroup.expectedMisbehaviorsUpper)
			assert.GreaterOrEqual(ss.T(), misbehaviorsCounter, loadGroup.expectedMisbehaviorsLower)
			assert.LessOrEqual(ss.T(), misbehaviorsCounter, loadGroup.expectedMisbehaviorsUpper)

			misbehaviorsCounter = 0 // reset counter for next subtest
		})
	}
}

// TestLoad_Process_RangeRequest_SometimesReportSpam is a load test that ensures that a misbehavior report is generated
// an appropriate range of times when the base probability factor and range are set to different values.
func (ss *SyncSuite) TestLoad_Process_RangeRequest_SometimesReportSpam() {
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(ss.T(), context.Background())
	ss.e.Start(ctx)
	unittest.AssertClosesBefore(ss.T(), ss.e.Ready(), time.Second)
	// Stop the engine and wait for its worker routines to exit before the test returns. Otherwise,
	// leaked workers would access suite fields, racing with the next test's SetupTest.
	defer func() {
		cancel()
		unittest.AssertClosesBefore(ss.T(), ss.e.Done(), time.Second)
	}()

	load := 1000

	// each load test is a load group that contains a set of factors with unique values to test how many misbehavior reports are generated.
	// Due to the probabilistic nature of how misbehavior reports are generated, we use an expected lower and
	// upper range of expected misbehaviors to determine if the load test passed or failed. As long as the number of misbehavior reports
	// falls within the expected range, the load test passes.
	type loadGroup struct {
		rangeRequestBaseProb      float32 // base probability factor that will be used to calculate the final probability factor
		expectedMisbehaviorsLower int     // lower range of expected misbehavior reports
		expectedMisbehaviorsUpper int     // upper range of expected misbehavior reports
		fromHeight                uint64  // from height of the range request
		toHeight                  uint64  // to height of the range request
	}

	loadGroups := []loadGroup{}

	// These load tests are wiring smoke tests: the exact decision boundary and the range-scaling
	// formula are unit-tested deterministically in `TestShouldReportProbabilistically` and
	// `TestRangeRequestMisbehaviorProbability`, so we only keep the groups with discriminating
	// power here. The bounds are the exact quantiles of the Binomial(1000, p) distribution of the
	// misbehavior report count (with p being the expected probability factor), such that the
	// probability of the count falling outside the bounds is at most 1e-9 per tail. This keeps the
	// test meaningful while making failures of a correct implementation practically impossible
	// (previous, tighter bounds caused flakiness; low-probability groups were removed because
	// their lower bound of 0 could not discriminate at all).

	// using a large range (99) with a 10% base probability factor, expect to get misbehavior report about 15% of the time (150 in 1000 requests)
	// expected probability factor: 0.1 * (99 + 1)/64 = 0.15625
	loadGroups = append(loadGroups, loadGroup{0.1, 92, 229, 1, 100})

	// ALWAYS REPORT SPAM FOR INVALID RANGE REQUESTS OR RANGE REQUESTS THAT ARE FAR OUTSIDE OF THE TOLERANCE

	// using an inverted range (from height > to height) always results in a misbehavior report, no matter how small the range is or how small the base probability factor is
	loadGroups = append(loadGroups, loadGroup{0.001, 1000, 1000, 2, 1})

	// using a very large range (999) with a 10% base probability factor, expect to get misbehavior report 100% of the time (1000 in 1000 requests)
	// expected probability factor: 0.1 * ((1000-1) + 1)/64 = 1.5625
	loadGroups = append(loadGroups, loadGroup{0.1, 1000, 1000, 1, 1000})

	// reset misbehavior report counter for each subtest
	misbehaviorsCounter := 0

	// Register loop-invariant mock expectations once, before the load loops. Registering them per
	// iteration accumulates thousands of expected-call entries, which makes the mock's call matching
	// quadratic in the load and slows this test down drastically.

	// maybe function calls that might or might not occur over the course of the load test
	ss.core.On("ScanPending", ss.head).Return([]chainsync.Range{}, []chainsync.Batch{}).Maybe()
	ss.con.On("Multicast", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	ss.metrics.On("MessageSent", metrics.EngineSynchronization, metrics.MessageSyncRequest).Maybe()

	// count misbehavior reports over the course of a load test
	ss.con.On("ReportMisbehavior", mock.Anything).Return(mock.Anything).Maybe().Run(
		func(args mock.Arguments) {
			misbehaviorsCounter++
		},
	)

	for _, loadGroup := range loadGroups {
		ss.e.spamDetectionConfig.rangeRequestBaseProb = loadGroup.rangeRequestBaseProb
		ss.metrics.On("MessageReceived", metrics.EngineSynchronization, metrics.MessageRangeRequest).Times(load)

		for i := 0; i < load; i++ {
			nonce, err := rand.Uint64()
			require.NoError(ss.T(), err, "should generate nonce")

			// generate origin and request message
			originID := unittest.IdentifierFixture()
			req := &flow.RangeRequest{
				Nonce:      nonce,
				FromHeight: loadGroup.fromHeight,
				ToHeight:   loadGroup.toHeight,
			}

			require.NoError(ss.T(), ss.e.Process(channels.SyncCommittee, originID, req))
		}
		// check function call expectations at the end of the load test; otherwise, load test would take much longer
		ss.core.AssertExpectations(ss.T())
		ss.con.AssertExpectations(ss.T())
		ss.metrics.AssertExpectations(ss.T())

		// check that correct range of misbehavior reports were generated
		// since we're using a probabilistic approach to generate misbehavior reports, we can't guarantee the exact number,
		// so we check that it's within an expected range
		ss.T().Logf("misbehaviors counter after load test: %d (expected lower bound: %d expected upper bound: %d)", misbehaviorsCounter, loadGroup.expectedMisbehaviorsLower, loadGroup.expectedMisbehaviorsUpper)
		assert.GreaterOrEqual(ss.T(), misbehaviorsCounter, loadGroup.expectedMisbehaviorsLower)
		assert.LessOrEqual(ss.T(), misbehaviorsCounter, loadGroup.expectedMisbehaviorsUpper)

		misbehaviorsCounter = 0 // reset counter for next subtest
	}
}

// TestLoad_Process_BatchRequest_SometimesReportSpam is a load test that ensures that a misbehavior report is generated
// an appropriate range of times when the base probability factor and number of block IDs are set to different values.
func (ss *SyncSuite) TestLoad_Process_BatchRequest_SometimesReportSpam() {
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(ss.T(), context.Background())
	ss.e.Start(ctx)
	unittest.AssertClosesBefore(ss.T(), ss.e.Ready(), time.Second)
	// Stop the engine and wait for its worker routines to exit before the test returns. Otherwise,
	// leaked workers would access suite fields, racing with the next test's SetupTest.
	defer func() {
		cancel()
		unittest.AssertClosesBefore(ss.T(), ss.e.Done(), time.Second)
	}()

	load := 1000

	// each load test is a load group that contains a set of factors with unique values to test how many misbehavior reports are generated.
	// Due to the probabilistic nature of how misbehavior reports are generated, we use an expected lower and
	// upper range of expected misbehaviors to determine if the load test passed or failed. As long as the number of misbehavior reports
	// falls within the expected range, the load test passes.
	type loadGroup struct {
		batchRequestBaseProb      float32
		expectedMisbehaviorsLower int
		expectedMisbehaviorsUpper int
		blockIDs                  []flow.Identifier
	}

	loadGroups := []loadGroup{}

	// These load tests are wiring smoke tests: the exact decision boundary and the batch-scaling
	// formula are unit-tested deterministically in `TestShouldReportProbabilistically` and
	// `TestBatchRequestMisbehaviorProbability`, so we only keep the groups with discriminating
	// power here. The bounds are the exact quantiles of the Binomial(1000, p) distribution of the
	// misbehavior report count (with p being the expected probability factor), such that the
	// probability of the count falling outside the bounds is at most 1e-9 per tail. This keeps the
	// test meaningful while making failures of a correct implementation practically impossible
	// (previous, tighter bounds caused flakiness; low-probability groups were removed because
	// their lower bound of 0 could not discriminate at all).

	// using a large batch request (99 block IDs) with a 10% base probability factor, expect to get misbehavior report about 15% of the time (150 in 1000 requests)
	// expected probability factor: 0.1 * (99 + 1)/64 = 0.15625
	loadGroups = append(loadGroups, loadGroup{0.1, 92, 229, repeatedBlockIDs(99)})

	// ALWAYS REPORT SPAM FOR INVALID BATCH REQUESTS OR BATCH REQUESTS THAT ARE FAR OUTSIDE OF THE TOLERANCE

	// using an empty batch request (0 block IDs) always results in a misbehavior report, no matter how small the base probability factor is
	loadGroups = append(loadGroups, loadGroup{0.001, 1000, 1000, []flow.Identifier{}})

	// using a very large batch request (999 block IDs) with a 10% base probability factor, expect to get misbehavior report 100% of the time (1000 in 1000 requests)
	// expected probability factor: 0.1 * ((999 + 1)/64 = 1.5625
	loadGroups = append(loadGroups, loadGroup{0.1, 1000, 1000, repeatedBlockIDs(999)})

	// reset misbehavior report counter for each subtest
	misbehaviorsCounter := 0

	// Register loop-invariant mock expectations once, before the load loops. Registering them per
	// iteration accumulates thousands of expected-call entries, which makes the mock's call matching
	// quadratic in the load and slows this test down drastically.

	// maybe function calls that might or might not occur over the course of the load test
	ss.core.On("ScanPending", ss.head).Return([]chainsync.Range{}, []chainsync.Batch{}).Maybe()
	ss.con.On("Multicast", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	ss.metrics.On("MessageSent", metrics.EngineSynchronization, metrics.MessageSyncRequest).Maybe()

	// count misbehavior reports over the course of a load test
	ss.con.On("ReportMisbehavior", mock.Anything).Return(mock.Anything).Maybe().Run(
		func(args mock.Arguments) {
			misbehaviorsCounter++
		},
	)

	for _, loadGroup := range loadGroups {
		ss.e.spamDetectionConfig.batchRequestBaseProb = loadGroup.batchRequestBaseProb
		ss.metrics.On("MessageReceived", metrics.EngineSynchronization, metrics.MessageBatchRequest).Times(load)

		for i := 0; i < load; i++ {
			nonce, err := rand.Uint64()
			require.NoError(ss.T(), err, "should generate nonce")

			// generate origin and request message
			originID := unittest.IdentifierFixture()
			req := &flow.BatchRequest{
				Nonce:    nonce,
				BlockIDs: loadGroup.blockIDs,
			}

			require.NoError(ss.T(), ss.e.Process(channels.SyncCommittee, originID, req))
		}
		// check function call expectations at the end of the load test; otherwise, load test would take much longer
		ss.core.AssertExpectations(ss.T())
		ss.con.AssertExpectations(ss.T())
		ss.metrics.AssertExpectations(ss.T())

		// check that correct range of misbehavior reports were generated
		// since we're using a probabilistic approach to generate misbehavior reports, we can't guarantee the exact number,
		// so we check that it's within an expected range
		ss.T().Logf("misbehaviors counter after load test: %d (expected lower bound: %d expected upper bound: %d)", misbehaviorsCounter, loadGroup.expectedMisbehaviorsLower, loadGroup.expectedMisbehaviorsUpper)
		assert.GreaterOrEqual(ss.T(), misbehaviorsCounter, loadGroup.expectedMisbehaviorsLower)
		assert.LessOrEqual(ss.T(), misbehaviorsCounter, loadGroup.expectedMisbehaviorsUpper)

		misbehaviorsCounter = 0 // reset counter for next subtest
	}
}

func repeatedBlockIDs(n int) []flow.Identifier {
	blockID := unittest.BlockFixture().ID()

	arr := make([]flow.Identifier, n)
	for i := range n {
		arr[i] = blockID
	}
	return arr
}
