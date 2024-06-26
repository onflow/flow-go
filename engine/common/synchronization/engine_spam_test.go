package synchronization

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/onflow/flow-go/model/flow"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/chainsync"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/irrecoverable"
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
	defer cancel()

	load := 1000

	// reset misbehavior report counter for each subtest
	misbehaviorsCounter := 0

	for i := 0; i < load; i++ {
		// generate origin and request message
		originID := unittest.IdentifierFixture()

		nonce, err := rand.Uint64()
		require.NoError(ss.T(), err, "should generate nonce")

		req := &messages.SyncRequest{
			Nonce:  nonce,
			Height: 0,
		}

		// if request height is higher than local finalized, we should not respond
		req.Height = ss.head.Height + 1

		ss.core.On("HandleHeight", ss.head, req.Height)
		ss.core.On("WithinTolerance", ss.head, req.Height).Return(false)
		ss.con.AssertNotCalled(ss.T(), "Unicast", mock.Anything, mock.Anything)

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

		require.NoError(ss.T(), ss.e.Process(channels.SyncCommittee, originID, req))
	}

	ss.core.AssertExpectations(ss.T())
	ss.con.AssertExpectations(ss.T())
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
	defer cancel()

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

	// expect to never get misbehavior report
	loadGroups = append(loadGroups, loadGroup{0.0, 0, 0})

	// expect to get misbehavior report about 0.1% of the time (1 in 1000 requests)
	loadGroups = append(loadGroups, loadGroup{0.001, 0, 7})

	// expect to get misbehavior report about 1% of the time
	loadGroups = append(loadGroups, loadGroup{0.01, 5, 15})

	// expect to get misbehavior report about 10% of the time
	loadGroups = append(loadGroups, loadGroup{0.1, 75, 140})

	// expect to get misbehavior report about 50% of the time
	loadGroups = append(loadGroups, loadGroup{0.5, 450, 550})

	// expect to get misbehavior report about 90% of the time
	loadGroups = append(loadGroups, loadGroup{0.9, 850, 950})

	// reset misbehavior report counter for each subtest
	misbehaviorsCounter := 0

	for _, loadGroup := range loadGroups {
		ss.T().Run(fmt.Sprintf("load test; pfactor=%f lower=%d upper=%d", loadGroup.syncRequestProbabilityFactor, loadGroup.expectedMisbehaviorsLower, loadGroup.expectedMisbehaviorsUpper), func(t *testing.T) {
			for i := 0; i < load; i++ {
				ss.T().Log("load iteration", i)
				nonce, err := rand.Uint64()
				require.NoError(ss.T(), err, "should generate nonce")

				// generate origin and request message
				originID := unittest.IdentifierFixture()
				req := &messages.SyncRequest{
					Nonce:  nonce,
					Height: 0,
				}

				// if request height is higher than local finalized, we should not respond
				req.Height = ss.head.Height + 1

				ss.core.On("HandleHeight", ss.head, req.Height)
				ss.core.On("WithinTolerance", ss.head, req.Height).Return(false)
				ss.con.AssertNotCalled(ss.T(), "Unicast", mock.Anything, mock.Anything)

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
				require.NoError(ss.T(), ss.e.Process(channels.SyncCommittee, originID, req))
			}

			// check function call expectations at the end of the load test; otherwise, load test would take much longer
			ss.core.AssertExpectations(ss.T())
			ss.con.AssertExpectations(ss.T())

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
	defer cancel()

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

	// using a very small range (1) with a 10% base probability factor, expect to almost never get misbehavior report, about 0.003% of the time (3 in 1000 requests)
	// expected probability factor: 0.1 * ((10-9) + 1)/64 = 0.003125
	loadGroups = append(loadGroups, loadGroup{0.1, 0, 15, 9, 10})

	// using a small range (10) with a 10% base probability factor, expect to get misbehavior report about 1.7% of the time (17 in 1000 requests)
	// expected probability factor: 0.1 * ((11-1) + 1)/64 = 0.0171875
	loadGroups = append(loadGroups, loadGroup{0.1, 5, 31, 1, 11})

	// using a large range (99) with a 10% base probability factor, expect to get misbehavior report about 15% of the time (150 in 1000 requests)
	// expected probability factor: 0.1 * ((100-1) + 1)/64 = 0.15625
	loadGroups = append(loadGroups, loadGroup{0.1, 110, 200, 1, 100})

	// using a flat range (0) (from height == to height) with a 1% base probability factor, expect to almost never get a misbehavior report, about 0.16% of the time (2 in 1000 requests)
	// expected probability factor: 0.01 * ((1-1) + 1)/64 = 0.0015625
	// Note: the expected upper misbehavior count is 5 even though the expected probability is close to 0 to cover outlier cases during the load test to avoid flakiness in CI.
	// Due of the probabilistic nature of the load tests, you sometimes get edge cases (that cover outliers where out of a 1000 messages, up to 5 could be reported as spam.
	// 5/1000 = 0.005 and the calculated probability is 0.00171875 which 2.9x as small.
	loadGroups = append(loadGroups, loadGroup{0.01, 0, 5, 1, 1})

	// using a small range (10) with a 1% base probability factor, expect to almost never get misbehavior report, about 0.17% of the time (2 in 1000 requests)
	// expected probability factor: 0.01 * ((11-1) + 1)/64 = 0.00171875
	loadGroups = append(loadGroups, loadGroup{0.01, 0, 7, 1, 11})

	// using a very large range (999) with a 1% base probability factor, expect to get misbehavior report about 15% of the time (150 in 1000 requests)
	// expected probability factor: 0.01 * ((1000-1) + 1)/64 = 0.15625
	loadGroups = append(loadGroups, loadGroup{0.01, 110, 200, 1, 1000})

	// ALWAYS REPORT SPAM FOR INVALID RANGE REQUESTS OR RANGE REQUESTS THAT ARE FAR OUTSIDE OF THE TOLERANCE

	// using an inverted range (from height > to height) always results in a misbehavior report, no matter how small the range is or how small the base probability factor is
	loadGroups = append(loadGroups, loadGroup{0.001, 1000, 1000, 2, 1})

	// using a very large range (999) with a 10% base probability factor, expect to get misbehavior report 100% of the time (1000 in 1000 requests)
	// expected probability factor: 0.1 * ((1000-1) + 1)/64 = 1.5625
	loadGroups = append(loadGroups, loadGroup{0.1, 1000, 1000, 1, 1000})

	// reset misbehavior report counter for each subtest
	misbehaviorsCounter := 0

	for _, loadGroup := range loadGroups {
		for i := 0; i < load; i++ {
			ss.T().Log("load iteration", i)

			nonce, err := rand.Uint64()
			require.NoError(ss.T(), err, "should generate nonce")

			// generate origin and request message
			originID := unittest.IdentifierFixture()
			req := &messages.RangeRequest{
				Nonce:      nonce,
				FromHeight: loadGroup.fromHeight,
				ToHeight:   loadGroup.toHeight,
			}

			// count misbehavior reports over the course of a load test
			ss.con.On("ReportMisbehavior", mock.Anything).Return(mock.Anything).Maybe().Run(
				func(args mock.Arguments) {
					misbehaviorsCounter++
				},
			)
			ss.e.spamDetectionConfig.rangeRequestBaseProb = loadGroup.rangeRequestBaseProb
			require.NoError(ss.T(), ss.e.Process(channels.SyncCommittee, originID, req))
		}
		// check function call expectations at the end of the load test; otherwise, load test would take much longer
		ss.core.AssertExpectations(ss.T())
		ss.con.AssertExpectations(ss.T())

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
	defer cancel()

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

	// using a very small batch request (1 block ID) with a 10% base probability factor, expect to almost never get misbehavior report, about 0.003% of the time (3 in 1000 requests)
	// expected probability factor: 0.1 * ((10-9) + 1)/64 = 0.003125
	loadGroups = append(loadGroups, loadGroup{0.1, 0, 15, repeatedBlockIDs(1)})

	// using a small batch request (10 block IDs) with a 10% base probability factor, expect to get misbehavior report about 1.7% of the time (17 in 1000 requests)
	// expected probability factor: 0.1 * ((11-1) + 1)/64 = 0.0171875
	loadGroups = append(loadGroups, loadGroup{0.1, 5, 31, repeatedBlockIDs(10)})

	// using a large batch request (99 block IDs) with a 10% base probability factor, expect to get misbehavior report about 15% of the time (150 in 1000 requests)
	// expected probability factor: 0.1 * ((100-1) + 1)/64 = 0.15625
	loadGroups = append(loadGroups, loadGroup{0.1, 110, 200, repeatedBlockIDs(99)})

	// using a small batch request (10 block IDs) with a 1% base probability factor, expect to almost never get misbehavior report, about 0.17% of the time (2 in 1000 requests)
	// expected probability factor: 0.01 * ((11-1) + 1)/64 = 0.00171875
	loadGroups = append(loadGroups, loadGroup{0.01, 0, 7, repeatedBlockIDs(10)})

	// using a very large batch request (999 block IDs) with a 1% base probability factor, expect to get misbehavior report about 15% of the time (150 in 1000 requests)
	// expected probability factor: 0.01 * ((1000-1) + 1)/64 = 0.15625
	loadGroups = append(loadGroups, loadGroup{0.01, 110, 200, repeatedBlockIDs(999)})

	// ALWAYS REPORT SPAM FOR INVALID BATCH REQUESTS OR BATCH REQUESTS THAT ARE FAR OUTSIDE OF THE TOLERANCE

	// using an empty batch request (0 block IDs) always results in a misbehavior report, no matter how small the base probability factor is
	loadGroups = append(loadGroups, loadGroup{0.001, 1000, 1000, []flow.Identifier{}})

	// using a very large batch request (999 block IDs) with a 10% base probability factor, expect to get misbehavior report 100% of the time (1000 in 1000 requests)
	// expected probability factor: 0.1 * ((999 + 1)/64 = 1.5625
	loadGroups = append(loadGroups, loadGroup{0.1, 1000, 1000, repeatedBlockIDs(999)})

	// reset misbehavior report counter for each subtest
	misbehaviorsCounter := 0

	for _, loadGroup := range loadGroups {
		for i := 0; i < load; i++ {
			ss.T().Log("load iteration", i)

			nonce, err := rand.Uint64()
			require.NoError(ss.T(), err, "should generate nonce")

			// generate origin and request message
			originID := unittest.IdentifierFixture()
			req := &messages.BatchRequest{
				Nonce:    nonce,
				BlockIDs: loadGroup.blockIDs,
			}

			// count misbehavior reports over the course of a load test
			ss.con.On("ReportMisbehavior", mock.Anything).Return(mock.Anything).Maybe().Run(
				func(args mock.Arguments) {
					misbehaviorsCounter++
				},
			)
			ss.e.spamDetectionConfig.batchRequestBaseProb = loadGroup.batchRequestBaseProb
			require.NoError(ss.T(), ss.e.Process(channels.SyncCommittee, originID, req))
		}
		// check function call expectations at the end of the load test; otherwise, load test would take much longer
		ss.core.AssertExpectations(ss.T())
		ss.con.AssertExpectations(ss.T())

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
	for i := 0; i < n; i++ {
		arr[i] = blockID
	}
	return arr
}
