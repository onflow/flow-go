package alspmgr_test

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/alsp"
	"github.com/onflow/flow-go/network/alsp/internal"
	alspmgr "github.com/onflow/flow-go/network/alsp/manager"
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
func TestHandleReportedMisbehavior(t *testing.T) {
	misbehaviorReportManger := mocknetwork.NewMisbehaviorReportManager(t)
	conduitFactory := conduit.NewDefaultConduitFactory(
		&alspmgr.MisbehaviorReportManagerConfig{
			Enabled:      true,
			Logger:       unittest.Logger(),
			AlspMetrics:  metrics.NewNoopCollector(),
			CacheMetrics: metrics.NewNoopCollector(),
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
			Enabled:      true,
			Logger:       unittest.Logger(),
			AlspMetrics:  metrics.NewNoopCollector(),
			CacheMetrics: metrics.NewNoopCollector(),
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
	logger := zerolog.Nop()
	alspMetrics := metrics.NewNoopCollector()
	cacheMetrics := metrics.NewNoopCollector()
	cacheSize := uint32(100)

	t.Run("with default values", func(t *testing.T) {
		cfg := &alspmgr.MisbehaviorReportManagerConfig{
			Logger:               logger,
			SpamRecordsCacheSize: cacheSize,
			AlspMetrics:          alspMetrics,
			CacheMetrics:         cacheMetrics,
			Enabled:              true,
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
			Enabled:              true,
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
			Enabled:              true,
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
			Enabled:              false,
		}

		m := alspmgr.NewMisbehaviorReportManager(cfg)
		assert.NotNil(t, m)
	})
}
