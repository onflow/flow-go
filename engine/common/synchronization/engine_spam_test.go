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
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/rand"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestLoad_Process_SyncRequest_HigherThanReceiver_OutsideTolerance_AlwaysReportSpam tests that a sync request that's higher
// than the receiver's height doesn't trigger a response, even if outside tolerance and generates ALSP
// spamming misbehavior report (simulating the unlikely probability).
// This load test ensures that a misbehavior report is generated every time when the probability factor is set to 1.0.
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

		// assert that HandleHeight, WithinTolerance are not called because misbehavior is reported
		// also, check that response is never sent
		ss.core.AssertNotCalled(ss.T(), "HandleHeight")
		ss.core.AssertNotCalled(ss.T(), "WithinTolerance")
		ss.con.AssertNotCalled(ss.T(), "Unicast", mock.Anything, mock.Anything)

		// count misbehavior reports over the course of a load test
		ss.con.On("ReportMisbehavior", mock.Anything).Return(mock.Anything).Run(
			func(args mock.Arguments) {
				misbehaviorsCounter++
			},
		)

		// force creating misbehavior report by setting syncRequestProbability to 1.0 (i.e. report misbehavior 100% of the time)
		ss.e.spamDetectionConfig.syncRequestProbability = 1.0

		require.NoError(ss.T(), ss.e.Process(channels.SyncCommittee, originID, req))
	}

	ss.core.AssertExpectations(ss.T())
	ss.con.AssertExpectations(ss.T())
	assert.Equal(ss.T(), misbehaviorsCounter, load) // should generate misbehavior report every time
}

// TestLoad_Process_SyncRequest_HigherThanReceiver_OutsideTolerance_SometimesReportSpam load tests that a sync request that's higher
// than the receiver's height doesn't trigger a response, even if outside tolerance. It checks that an ALSP
// spam misbehavior report was generated and that the number of misbehavior reports is within a reasonable range.
// This load test ensures that a misbehavior report is generated an appropriate range of times when the probability factor is set to different values.
func (ss *SyncSuite) TestLoad_Process_SyncRequest_HigherThanReceiver_OutsideTolerance_SometimesReportSpam() {
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(ss.T(), context.Background())
	ss.e.Start(ctx)
	unittest.AssertClosesBefore(ss.T(), ss.e.Ready(), time.Second)
	defer cancel()

	load := 1000

	type loadGroup struct {
		syncRequestProbabilityFactor float32
		expectedMisbehaviorsLower    int
		expectedMisbehaviorsUpper    int
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
				ss.e.spamDetectionConfig.syncRequestProbability = loadGroup.syncRequestProbabilityFactor
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

func (ss *SyncSuite) TestLoad_Process_RangeRequest_SometimesReportSpam() {
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(ss.T(), context.Background())
	ss.e.Start(ctx)
	unittest.AssertClosesBefore(ss.T(), ss.e.Ready(), time.Second)
	defer cancel()

	load := 1000

	type loadGroup struct {
		rangeRequestBaseProb      float32
		expectedMisbehaviorsLower int
		expectedMisbehaviorsUpper int
		fromHeight                uint64
		toHeight                  uint64
	}

	loadGroups := []loadGroup{}

	// using a very small range with a 10% base probability factor, expect to almost never get misbehavior report, about 0.003% of the time (3 in 1000 requests)
	// expected probability factor: 0.1 * ((10-9) + 1)/64 = 0.003125
	loadGroups = append(loadGroups, loadGroup{0.1, 0, 15, 9, 10})

	// using a large range (99) with a 10% base probability factor, expect to get misbehavior report about 15% of the time (150 in 1000 requests)
	// expected probability factor: 0.1 * ((100-1) + 1)/64 = 0.15625
	loadGroups = append(loadGroups, loadGroup{0.1, 110, 200, 1, 100})

	// using a very large range (999) with a 10% base probability factor, expect to get misbehavior report 100% of the time (1000 in 1000 requests)
	// expected probability factor: 0.1 * ((1000-1) + 1)/64 = 1.5625
	loadGroups = append(loadGroups, loadGroup{0.1, 1000, 1000, 1, 1000})

	// using a very large range (999) with a 1% base probability factor, expect to get misbehavior report about 15% of the time (150 in 1000 requests)
	// expected probability factor: 0.01 * ((1000-1) + 1)/64 = 0.15625
	loadGroups = append(loadGroups, loadGroup{0.01, 110, 200, 1, 1000})

	// INVALID RANGE REQUESTS
	// the following range requests are invalid and should always result in a misbehavior report

	// using an inverted range (from height > to height) always results in a misbehavior report, no matter how small the range is or what the base probability factor is
	loadGroups = append(loadGroups, loadGroup{0.001, 1000, 1000, 2, 1})

	// using a flat range (from height == to height) always results in a misbehavior report, no matter how small the range is or what the base probability factor is
	loadGroups = append(loadGroups, loadGroup{0.001, 1000, 1000, 1, 1})

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
