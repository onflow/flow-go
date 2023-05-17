package alspmgr_test

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
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
	"github.com/onflow/flow-go/network/p2p/conduit"
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
	conduitFactory := conduit.NewDefaultConduitFactory(
		&alspmgr.MisbehaviorReportManagerConfig{
			Logger:               unittest.Logger(),
			AlspMetrics:          metrics.NewNoopCollector(),
			CacheMetrics:         metrics.NewNoopCollector(),
			SpamRecordsCacheSize: 100,
		},
		conduit.WithMisbehaviorManager(misbehaviorReportManger))

	ids, nodes, mws, _, _ := testutils.GenerateIDsAndMiddlewares(
		t,
		1,
		unittest.Logger(),
		unittest.NetworkCodec(),
		unittest.NetworkSlashingViolationsConsumer(unittest.Logger(), metrics.NewNoopCollector()))
	sms := testutils.GenerateSubscriptionManagers(t, mws)
	networks := testutils.GenerateNetworks(
		t,
		unittest.Logger(),
		ids,
		mws,
		sms,
		p2p.WithConduitFactory(conduitFactory))

	ctx, cancel := context.WithCancel(context.Background())

	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	testutils.StartNodesAndNetworks(signalerCtx, t, nodes, networks, 100*time.Millisecond)
	defer testutils.StopComponents[p2p.LibP2PNode](t, nodes, 100*time.Millisecond)
	defer cancel()

	e := mocknetwork.NewEngine(t)
	con, err := networks[0].Register(channels.TestNetworkChannel, e)
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

// TestMisbehaviorReportMetrics tests the recording of misbehavior report metrics.
// It checks that when a misbehavior report is received by the ALSP manager, the metrics are recorded.
// It fails the test if the metrics are not recorded or if they are recorded incorrectly.
func TestMisbehaviorReportMetrics(t *testing.T) {
	alspMetrics := mockmodule.NewAlspMetrics(t)
	conduitFactory := conduit.NewDefaultConduitFactory(
		&alspmgr.MisbehaviorReportManagerConfig{
			SpamRecordsCacheSize: uint32(100),
			Logger:               unittest.Logger(),
			AlspMetrics:          alspMetrics,
			CacheMetrics:         metrics.NewNoopCollector(),
		})

	ids, nodes, mws, _, _ := testutils.GenerateIDsAndMiddlewares(
		t,
		1,
		unittest.Logger(),
		unittest.NetworkCodec(),
		unittest.NetworkSlashingViolationsConsumer(unittest.Logger(), metrics.NewNoopCollector()))
	sms := testutils.GenerateSubscriptionManagers(t, mws)
	networks := testutils.GenerateNetworks(
		t,
		unittest.Logger(),
		ids,
		mws,
		sms,
		p2p.WithConduitFactory(conduitFactory))

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
	cacheMetrics := metrics.NewNoopCollector()
	cacheSize := uint32(100)

	t.Run("with default values", func(t *testing.T) {
		cfg := &alspmgr.MisbehaviorReportManagerConfig{
			Logger:               logger,
			SpamRecordsCacheSize: cacheSize,
			AlspMetrics:          alspMetrics,
			CacheMetrics:         cacheMetrics,
		}

		m := alspmgr.NewMisbehaviorReportManager(cfg)
		assert.NotNil(t, m)

	})

	t.Run("with a custom spam record cache", func(t *testing.T) {
		customCache := internal.NewSpamRecordCache(100, logger, cacheMetrics, model.SpamRecordFactory())

		cfg := &alspmgr.MisbehaviorReportManagerConfig{
			Logger:               logger,
			SpamRecordsCacheSize: cacheSize,
			AlspMetrics:          alspMetrics,
			CacheMetrics:         cacheMetrics,
		}

		m := alspmgr.NewMisbehaviorReportManager(cfg, alspmgr.WithSpamRecordsCache(customCache))
		assert.NotNil(t, m)
	})

	t.Run("with ALSP module enabled", func(t *testing.T) {
		cfg := &alspmgr.MisbehaviorReportManagerConfig{
			Logger:               logger,
			SpamRecordsCacheSize: cacheSize,
			AlspMetrics:          alspMetrics,
			CacheMetrics:         cacheMetrics,
		}

		m := alspmgr.NewMisbehaviorReportManager(cfg)
		assert.NotNil(t, m)
	})

	t.Run("with ALSP module disabled", func(t *testing.T) {
		cfg := &alspmgr.MisbehaviorReportManagerConfig{
			Logger:               logger,
			SpamRecordsCacheSize: cacheSize,
			AlspMetrics:          alspMetrics,
			CacheMetrics:         cacheMetrics,
		}

		m := alspmgr.NewMisbehaviorReportManager(cfg)
		assert.NotNil(t, m)
	})
}

// TestHandleMisbehaviorReport_SinglePenaltyReport tests the handling of a single misbehavior report.
// The test ensures that the misbehavior report is handled correctly and the penalty is applied to the peer in the cache.
func TestHandleMisbehaviorReport_SinglePenaltyReport(t *testing.T) {
	logger := unittest.Logger()
	alspMetrics := metrics.NewNoopCollector()
	cacheMetrics := metrics.NewNoopCollector()
	cacheSize := uint32(100)

	cfg := &alspmgr.MisbehaviorReportManagerConfig{
		Logger:               logger,
		SpamRecordsCacheSize: cacheSize,
		AlspMetrics:          alspMetrics,
		CacheMetrics:         cacheMetrics,
	}

	cache := internal.NewSpamRecordCache(cfg.SpamRecordsCacheSize, cfg.Logger, cfg.CacheMetrics, model.SpamRecordFactory())

	// create a new MisbehaviorReportManager
	m := alspmgr.NewMisbehaviorReportManager(cfg, alspmgr.WithSpamRecordsCache(cache))

	// create a mock misbehavior report with a negative penalty value
	penalty := float64(-5)
	report := mocknetwork.NewMisbehaviorReport(t)
	report.On("OriginId").Return(unittest.IdentifierFixture())
	report.On("Reason").Return(alsp.InvalidMessage)
	report.On("Penalty").Return(penalty)

	channel := channels.Channel("test-channel")

	// handle the misbehavior report
	m.HandleMisbehaviorReport(channel, report)
	// check if the misbehavior report has been processed by verifying that the Adjust method was called on the cache
	record, ok := cache.Get(report.OriginId())
	require.True(t, ok)
	require.NotNil(t, record)
	require.Equal(t, penalty, record.Penalty)
	require.Equal(t, uint64(0), record.CutoffCounter)                                             // with just reporting a misbehavior, the cutoff counter should not be incremented.
	require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay) // the decay should be the default decay value.
}

// TestHandleMisbehaviorReport_SinglePenaltyReport_PenaltyDisable tests the handling of a single misbehavior report when the penalty is disabled.
// The test ensures that the misbehavior is reported on metrics but the penalty is not applied to the peer in the cache.
func TestHandleMisbehaviorReport_SinglePenaltyReport_PenaltyDisable(t *testing.T) {
	alspMetrics := mockmodule.NewAlspMetrics(t)
	cacheMetrics := metrics.NewNoopCollector()
	cacheSize := uint32(100)

	cfg := &alspmgr.MisbehaviorReportManagerConfig{
		Logger:               unittest.Logger(),
		SpamRecordsCacheSize: cacheSize,
		AlspMetrics:          alspMetrics,
		CacheMetrics:         cacheMetrics,
		DisablePenalty:       true, // disable penalty for misbehavior reports
	}

	// we use a mock cache but we do not expect any calls to the cache, since the penalty is disabled.
	cache := mockalsp.NewSpamRecordCache(t)

	// create a new MisbehaviorReportManager
	m := alspmgr.NewMisbehaviorReportManager(cfg, alspmgr.WithSpamRecordsCache(cache))

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

// TestHandleMisbehaviorReport_MultiplePenaltyReportsForSinglePeer_Sequentially tests the handling of multiple misbehavior reports for a single peer.
// Reports are coming in sequentially.
// The test ensures that each misbehavior report is handled correctly and the penalties are cumulatively applied to the peer in the cache.
func TestHandleMisbehaviorReport_MultiplePenaltyReportsForSinglePeer_Sequentially(t *testing.T) {
	alspMetrics := metrics.NewNoopCollector()
	cacheMetrics := metrics.NewNoopCollector()
	cacheSize := uint32(100)

	cfg := &alspmgr.MisbehaviorReportManagerConfig{
		Logger:               unittest.Logger(),
		SpamRecordsCacheSize: cacheSize,
		AlspMetrics:          alspMetrics,
		CacheMetrics:         cacheMetrics,
	}

	cache := internal.NewSpamRecordCache(cfg.SpamRecordsCacheSize, cfg.Logger, cfg.CacheMetrics, model.SpamRecordFactory())

	// create a new MisbehaviorReportManager
	m := alspmgr.NewMisbehaviorReportManager(cfg, alspmgr.WithSpamRecordsCache(cache))

	// creates a list of mock misbehavior reports with negative penalty values for a single peer
	originId := unittest.IdentifierFixture()
	reports := createRandomMisbehaviorReportsForOriginId(t, originId, 5)

	channel := channels.Channel("test-channel")

	// handle the misbehavior reports
	totalPenalty := float64(0)
	for _, report := range reports {
		totalPenalty += report.Penalty()
		m.HandleMisbehaviorReport(channel, report)
	}

	// check if the misbehavior reports have been processed by verifying that the Adjust method was called on the cache
	record, ok := cache.Get(originId)
	require.True(t, ok)
	require.NotNil(t, record)

	require.Equal(t, totalPenalty, record.Penalty)
	// with just reporting a few misbehavior reports, the cutoff counter should not be incremented.
	require.Equal(t, uint64(0), record.CutoffCounter)
	// the decay should be the default decay value.
	require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)
}

// TestHandleMisbehaviorReport_MultiplePenaltyReportsForSinglePeer_Sequential tests the handling of multiple misbehavior reports for a single peer.
// Reports are coming in concurrently.
// The test ensures that each misbehavior report is handled correctly and the penalties are cumulatively applied to the peer in the cache.
func TestHandleMisbehaviorReport_MultiplePenaltyReportsForSinglePeer_Concurrently(t *testing.T) {
	alspMetrics := metrics.NewNoopCollector()
	cacheMetrics := metrics.NewNoopCollector()
	cacheSize := uint32(100)

	cfg := &alspmgr.MisbehaviorReportManagerConfig{
		Logger:               unittest.Logger(),
		SpamRecordsCacheSize: cacheSize,
		AlspMetrics:          alspMetrics,
		CacheMetrics:         cacheMetrics,
	}

	cache := internal.NewSpamRecordCache(cfg.SpamRecordsCacheSize, cfg.Logger, cfg.CacheMetrics, model.SpamRecordFactory())

	// create a new MisbehaviorReportManager
	m := alspmgr.NewMisbehaviorReportManager(cfg, alspmgr.WithSpamRecordsCache(cache))

	// creates a list of mock misbehavior reports with negative penalty values for a single peer
	originId := unittest.IdentifierFixture()
	reports := createRandomMisbehaviorReportsForOriginId(t, originId, 5)

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
	record, ok := cache.Get(originId)
	require.True(t, ok)
	require.NotNil(t, record)

	require.Equal(t, totalPenalty, record.Penalty)
	// with just reporting a few misbehavior reports, the cutoff counter should not be incremented.
	require.Equal(t, uint64(0), record.CutoffCounter)
	// the decay should be the default decay value.
	require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)
}

// TestHandleMisbehaviorReport_SinglePenaltyReportsForMultiplePeers_Sequentially tests the handling of single misbehavior reports for multiple peers.
// Reports are coming in sequentially.
// The test ensures that each misbehavior report is handled correctly and the penalties are applied to the corresponding peers in the cache.
func TestHandleMisbehaviorReport_SinglePenaltyReportsForMultiplePeers_Sequentially(t *testing.T) {
	alspMetrics := metrics.NewNoopCollector()
	cacheMetrics := metrics.NewNoopCollector()
	cacheSize := uint32(100)

	cfg := &alspmgr.MisbehaviorReportManagerConfig{
		Logger:               unittest.Logger(),
		SpamRecordsCacheSize: cacheSize,
		AlspMetrics:          alspMetrics,
		CacheMetrics:         cacheMetrics,
	}

	cache := internal.NewSpamRecordCache(cfg.SpamRecordsCacheSize, cfg.Logger, cfg.CacheMetrics, model.SpamRecordFactory())

	// create a new MisbehaviorReportManager
	m := alspmgr.NewMisbehaviorReportManager(cfg, alspmgr.WithSpamRecordsCache(cache))

	// creates a list of single misbehavior reports for multiple peers (10 peers)
	numPeers := 10
	reports := createRandomMisbehaviorReports(t, numPeers)

	channel := channels.Channel("test-channel")

	// handle the misbehavior reports
	for _, report := range reports {
		m.HandleMisbehaviorReport(channel, report)
	}

	// check if the misbehavior reports have been processed by verifying that the Adjust method was called on the cache
	for _, report := range reports {
		originID := report.OriginId()
		record, ok := cache.Get(originID)
		require.True(t, ok)
		require.NotNil(t, record)

		require.Equal(t, report.Penalty(), record.Penalty)
		// with just reporting a single misbehavior report, the cutoff counter should not be incremented.
		require.Equal(t, uint64(0), record.CutoffCounter)
		// the decay should be the default decay value.
		require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)
	}
}

// TestHandleMisbehaviorReport_SinglePenaltyReportsForMultiplePeers_Concurrently tests the handling of single misbehavior reports for multiple peers.
// Reports are coming in concurrently.
// The test ensures that each misbehavior report is handled correctly and the penalties are applied to the corresponding peers in the cache.
func TestHandleMisbehaviorReport_SinglePenaltyReportsForMultiplePeers_Concurrently(t *testing.T) {
	alspMetrics := metrics.NewNoopCollector()
	cacheMetrics := metrics.NewNoopCollector()
	cacheSize := uint32(100)

	cfg := &alspmgr.MisbehaviorReportManagerConfig{
		Logger:               unittest.Logger(),
		SpamRecordsCacheSize: cacheSize,
		AlspMetrics:          alspMetrics,
		CacheMetrics:         cacheMetrics,
	}

	cache := internal.NewSpamRecordCache(cfg.SpamRecordsCacheSize, cfg.Logger, cfg.CacheMetrics, model.SpamRecordFactory())

	// create a new MisbehaviorReportManager
	m := alspmgr.NewMisbehaviorReportManager(cfg, alspmgr.WithSpamRecordsCache(cache))

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
	for _, report := range reports {
		originID := report.OriginId()
		record, ok := cache.Get(originID)
		require.True(t, ok)
		require.NotNil(t, record)

		require.Equal(t, report.Penalty(), record.Penalty)
		// with just reporting a single misbehavior report, the cutoff counter should not be incremented.
		require.Equal(t, uint64(0), record.CutoffCounter)
		// the decay should be the default decay value.
		require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)
	}
}

// TestHandleMisbehaviorReport_MultiplePenaltyReportsForMultiplePeers_Sequentially tests the handling of multiple misbehavior reports for multiple peers.
// Reports are coming in sequentially.
// The test ensures that each misbehavior report is handled correctly and the penalties are cumulatively applied to the corresponding peers in the cache.
func TestHandleMisbehaviorReport_MultiplePenaltyReportsForMultiplePeers_Sequentially(t *testing.T) {
	alspMetrics := metrics.NewNoopCollector()
	cacheMetrics := metrics.NewNoopCollector()
	cacheSize := uint32(100)

	cfg := &alspmgr.MisbehaviorReportManagerConfig{
		Logger:               unittest.Logger(),
		SpamRecordsCacheSize: cacheSize,
		AlspMetrics:          alspMetrics,
		CacheMetrics:         cacheMetrics,
	}

	cache := internal.NewSpamRecordCache(cfg.SpamRecordsCacheSize, cfg.Logger, cfg.CacheMetrics, model.SpamRecordFactory())

	// create a new MisbehaviorReportManager
	m := alspmgr.NewMisbehaviorReportManager(cfg, alspmgr.WithSpamRecordsCache(cache))

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
	totalPenalty := float64(0)
	for _, reports := range peersReports {
		wg.Add(len(reports))
		for _, report := range reports {
			report := report // capture range variable
			totalPenalty += report.Penalty()
			go func() {
				defer wg.Done()

				m.HandleMisbehaviorReport(channel, report)
			}()
		}
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "not all misbehavior reports have been processed")

	// check if the misbehavior reports have been processed by verifying that the Adjust method was called on the cache
	for originID, reports := range peersReports {
		totalPenalty := float64(0)
		for _, report := range reports {
			totalPenalty += report.Penalty()
		}

		record, ok := cache.Get(originID)
		require.True(t, ok)
		require.NotNil(t, record)

		require.Equal(t, totalPenalty, record.Penalty)
		// with just reporting a few misbehavior reports, the cutoff counter should not be incremented.
		require.Equal(t, uint64(0), record.CutoffCounter)
		// the decay should be the default decay value.
		require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)
	}
}

// TestHandleMisbehaviorReport_MultiplePenaltyReportsForMultiplePeers_Sequentially tests the handling of multiple misbehavior reports for multiple peers.
// Reports are coming in concurrently.
// The test ensures that each misbehavior report is handled correctly and the penalties are cumulatively applied to the corresponding peers in the cache.
func TestHandleMisbehaviorReport_MultiplePenaltyReportsForMultiplePeers_Concurrently(t *testing.T) {
	alspMetrics := metrics.NewNoopCollector()
	cacheMetrics := metrics.NewNoopCollector()
	cacheSize := uint32(100)

	cfg := &alspmgr.MisbehaviorReportManagerConfig{
		Logger:               unittest.Logger(),
		SpamRecordsCacheSize: cacheSize,
		AlspMetrics:          alspMetrics,
		CacheMetrics:         cacheMetrics,
	}

	cache := internal.NewSpamRecordCache(cfg.SpamRecordsCacheSize, cfg.Logger, cfg.CacheMetrics, model.SpamRecordFactory())

	// create a new MisbehaviorReportManager
	m := alspmgr.NewMisbehaviorReportManager(cfg, alspmgr.WithSpamRecordsCache(cache))

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
	for originID, reports := range peersReports {
		totalPenalty := float64(0)
		for _, report := range reports {
			totalPenalty += report.Penalty()
		}

		record, ok := cache.Get(originID)
		require.True(t, ok)
		require.NotNil(t, record)

		require.Equal(t, totalPenalty, record.Penalty)
		// with just reporting a few misbehavior reports, the cutoff counter should not be incremented.
		require.Equal(t, uint64(0), record.CutoffCounter)
		// the decay should be the default decay value.
		require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)
	}
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
