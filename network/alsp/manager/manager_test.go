package alspmgr_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/alsp"
	"github.com/onflow/flow-go/network/alsp/internal"
	alspmgr "github.com/onflow/flow-go/network/alsp/manager"
	mockalsp "github.com/onflow/flow-go/network/alsp/mock"
	"github.com/onflow/flow-go/network/alsp/model"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/testutils"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestHandleReportedMisbehavior tests the handling of reported misbehavior by the network.
//
// The test sets up a mock MisbehaviorReportManager and a conduitFactory with this manager.
// It generates a single node network with the conduitFactory and starts it.
// It then uses a mock engine to register a channel with the network.
// It prepares a set of misbehavior reports and reports them to the conduit on the test channel.
// The test ensures that the MisbehaviorReportManager receives and handles all reported misbehavior
// without any duplicate reports and within a specified time.
func TestNetworkPassesReportedMisbehavior(t *testing.T) {
	misbehaviorReportManger := mocknetwork.NewMisbehaviorReportManager(t)
	misbehaviorReportManger.On("Start", mock.Anything).Return().Once()

	readyDoneChan := func() <-chan struct{} {
		ch := make(chan struct{})
		close(ch)
		return ch
	}()

	misbehaviorReportManger.On("Ready").Return(readyDoneChan).Once()
	misbehaviorReportManger.On("Done").Return(readyDoneChan).Once()

	ids, nodes, mws, _, _ := testutils.GenerateIDsAndMiddlewares(
		t,
		1,
		unittest.Logger(),
		unittest.NetworkCodec(),
		unittest.NetworkSlashingViolationsConsumer(unittest.Logger(), metrics.NewNoopCollector()))
	sms := testutils.GenerateSubscriptionManagers(t, mws)

	networkCfg := testutils.NetworkConfigFixture(t, unittest.Logger(), *ids[0], ids, mws[0], sms[0])
	net, err := p2p.NewNetwork(networkCfg, p2p.WithAlspManager(misbehaviorReportManger))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	testutils.StartNodesAndNetworks(signalerCtx, t, nodes, []network.Network{net}, 100*time.Millisecond)
	defer testutils.StopComponents[p2p.LibP2PNode](t, nodes, 100*time.Millisecond)
	defer cancel()

	e := mocknetwork.NewEngine(t)
	con, err := net.Register(channels.TestNetworkChannel, e)
	require.NoError(t, err)

	reports := testutils.MisbehaviorReportsFixture(t, 10)
	allReportsManaged := sync.WaitGroup{}
	allReportsManaged.Add(len(reports))
	var seenReports []network.MisbehaviorReport
	misbehaviorReportManger.On("HandleMisbehaviorReport", channels.TestNetworkChannel, mock.Anything).Run(func(args mock.Arguments) {
		report := args.Get(1).(network.MisbehaviorReport)
		require.Contains(t, reports, report)                                         // ensures that the report is one of the reports we expect.
		require.NotContainsf(t, seenReports, report, "duplicate report: %v", report) // ensures that we have not seen this report before.
		seenReports = append(seenReports, report)                                    // adds the report to the list of seen reports.
		allReportsManaged.Done()
	}).Return(nil)

	for _, report := range reports {
		con.ReportMisbehavior(report) // reports the misbehavior
	}

	unittest.RequireReturnsBefore(t, allReportsManaged.Wait, 100*time.Millisecond, "did not receive all reports")
}

// TestHandleReportedMisbehavior tests the handling of reported misbehavior by the network.
//
// The test sets up a mock MisbehaviorReportManager and a conduitFactory with this manager.
// It generates a single node network with the conduitFactory and starts it.
// It then uses a mock engine to register a channel with the network.
// It prepares a set of misbehavior reports and reports them to the conduit on the test channel.
// The test ensures that the MisbehaviorReportManager receives and handles all reported misbehavior
// without any duplicate reports and within a specified time.
func TestHandleReportedMisbehavior_Integration(t *testing.T) {
	var cache alsp.SpamRecordCache
	cfg := &alspmgr.MisbehaviorReportManagerConfig{
		Logger:                  unittest.Logger(),
		SpamRecordCacheSize:     uint32(100),
		SpamReportQueueSize:     uint32(100),
		AlspMetrics:             metrics.NewNoopCollector(),
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		Opts: []alspmgr.MisbehaviorReportManagerOption{alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
			cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
			return cache
		}),
		},
	}

	ids, nodes, mws, _, _ := testutils.GenerateIDsAndMiddlewares(
		t,
		1,
		unittest.Logger(),
		unittest.NetworkCodec(),
		unittest.NetworkSlashingViolationsConsumer(unittest.Logger(), metrics.NewNoopCollector()))
	sms := testutils.GenerateSubscriptionManagers(t, mws)
	networks := testutils.NetworkFixtures(
		t,
		unittest.Logger(),
		ids,
		mws,
		sms, p2p.WithAlspConfig(cfg))

	ctx, cancel := context.WithCancel(context.Background())

	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	testutils.StartNodesAndNetworks(signalerCtx, t, nodes, networks, 100*time.Millisecond)
	defer testutils.StopComponents[p2p.LibP2PNode](t, nodes, 100*time.Millisecond)
	defer cancel()

	e := mocknetwork.NewEngine(t)
	con, err := networks[0].Register(channels.TestNetworkChannel, e)
	require.NoError(t, err)

	// create a map of origin IDs to their respective misbehavior reports (10 peers, 5 reports each)
	numPeers := 10
	numReportsPerPeer := 5
	peersReports := make(map[flow.Identifier][]network.MisbehaviorReport)

	for i := 0; i < numPeers; i++ {
		originID := unittest.IdentifierFixture()
		reports := createRandomMisbehaviorReportsForOriginId(t, originID, numReportsPerPeer)
		peersReports[originID] = reports
	}

	wg := sync.WaitGroup{}
	for _, reports := range peersReports {
		wg.Add(len(reports))
		// reports the misbehavior
		for _, report := range reports {
			report := report // capture range variable
			go func() {
				defer wg.Done()

				con.ReportMisbehavior(report)
			}()
		}
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "not all misbehavior reports have been processed")

	// check if the misbehavior reports have been processed by verifying that the Adjust method was called on the cache
	require.Eventually(t, func() bool {
		for originID, reports := range peersReports {
			totalPenalty := float64(0)
			for _, report := range reports {
				totalPenalty += report.Penalty()
			}

			record, ok := cache.Get(originID)
			if !ok {
				return false
			}
			require.NotNil(t, record)

			require.Equal(t, totalPenalty, record.Penalty)
			// with just reporting a single misbehavior report, the cutoff counter should not be incremented.
			require.Equal(t, uint64(0), record.CutoffCounter)
			// the decay should be the default decay value.
			require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)
		}

		return true
	}, 1*time.Second, 10*time.Millisecond, "ALSP manager did not handle the misbehavior report")
}

// TestMisbehaviorReportMetrics tests the recording of misbehavior report metrics.
// It checks that when a misbehavior report is received by the ALSP manager, the metrics are recorded.
// It fails the test if the metrics are not recorded or if they are recorded incorrectly.
func TestMisbehaviorReportMetrics(t *testing.T) {
	alspMetrics := mockmodule.NewAlspMetrics(t)
	cfg := &alspmgr.MisbehaviorReportManagerConfig{
		SpamRecordCacheSize:     uint32(100),
		SpamReportQueueSize:     uint32(100),
		Logger:                  unittest.Logger(),
		AlspMetrics:             alspMetrics,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
	}

	ids, nodes, mws, _, _ := testutils.GenerateIDsAndMiddlewares(
		t,
		1,
		unittest.Logger(),
		unittest.NetworkCodec(),
		unittest.NetworkSlashingViolationsConsumer(unittest.Logger(), metrics.NewNoopCollector()))
	sms := testutils.GenerateSubscriptionManagers(t, mws)
	networks := testutils.NetworkFixtures(
		t,
		unittest.Logger(),
		ids,
		mws,
		sms,
		p2p.WithAlspConfig(cfg))

	ctx, cancel := context.WithCancel(context.Background())

	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	testutils.StartNodesAndNetworks(signalerCtx, t, nodes, networks, 100*time.Millisecond)
	defer testutils.StopComponents[p2p.LibP2PNode](t, nodes, 100*time.Millisecond)
	defer cancel()

	e := mocknetwork.NewEngine(t)
	con, err := networks[0].Register(channels.TestNetworkChannel, e)
	require.NoError(t, err)

	report := testutils.MisbehaviorReportFixture(t)

	// this channel is used to signal that the metrics have been recorded by the ALSP manager correctly.
	reported := make(chan struct{})

	// ensures that the metrics are recorded when a misbehavior report is received.
	alspMetrics.On("OnMisbehaviorReported", channels.TestNetworkChannel.String(), report.Reason().String()).Run(func(args mock.Arguments) {
		close(reported)
	}).Once()

	con.ReportMisbehavior(report) // reports the misbehavior

	unittest.RequireCloseBefore(t, reported, 100*time.Millisecond, "metrics for the misbehavior report were not recorded")
}

// The TestReportCreation tests the creation of misbehavior reports using the alsp.NewMisbehaviorReport function.
// The function tests the creation of both valid and invalid misbehavior reports by setting different penalty amplification values.
func TestReportCreation(t *testing.T) {

	// creates a valid misbehavior report (i.e., amplification between 1 and 100)
	report, err := alsp.NewMisbehaviorReport(
		unittest.IdentifierFixture(),
		testutils.MisbehaviorTypeFixture(t),
		alsp.WithPenaltyAmplification(10))
	require.NoError(t, err)
	require.NotNil(t, report)

	// creates a valid misbehavior report with default amplification.
	report, err = alsp.NewMisbehaviorReport(
		unittest.IdentifierFixture(),
		testutils.MisbehaviorTypeFixture(t))
	require.NoError(t, err)
	require.NotNil(t, report)

	// creates an in valid misbehavior report (i.e., amplification greater than 100 and less than 1)
	report, err = alsp.NewMisbehaviorReport(
		unittest.IdentifierFixture(),
		testutils.MisbehaviorTypeFixture(t),
		alsp.WithPenaltyAmplification(100*rand.Float64()-101))
	require.Error(t, err)
	require.Nil(t, report)

	report, err = alsp.NewMisbehaviorReport(
		unittest.IdentifierFixture(),
		testutils.MisbehaviorTypeFixture(t),
		alsp.WithPenaltyAmplification(100*rand.Float64()+101))
	require.Error(t, err)
	require.Nil(t, report)

	// 0 is not a valid amplification
	report, err = alsp.NewMisbehaviorReport(
		unittest.IdentifierFixture(),
		testutils.MisbehaviorTypeFixture(t),
		alsp.WithPenaltyAmplification(0))
	require.Error(t, err)
	require.Nil(t, report)
}

// TestNewMisbehaviorReportManager tests the creation of a new ALSP manager.
// It is a minimum viable test that ensures that a non-nil ALSP manager is created with expected set of inputs.
// In other words, variation of input values do not cause a nil ALSP manager to be created or a panic.
func TestNewMisbehaviorReportManager(t *testing.T) {
	logger := unittest.Logger()
	alspMetrics := metrics.NewNoopCollector()

	t.Run("with default values", func(t *testing.T) {
		cfg := &alspmgr.MisbehaviorReportManagerConfig{
			Logger:                  logger,
			SpamRecordCacheSize:     uint32(100),
			SpamReportQueueSize:     uint32(100),
			AlspMetrics:             alspMetrics,
			HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		}

		m, err := alspmgr.NewMisbehaviorReportManager(cfg)
		require.NoError(t, err)
		assert.NotNil(t, m)
	})

	t.Run("with a custom spam record cache", func(t *testing.T) {
		cfg := &alspmgr.MisbehaviorReportManagerConfig{
			Logger:                  logger,
			SpamRecordCacheSize:     uint32(100),
			SpamReportQueueSize:     uint32(100),
			AlspMetrics:             alspMetrics,
			HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
			Opts: []alspmgr.MisbehaviorReportManagerOption{alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
				return internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
			})},
		}
		m, err := alspmgr.NewMisbehaviorReportManager(cfg)
		require.NoError(t, err)
		assert.NotNil(t, m)
	})

	t.Run("with ALSP module enabled", func(t *testing.T) {
		cfg := &alspmgr.MisbehaviorReportManagerConfig{
			Logger:                  logger,
			SpamRecordCacheSize:     uint32(100),
			SpamReportQueueSize:     uint32(100),
			AlspMetrics:             alspMetrics,
			HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		}

		m, err := alspmgr.NewMisbehaviorReportManager(cfg)
		require.NoError(t, err)
		assert.NotNil(t, m)
	})

	t.Run("with ALSP module disabled", func(t *testing.T) {
		cfg := &alspmgr.MisbehaviorReportManagerConfig{
			Logger:                  logger,
			SpamRecordCacheSize:     uint32(100),
			SpamReportQueueSize:     uint32(100),
			AlspMetrics:             alspMetrics,
			HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		}

		m, err := alspmgr.NewMisbehaviorReportManager(cfg)
		require.NoError(t, err)
		assert.NotNil(t, m)
	})
}

// TestMisbehaviorReportManager_InitializationError tests the creation of a new ALSP manager with invalid inputs.
// It is a minimum viable test that ensures that a nil ALSP manager is created with invalid set of inputs.
func TestMisbehaviorReportManager_InitializationError(t *testing.T) {
	logger := unittest.Logger()
	alspMetrics := metrics.NewNoopCollector()

	t.Run("missing spam report queue size", func(t *testing.T) {
		cfg := &alspmgr.MisbehaviorReportManagerConfig{
			Logger:                  logger,
			SpamRecordCacheSize:     uint32(100),
			AlspMetrics:             alspMetrics,
			HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		}

		m, err := alspmgr.NewMisbehaviorReportManager(cfg)
		require.Error(t, err)
		require.ErrorIs(t, err, alspmgr.ErrSpamReportQueueSizeNotSet)
		assert.Nil(t, m)
	})

	t.Run("missing spam record cache size", func(t *testing.T) {
		cfg := &alspmgr.MisbehaviorReportManagerConfig{
			Logger:                  logger,
			SpamReportQueueSize:     uint32(100),
			AlspMetrics:             alspMetrics,
			HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		}

		m, err := alspmgr.NewMisbehaviorReportManager(cfg)
		require.Error(t, err)
		require.ErrorIs(t, err, alspmgr.ErrSpamRecordCacheSizeNotSet)
		assert.Nil(t, m)
	})
}

// TestHandleMisbehaviorReport_SinglePenaltyReport tests the handling of a single misbehavior report.
// The test ensures that the misbehavior report is handled correctly and the penalty is applied to the peer in the cache.
func TestHandleMisbehaviorReport_SinglePenaltyReport(t *testing.T) {
	logger := unittest.Logger()
	alspMetrics := metrics.NewNoopCollector()
	// create a new MisbehaviorReportManager
	var cache alsp.SpamRecordCache
	cfg := &alspmgr.MisbehaviorReportManagerConfig{
		Logger:                  logger,
		SpamRecordCacheSize:     uint32(100),
		SpamReportQueueSize:     uint32(100),
		AlspMetrics:             alspMetrics,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		Opts: []alspmgr.MisbehaviorReportManagerOption{
			alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
				cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
				return cache
			}),
		},
	}

	m, err := alspmgr.NewMisbehaviorReportManager(cfg)
	require.NoError(t, err)

	// start the ALSP manager
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		unittest.RequireCloseBefore(t, m.Done(), 100*time.Millisecond, "ALSP manager did not stop")
	}()
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	m.Start(signalerCtx)
	unittest.RequireCloseBefore(t, m.Ready(), 100*time.Millisecond, "ALSP manager did not start")

	// create a mock misbehavior report with a negative penalty value
	penalty := float64(-5)
	report := mocknetwork.NewMisbehaviorReport(t)
	report.On("OriginId").Return(unittest.IdentifierFixture())
	report.On("Reason").Return(alsp.InvalidMessage)
	report.On("Penalty").Return(penalty)

	channel := channels.Channel("test-channel")

	// handle the misbehavior report
	m.HandleMisbehaviorReport(channel, report)

	require.Eventually(t, func() bool {
		// check if the misbehavior report has been processed by verifying that the Adjust method was called on the cache
		record, ok := cache.Get(report.OriginId())
		if !ok {
			return false
		}
		require.NotNil(t, record)
		require.Equal(t, penalty, record.Penalty)
		require.Equal(t, uint64(0), record.CutoffCounter)                                             // with just reporting a misbehavior, the cutoff counter should not be incremented.
		require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay) // the decay should be the default decay value.

		return true
	}, 1*time.Second, 10*time.Millisecond, "ALSP manager did not handle the misbehavior report")
}

// TestHandleMisbehaviorReport_SinglePenaltyReport_PenaltyDisable tests the handling of a single misbehavior report when the penalty is disabled.
// The test ensures that the misbehavior is reported on metrics but the penalty is not applied to the peer in the cache.
func TestHandleMisbehaviorReport_SinglePenaltyReport_PenaltyDisable(t *testing.T) {
	alspMetrics := mockmodule.NewAlspMetrics(t)
	// create a new MisbehaviorReportManager
	// we use a mock cache but we do not expect any calls to the cache, since the penalty is disabled.
	var cache *mockalsp.SpamRecordCache
	cfg := &alspmgr.MisbehaviorReportManagerConfig{
		Logger:                  unittest.Logger(),
		SpamRecordCacheSize:     uint32(100),
		SpamReportQueueSize:     uint32(100),
		AlspMetrics:             alspMetrics,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		DisablePenalty:          true, // disable penalty for misbehavior reports
		Opts: []alspmgr.MisbehaviorReportManagerOption{
			alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
				cache = mockalsp.NewSpamRecordCache(t)
				return cache
			}),
		},
	}

	m, err := alspmgr.NewMisbehaviorReportManager(cfg)
	require.NoError(t, err)

	// start the ALSP manager
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		unittest.RequireCloseBefore(t, m.Done(), 100*time.Millisecond, "ALSP manager did not stop")
	}()
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	m.Start(signalerCtx)
	unittest.RequireCloseBefore(t, m.Ready(), 100*time.Millisecond, "ALSP manager did not start")

	// create a mock misbehavior report with a negative penalty value
	penalty := float64(-5)
	report := mocknetwork.NewMisbehaviorReport(t)
	report.On("OriginId").Return(unittest.IdentifierFixture())
	report.On("Reason").Return(alsp.InvalidMessage)
	report.On("Penalty").Return(penalty)

	channel := channels.Channel("test-channel")

	// this channel is used to signal that the metrics have been recorded by the ALSP manager correctly.
	// even in case of a disabled penalty, the metrics should be recorded.
	reported := make(chan struct{})

	// ensures that the metrics are recorded when a misbehavior report is received.
	alspMetrics.On("OnMisbehaviorReported", channel.String(), report.Reason().String()).Run(func(args mock.Arguments) {
		close(reported)
	}).Once()

	// handle the misbehavior report
	m.HandleMisbehaviorReport(channel, report)

	unittest.RequireCloseBefore(t, reported, 100*time.Millisecond, "metrics for the misbehavior report were not recorded")

	// since the penalty is disabled, we do not expect any calls to the cache.
	cache.AssertNotCalled(t, "Adjust", mock.Anything, mock.Anything)
}

// // TestHandleMisbehaviorReport_MultiplePenaltyReportsForSinglePeer_Sequentially tests the handling of multiple misbehavior reports for a single peer.
// // Reports are coming in sequentially.
// // The test ensures that each misbehavior report is handled correctly and the penalties are cumulatively applied to the peer in the cache.
//
//	func TestHandleMisbehaviorReport_MultiplePenaltyReportsForSinglePeer_Sequentially(t *testing.T) {
//		alspMetrics := metrics.NewNoopCollector()
//
//		cfg := &alspmgr.MisbehaviorReportManagerConfig{
//			Logger:                  unittest.Logger(),
//			SpamRecordCacheSize:     uint32(100),
//			SpamReportQueueSize:     uint32(100),
//			AlspMetrics:             alspMetrics,
//			HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
//		}
//
//		// create a new MisbehaviorReportManager
//		var cache alsp.SpamRecordCache
//		m, err := alspmgr.NewMisbehaviorReportManager(cfg,
//			alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
//				cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
//				return cache
//			}))
//		require.NoError(t, err)
//
//		// start the ALSP manager
//		ctx, cancel := context.WithCancel(context.Background())
//		defer func() {
//			cancel()
//			unittest.RequireCloseBefore(t, m.Done(), 100*time.Millisecond, "ALSP manager did not stop")
//		}()
//		signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
//		m.Start(signalerCtx)
//		unittest.RequireCloseBefore(t, m.Ready(), 100*time.Millisecond, "ALSP manager did not start")
//
//		// creates a list of mock misbehavior reports with negative penalty values for a single peer
//		originId := unittest.IdentifierFixture()
//		reports := createRandomMisbehaviorReportsForOriginId(t, originId, 5)
//
//		channel := channels.Channel("test-channel")
//
//		// handle the misbehavior reports
//		totalPenalty := float64(0)
//		for _, report := range reports {
//			totalPenalty += report.Penalty()
//			m.HandleMisbehaviorReport(channel, report)
//		}
//
//		require.Eventually(t, func() bool {
//			// check if the misbehavior report has been processed by verifying that the Adjust method was called on the cache
//			record, ok := cache.Get(originId)
//			if !ok {
//				return false
//			}
//			require.NotNil(t, record)
//
//			if totalPenalty != record.Penalty {
//				// all the misbehavior reports should be processed by now, so the penalty should be equal to the total penalty
//				return false
//			}
//			// with just reporting a few misbehavior reports, the cutoff counter should not be incremented.
//			require.Equal(t, uint64(0), record.CutoffCounter)
//			// the decay should be the default decay value.
//			require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)
//
//			return true
//		}, 1*time.Second, 10*time.Millisecond, "ALSP manager did not handle the misbehavior report")
//	}
//
// // TestHandleMisbehaviorReport_MultiplePenaltyReportsForSinglePeer_Sequential tests the handling of multiple misbehavior reports for a single peer.
// // Reports are coming in concurrently.
// // The test ensures that each misbehavior report is handled correctly and the penalties are cumulatively applied to the peer in the cache.
//
//	func TestHandleMisbehaviorReport_MultiplePenaltyReportsForSinglePeer_Concurrently(t *testing.T) {
//		alspMetrics := metrics.NewNoopCollector()
//
//		cfg := &alspmgr.MisbehaviorReportManagerConfig{
//			Logger:                  unittest.Logger(),
//			SpamRecordCacheSize:     uint32(100),
//			SpamReportQueueSize:     uint32(100),
//			AlspMetrics:             alspMetrics,
//			HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
//		}
//
//		// create a new MisbehaviorReportManager
//		var cache alsp.SpamRecordCache
//		m, err := alspmgr.NewMisbehaviorReportManager(cfg,
//			alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
//				cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
//				return cache
//			}))
//		require.NoError(t, err)
//
//		// start the ALSP manager
//		ctx, cancel := context.WithCancel(context.Background())
//		defer func() {
//			cancel()
//			unittest.RequireCloseBefore(t, m.Done(), 100*time.Millisecond, "ALSP manager did not stop")
//		}()
//		signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
//		m.Start(signalerCtx)
//		unittest.RequireCloseBefore(t, m.Ready(), 100*time.Millisecond, "ALSP manager did not start")
//
//		// creates a list of mock misbehavior reports with negative penalty values for a single peer
//		originId := unittest.IdentifierFixture()
//		reports := createRandomMisbehaviorReportsForOriginId(t, originId, 5)
//
//		channel := channels.Channel("test-channel")
//
//		wg := sync.WaitGroup{}
//		wg.Add(len(reports))
//		// handle the misbehavior reports
//		totalPenalty := float64(0)
//		for _, report := range reports {
//			report := report // capture range variable
//			totalPenalty += report.Penalty()
//			go func() {
//				defer wg.Done()
//
//				m.HandleMisbehaviorReport(channel, report)
//			}()
//		}
//
//		unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "not all misbehavior reports have been processed")
//
//		require.Eventually(t, func() bool {
//			// check if the misbehavior report has been processed by verifying that the Adjust method was called on the cache
//			record, ok := cache.Get(originId)
//			if !ok {
//				return false
//			}
//			require.NotNil(t, record)
//
//			if totalPenalty != record.Penalty {
//				// all the misbehavior reports should be processed by now, so the penalty should be equal to the total penalty
//				return false
//			}
//			// with just reporting a few misbehavior reports, the cutoff counter should not be incremented.
//			require.Equal(t, uint64(0), record.CutoffCounter)
//			// the decay should be the default decay value.
//			require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)
//
//			return true
//		}, 1*time.Second, 10*time.Millisecond, "ALSP manager did not handle the misbehavior report")
//	}
//
// // TestHandleMisbehaviorReport_SinglePenaltyReportsForMultiplePeers_Sequentially tests the handling of single misbehavior reports for multiple peers.
// // Reports are coming in sequentially.
// // The test ensures that each misbehavior report is handled correctly and the penalties are applied to the corresponding peers in the cache.
//
//	func TestHandleMisbehaviorReport_SinglePenaltyReportsForMultiplePeers_Sequentially(t *testing.T) {
//		alspMetrics := metrics.NewNoopCollector()
//
//		cfg := &alspmgr.MisbehaviorReportManagerConfig{
//			Logger:                  unittest.Logger(),
//			SpamRecordCacheSize:     uint32(100),
//			SpamReportQueueSize:     uint32(100),
//			AlspMetrics:             alspMetrics,
//			HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
//		}
//
//		// create a new MisbehaviorReportManager
//		var cache alsp.SpamRecordCache
//		m, err := alspmgr.NewMisbehaviorReportManager(cfg,
//			alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
//				cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
//				return cache
//			}))
//		require.NoError(t, err)
//
//		// start the ALSP manager
//		ctx, cancel := context.WithCancel(context.Background())
//		defer func() {
//			cancel()
//			unittest.RequireCloseBefore(t, m.Done(), 100*time.Millisecond, "ALSP manager did not stop")
//		}()
//		signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
//		m.Start(signalerCtx)
//		unittest.RequireCloseBefore(t, m.Ready(), 100*time.Millisecond, "ALSP manager did not start")
//
//		// creates a list of single misbehavior reports for multiple peers (10 peers)
//		numPeers := 10
//		reports := createRandomMisbehaviorReports(t, numPeers)
//
//		channel := channels.Channel("test-channel")
//
//		// handle the misbehavior reports
//		for _, report := range reports {
//			m.HandleMisbehaviorReport(channel, report)
//		}
//
//		// check if the misbehavior reports have been processed by verifying that the Adjust method was called on the cache
//		require.Eventually(t, func() bool {
//			for _, report := range reports {
//				originID := report.OriginId()
//				record, ok := cache.Get(originID)
//				if !ok {
//					return false
//				}
//				require.NotNil(t, record)
//
//				require.Equal(t, report.Penalty(), record.Penalty)
//				// with just reporting a single misbehavior report, the cutoff counter should not be incremented.
//				require.Equal(t, uint64(0), record.CutoffCounter)
//				// the decay should be the default decay value.
//				require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)
//			}
//
//			return true
//		}, 1*time.Second, 10*time.Millisecond, "ALSP manager did not handle the misbehavior report")
//
// }
//
// TestHandleMisbehaviorReport_SinglePenaltyReportsForMultiplePeers_Concurrently tests the handling of single misbehavior reports for multiple peers.
// Reports are coming in concurrently.
// The test ensures that each misbehavior report is handled correctly and the penalties are applied to the corresponding peers in the cache.
func TestHandleMisbehaviorReport_SinglePenaltyReportsForMultiplePeers_Concurrently(t *testing.T) {
	alspMetrics := metrics.NewNoopCollector()
	// create a new MisbehaviorReportManager
	var cache alsp.SpamRecordCache
	cfg := &alspmgr.MisbehaviorReportManagerConfig{
		Logger:                  unittest.Logger(),
		SpamRecordCacheSize:     uint32(100),
		SpamReportQueueSize:     uint32(100),
		AlspMetrics:             alspMetrics,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		Opts: []alspmgr.MisbehaviorReportManagerOption{
			alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
				cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
				return cache
			}),
		},
	}

	m, err := alspmgr.NewMisbehaviorReportManager(cfg)
	require.NoError(t, err)

	// start the ALSP manager
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		unittest.RequireCloseBefore(t, m.Done(), 100*time.Millisecond, "ALSP manager did not stop")
	}()
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	m.Start(signalerCtx)
	unittest.RequireCloseBefore(t, m.Ready(), 100*time.Millisecond, "ALSP manager did not start")

	// creates a list of single misbehavior reports for multiple peers (10 peers)
	numPeers := 10
	reports := createRandomMisbehaviorReports(t, numPeers)

	channel := channels.Channel("test-channel")

	wg := sync.WaitGroup{}
	wg.Add(len(reports))
	// handle the misbehavior reports
	totalPenalty := float64(0)
	for _, report := range reports {
		report := report // capture range variable
		totalPenalty += report.Penalty()
		go func() {
			defer wg.Done()

			m.HandleMisbehaviorReport(channel, report)
		}()
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "not all misbehavior reports have been processed")

	// check if the misbehavior reports have been processed by verifying that the Adjust method was called on the cache
	require.Eventually(t, func() bool {
		for _, report := range reports {
			originID := report.OriginId()
			record, ok := cache.Get(originID)
			if !ok {
				return false
			}
			require.NotNil(t, record)

			require.Equal(t, report.Penalty(), record.Penalty)
			// with just reporting a single misbehavior report, the cutoff counter should not be incremented.
			require.Equal(t, uint64(0), record.CutoffCounter)
			// the decay should be the default decay value.
			require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)
		}

		return true
	}, 1*time.Second, 10*time.Millisecond, "ALSP manager did not handle the misbehavior report")
}

// TestHandleMisbehaviorReport_MultiplePenaltyReportsForMultiplePeers_Sequentially tests the handling of multiple misbehavior reports for multiple peers.
// Reports are coming in sequentially.
// The test ensures that each misbehavior report is handled correctly and the penalties are cumulatively applied to the corresponding peers in the cache.
func TestHandleMisbehaviorReport_MultiplePenaltyReportsForMultiplePeers_Sequentially(t *testing.T) {
	alspMetrics := metrics.NewNoopCollector()
	// create a new MisbehaviorReportManager
	var cache alsp.SpamRecordCache
	cfg := &alspmgr.MisbehaviorReportManagerConfig{
		Logger:                  unittest.Logger(),
		SpamRecordCacheSize:     uint32(100),
		SpamReportQueueSize:     uint32(100),
		AlspMetrics:             alspMetrics,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		Opts: []alspmgr.MisbehaviorReportManagerOption{
			alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
				cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
				return cache
			}),
		},
	}

	m, err := alspmgr.NewMisbehaviorReportManager(cfg)
	require.NoError(t, err)

	// start the ALSP manager
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		unittest.RequireCloseBefore(t, m.Done(), 100*time.Millisecond, "ALSP manager did not stop")
	}()
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	m.Start(signalerCtx)
	unittest.RequireCloseBefore(t, m.Ready(), 100*time.Millisecond, "ALSP manager did not start")

	// create a map of origin IDs to their respective misbehavior reports (10 peers, 5 reports each)
	numPeers := 10
	numReportsPerPeer := 5
	peersReports := make(map[flow.Identifier][]network.MisbehaviorReport)

	for i := 0; i < numPeers; i++ {
		originID := unittest.IdentifierFixture()
		reports := createRandomMisbehaviorReportsForOriginId(t, originID, numReportsPerPeer)
		peersReports[originID] = reports
	}

	channel := channels.Channel("test-channel")

	wg := sync.WaitGroup{}
	// handle the misbehavior reports
	for _, reports := range peersReports {
		wg.Add(len(reports))
		for _, report := range reports {
			report := report // capture range variable
			go func() {
				defer wg.Done()

				m.HandleMisbehaviorReport(channel, report)
			}()
		}
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "not all misbehavior reports have been processed")

	// check if the misbehavior reports have been processed by verifying that the Adjust method was called on the cache
	require.Eventually(t, func() bool {
		for originID, reports := range peersReports {
			totalPenalty := float64(0)
			for _, report := range reports {
				totalPenalty += report.Penalty()
			}

			record, ok := cache.Get(originID)
			if !ok {
				fmt.Println("not ok")
				return false
			}
			require.NotNil(t, record)

			require.Equal(t, totalPenalty, record.Penalty)
			// with just reporting a single misbehavior report, the cutoff counter should not be incremented.
			require.Equal(t, uint64(0), record.CutoffCounter)
			// the decay should be the default decay value.
			require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)
		}

		return true
	}, 2*time.Second, 10*time.Millisecond, "ALSP manager did not handle the misbehavior report")
}

// TestHandleMisbehaviorReport_MultiplePenaltyReportsForMultiplePeers_Sequentially tests the handling of multiple misbehavior reports for multiple peers.
// Reports are coming in concurrently.
// The test ensures that each misbehavior report is handled correctly and the penalties are cumulatively applied to the corresponding peers in the cache.
func TestHandleMisbehaviorReport_MultiplePenaltyReportsForMultiplePeers_Concurrently(t *testing.T) {
	alspMetrics := metrics.NewNoopCollector()
	// create a new MisbehaviorReportManager
	var cache alsp.SpamRecordCache

	cfg := &alspmgr.MisbehaviorReportManagerConfig{
		Logger:                  unittest.Logger(),
		SpamRecordCacheSize:     uint32(100),
		SpamReportQueueSize:     uint32(100),
		AlspMetrics:             alspMetrics,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		Opts: []alspmgr.MisbehaviorReportManagerOption{
			alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
				cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
				return cache
			}),
		},
	}

	m, err := alspmgr.NewMisbehaviorReportManager(cfg)
	require.NoError(t, err)

	// start the ALSP manager
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		unittest.RequireCloseBefore(t, m.Done(), 100*time.Millisecond, "ALSP manager did not stop")
	}()
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	m.Start(signalerCtx)
	unittest.RequireCloseBefore(t, m.Ready(), 100*time.Millisecond, "ALSP manager did not start")

	// create a map of origin IDs to their respective misbehavior reports (10 peers, 5 reports each)
	numPeers := 10
	numReportsPerPeer := 5
	peersReports := make(map[flow.Identifier][]network.MisbehaviorReport)

	for i := 0; i < numPeers; i++ {
		originID := unittest.IdentifierFixture()
		reports := createRandomMisbehaviorReportsForOriginId(t, originID, numReportsPerPeer)
		peersReports[originID] = reports
	}

	channel := channels.Channel("test-channel")

	// handle the misbehavior reports
	for _, reports := range peersReports {
		for _, report := range reports {
			m.HandleMisbehaviorReport(channel, report)
		}
	}

	// check if the misbehavior reports have been processed by verifying that the Adjust method was called on the cache
	require.Eventually(t, func() bool {
		for originID, reports := range peersReports {
			totalPenalty := float64(0)
			for _, report := range reports {
				totalPenalty += report.Penalty()
			}

			record, ok := cache.Get(originID)
			if !ok {
				return false
			}
			require.NotNil(t, record)

			require.Equal(t, totalPenalty, record.Penalty)
			// with just reporting a single misbehavior report, the cutoff counter should not be incremented.
			require.Equal(t, uint64(0), record.CutoffCounter)
			// the decay should be the default decay value.
			require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)
		}

		return true
	}, 1*time.Second, 10*time.Millisecond, "ALSP manager did not handle the misbehavior report")
}

// TestHandleMisbehaviorReport_DuplicateReportsForSinglePeer_Concurrently tests the handling of duplicate misbehavior reports for a single peer.
// Reports are coming in concurrently.
// The test ensures that each misbehavior report is handled correctly and the penalties are cumulatively applied to the peer in the cache, in
// other words, the duplicate reports are not ignored. This is important because the misbehavior reports are assumed each uniquely reporting
// a different misbehavior even though they are coming with the same description. This is similar to the traffic tickets, where each ticket
// is uniquely identifying a traffic violation, even though the description of the violation is the same.
func TestHandleMisbehaviorReport_DuplicateReportsForSinglePeer_Concurrently(t *testing.T) {
	var cache alsp.SpamRecordCache
	cfg := &alspmgr.MisbehaviorReportManagerConfig{
		Logger:                  unittest.Logger(),
		SpamRecordCacheSize:     uint32(100),
		SpamReportQueueSize:     uint32(100),
		AlspMetrics:             metrics.NewNoopCollector(),
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		Opts: []alspmgr.MisbehaviorReportManagerOption{
			alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
				cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
				return cache
			}),
		},
	}

	// create a new MisbehaviorReportManager
	m, err := alspmgr.NewMisbehaviorReportManager(cfg)
	require.NoError(t, err)

	// start the ALSP manager
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		unittest.RequireCloseBefore(t, m.Done(), 100*time.Millisecond, "ALSP manager did not stop")
	}()
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	m.Start(signalerCtx)
	unittest.RequireCloseBefore(t, m.Ready(), 100*time.Millisecond, "ALSP manager did not start")

	// creates a single misbehavior report
	originId := unittest.IdentifierFixture()
	report := createMisbehaviorReportForOriginId(t, originId)

	channel := channels.Channel("test-channel")

	times := 100 // number of times the duplicate misbehavior report is reported concurrently
	wg := sync.WaitGroup{}
	wg.Add(times)

	// concurrently reports the same misbehavior report twice
	for i := 0; i < times; i++ {
		go func() {
			defer wg.Done()

			m.HandleMisbehaviorReport(channel, report)
		}()
	}
	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "not all misbehavior reports have been processed")

	require.Eventually(t, func() bool {
		// check if the misbehavior reports have been processed by verifying that the Adjust method was called on the cache
		record, ok := cache.Get(originId)
		if !ok {
			return false
		}
		require.NotNil(t, record)

		// eventually, the penalty should be the accumulated penalty of all the duplicate misbehavior reports.
		if record.Penalty != report.Penalty()*float64(times) {
			return false
		}
		// with just reporting a few misbehavior reports, the cutoff counter should not be incremented.
		require.Equal(t, uint64(0), record.CutoffCounter)
		// the decay should be the default decay value.
		require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)

		return true
	}, 1*time.Second, 10*time.Millisecond, "ALSP manager did not handle the misbehavior report")
}

// createMisbehaviorReportForOriginId creates a mock misbehavior report for a single origin id.
// Args:
// - t: the testing.T instance
// - originID: the origin id of the misbehavior report
// Returns:
// - network.MisbehaviorReport: the misbehavior report
// Note: the penalty of the misbehavior report is randomly chosen between -1 and -10.
func createMisbehaviorReportForOriginId(t *testing.T, originID flow.Identifier) network.MisbehaviorReport {
	report := mocknetwork.NewMisbehaviorReport(t)
	report.On("OriginId").Return(originID)
	report.On("Reason").Return(alsp.AllMisbehaviorTypes()[rand.Intn(len(alsp.AllMisbehaviorTypes()))])
	report.On("Penalty").Return(float64(-1 * rand.Intn(10))) // random penalty between -1 and -10

	return report
}

// createRandomMisbehaviorReportsForOriginId creates a slice of random misbehavior reports for a single origin id.
// Args:
// - t: the testing.T instance
// - originID: the origin id of the misbehavior reports
// - numReports: the number of misbehavior reports to create
// Returns:
// - []network.MisbehaviorReport: the slice of misbehavior reports
// Note: the penalty of the misbehavior reports is randomly chosen between -1 and -10.
func createRandomMisbehaviorReportsForOriginId(t *testing.T, originID flow.Identifier, numReports int) []network.MisbehaviorReport {
	reports := make([]network.MisbehaviorReport, numReports)

	for i := 0; i < numReports; i++ {
		reports[i] = createMisbehaviorReportForOriginId(t, originID)
	}

	return reports
}

// createRandomMisbehaviorReports creates a slice of random misbehavior reports.
// Args:
// - t: the testing.T instance
// - numReports: the number of misbehavior reports to create
// Returns:
// - []network.MisbehaviorReport: the slice of misbehavior reports
// Note: the penalty of the misbehavior reports is randomly chosen between -1 and -10.
func createRandomMisbehaviorReports(t *testing.T, numReports int) []network.MisbehaviorReport {
	reports := make([]network.MisbehaviorReport, numReports)

	for i := 0; i < numReports; i++ {
		reports[i] = createMisbehaviorReportForOriginId(t, unittest.IdentifierFixture())
	}

	return reports
}
