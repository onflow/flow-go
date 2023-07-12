package alspmgr_test

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
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
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
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
	ids, nodes, _ := testutils.LibP2PNodeForMiddlewareFixture(t, 1)
	mws, _ := testutils.MiddlewareFixtures(t, ids, nodes, testutils.MiddlewareConfigFixture(t), mocknetwork.NewViolationsConsumer(t))

	networkCfg := testutils.NetworkConfigFixture(t, *ids[0], ids, mws[0])
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
func TestHandleReportedMisbehavior_Cache_Integration(t *testing.T) {
	cfg := managerCfgFixture(t)

	// this test is assessing the integration of the ALSP manager with the network. As the ALSP manager is an attribute
	// of the network, we need to configure the ALSP manager via the network configuration, and let the network create
	// the ALSP manager.
	var cache alsp.SpamRecordCache
	cfg.Opts = []alspmgr.MisbehaviorReportManagerOption{
		alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
			cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
			return cache
		}),
	}
	ids, nodes, _ := testutils.LibP2PNodeForMiddlewareFixture(t, 1)
	mws, _ := testutils.MiddlewareFixtures(t, ids, nodes, testutils.MiddlewareConfigFixture(t), mocknetwork.NewViolationsConsumer(t))
	networkCfg := testutils.NetworkConfigFixture(t, *ids[0], ids, mws[0], p2p.WithAlspConfig(cfg))
	net, err := p2p.NewNetwork(networkCfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	testutils.StartNodesAndNetworks(signalerCtx, t, nodes, []network.Network{net}, 100*time.Millisecond)
	defer testutils.StopComponents[p2p.LibP2PNode](t, nodes, 100*time.Millisecond)
	defer cancel()

	e := mocknetwork.NewEngine(t)
	con, err := net.Register(channels.TestNetworkChannel, e)
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
			r := report // capture range variable
			go func() {
				defer wg.Done()

				con.ReportMisbehavior(r)
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
			// with just reporting a single misbehavior report, the node should not be disallowed.
			require.False(t, record.DisallowListed)
			// the decay should be the default decay value.
			require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)
		}

		return true
	}, 1*time.Second, 10*time.Millisecond, "ALSP manager did not handle the misbehavior report")
}

// TestHandleReportedMisbehavior_And_DisallowListing_Integration implements an end-to-end integration test for the
// handling of reported misbehavior and disallow listing.
//
// The test sets up 3 nodes, one victim, one honest, and one (alledged) spammer.
// Initially, the test ensures that all nodes are connected to each other.
// Then, test imitates that victim node reports the spammer node for spamming.
// The test generates enough spam reports to trigger the disallow-listing of the victim node.
// The test ensures that the victim node is disconnected from the spammer node.
// The test ensures that despite attempting on connections, no inbound or outbound connections between the victim and
// the disallow-listed spammer node are established.
func TestHandleReportedMisbehavior_And_DisallowListing_Integration(t *testing.T) {
	cfg := managerCfgFixture(t)

	// this test is assessing the integration of the ALSP manager with the network. As the ALSP manager is an attribute
	// of the network, we need to configure the ALSP manager via the network configuration, and let the network create
	// the ALSP manager.
	var victimSpamRecordCacheCache alsp.SpamRecordCache
	cfg.Opts = []alspmgr.MisbehaviorReportManagerOption{
		alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
			victimSpamRecordCacheCache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
			return victimSpamRecordCacheCache
		}),
	}

	ids, nodes, _ := testutils.LibP2PNodeForMiddlewareFixture(t, 3,
		p2ptest.WithPeerManagerEnabled(p2ptest.PeerManagerConfigFixture(), nil))
	mws, _ := testutils.MiddlewareFixtures(t, ids, nodes, testutils.MiddlewareConfigFixture(t), mocknetwork.NewViolationsConsumer(t))
	networkCfg := testutils.NetworkConfigFixture(t, *ids[0], ids, mws[0], p2p.WithAlspConfig(cfg))
	victimNetwork, err := p2p.NewNetwork(networkCfg)
	require.NoError(t, err)

	// index of the victim node in the nodes slice.
	victimIndex := 0
	// index of the spammer node in the nodes slice (the node that will be reported for misbehavior and disallow-listed by victim).
	spammerIndex := 1
	// other node (not victim and not spammer) that we have to ensure is not affected by the disallow-listing of the spammer.
	honestIndex := 2

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	testutils.StartNodesAndNetworks(signalerCtx, t, nodes, []network.Network{victimNetwork}, 100*time.Millisecond)
	defer testutils.StopComponents[p2p.LibP2PNode](t, nodes, 100*time.Millisecond)
	defer cancel()

	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)
	// initially victim and spammer should be able to connect to each other.
	p2ptest.TryConnectionAndEnsureConnected(t, ctx, nodes)

	e := mocknetwork.NewEngine(t)
	con, err := victimNetwork.Register(channels.TestNetworkChannel, e)
	require.NoError(t, err)

	// creates a misbehavior report for the spammer
	report := misbehaviorReportFixtureWithPenalty(t, ids[spammerIndex].NodeID, model.DefaultPenaltyValue)

	// simulates the victim node reporting the spammer node misbehavior 120 times
	// to the network. As each report has the default penalty, ideally the spammer should be disallow-listed after
	// 100 reports (each having 0.01 * disallow-listing penalty). But we take 120 as a safe number to ensure that
	// the spammer is definitely disallow-listed.
	reportCount := 120
	wg := sync.WaitGroup{}
	for i := 0; i < reportCount; i++ {
		wg.Add(1)
		// reports the misbehavior
		r := report // capture range variable
		go func() {
			defer wg.Done()

			con.ReportMisbehavior(r)
		}()
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "not all misbehavior reports have been processed")

	// ensures that the spammer is disallow-listed by the victim
	p2ptest.RequireEventuallyNotConnected(t, []p2p.LibP2PNode{nodes[victimIndex]}, []p2p.LibP2PNode{nodes[spammerIndex]}, 100*time.Millisecond, 2*time.Second)

	// despite disallow-listing spammer, ensure that (victim and honest) and (honest and spammer) are still connected.
	p2ptest.RequireConnectedEventually(t, []p2p.LibP2PNode{nodes[spammerIndex], nodes[honestIndex]}, 1*time.Millisecond, 100*time.Millisecond)
	p2ptest.RequireConnectedEventually(t, []p2p.LibP2PNode{nodes[honestIndex], nodes[victimIndex]}, 1*time.Millisecond, 100*time.Millisecond)

	// while the spammer node is disallow-listed, it cannot connect to the victim node. Also, the victim node  cannot directly dial and connect to the spammer node, unless
	// it is allow-listed again.
	p2ptest.EnsureNotConnectedBetweenGroups(t, ctx, []p2p.LibP2PNode{nodes[victimIndex]}, []p2p.LibP2PNode{nodes[spammerIndex]})
}

// TestMisbehaviorReportMetrics tests the recording of misbehavior report metrics.
// It checks that when a misbehavior report is received by the ALSP manager, the metrics are recorded.
// It fails the test if the metrics are not recorded or if they are recorded incorrectly.
func TestMisbehaviorReportMetrics(t *testing.T) {
	cfg := managerCfgFixture(t)

	// this test is assessing the integration of the ALSP manager with the network. As the ALSP manager is an attribute
	// of the network, we need to configure the ALSP manager via the network configuration, and let the network create
	// the ALSP manager.
	alspMetrics := mockmodule.NewAlspMetrics(t)
	cfg.AlspMetrics = alspMetrics

	ids, nodes, _ := testutils.LibP2PNodeForMiddlewareFixture(t, 1)
	mws, _ := testutils.MiddlewareFixtures(t, ids, nodes, testutils.MiddlewareConfigFixture(t), mocknetwork.NewViolationsConsumer(t))
	networkCfg := testutils.NetworkConfigFixture(t, *ids[0], ids, mws[0], p2p.WithAlspConfig(cfg))
	net, err := p2p.NewNetwork(networkCfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	testutils.StartNodesAndNetworks(signalerCtx, t, nodes, []network.Network{net}, 100*time.Millisecond)
	defer testutils.StopComponents[p2p.LibP2PNode](t, nodes, 100*time.Millisecond)
	defer cancel()

	e := mocknetwork.NewEngine(t)
	con, err := net.Register(channels.TestNetworkChannel, e)
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
	cfg := managerCfgFixture(t)
	consumer := mocknetwork.NewDisallowListNotificationConsumer(t)
	var cache alsp.SpamRecordCache
	cfg.Opts = []alspmgr.MisbehaviorReportManagerOption{
		alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
			cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
			return cache
		}),
	}

	t.Run("with default values", func(t *testing.T) {
		m, err := alspmgr.NewMisbehaviorReportManager(cfg, consumer)
		require.NoError(t, err)
		assert.NotNil(t, m)
	})

	t.Run("with a custom spam record cache", func(t *testing.T) {
		m, err := alspmgr.NewMisbehaviorReportManager(cfg, consumer)
		require.NoError(t, err)
		assert.NotNil(t, m)
	})

	t.Run("with ALSP module enabled", func(t *testing.T) {
		m, err := alspmgr.NewMisbehaviorReportManager(cfg, consumer)
		require.NoError(t, err)
		assert.NotNil(t, m)
	})

	t.Run("with ALSP module disabled", func(t *testing.T) {
		m, err := alspmgr.NewMisbehaviorReportManager(cfg, consumer)
		require.NoError(t, err)
		assert.NotNil(t, m)
	})
}

// TestMisbehaviorReportManager_InitializationError tests the creation of a new ALSP manager with invalid inputs.
// It is a minimum viable test that ensures that a nil ALSP manager is created with invalid set of inputs.
func TestMisbehaviorReportManager_InitializationError(t *testing.T) {
	cfg := managerCfgFixture(t)
	consumer := mocknetwork.NewDisallowListNotificationConsumer(t)

	t.Run("missing spam report queue size", func(t *testing.T) {
		cfg.SpamReportQueueSize = 0
		m, err := alspmgr.NewMisbehaviorReportManager(cfg, consumer)
		require.Error(t, err)
		require.ErrorIs(t, err, alspmgr.ErrSpamReportQueueSizeNotSet)
		assert.Nil(t, m)
	})

	t.Run("missing spam record cache size", func(t *testing.T) {
		cfg.SpamRecordCacheSize = 0
		m, err := alspmgr.NewMisbehaviorReportManager(cfg, consumer)
		require.Error(t, err)
		require.ErrorIs(t, err, alspmgr.ErrSpamRecordCacheSizeNotSet)
		assert.Nil(t, m)
	})

	t.Run("missing heartbeat intervals", func(t *testing.T) {
		cfg.HeartBeatInterval = 0
		m, err := alspmgr.NewMisbehaviorReportManager(cfg, consumer)
		require.Error(t, err)
		require.ErrorIs(t, err, alspmgr.ErrSpamRecordCacheSizeNotSet)
		assert.Nil(t, m)
	})
}

// TestHandleMisbehaviorReport_SinglePenaltyReport tests the handling of a single misbehavior report.
// The test ensures that the misbehavior report is handled correctly and the penalty is applied to the peer in the cache.
func TestHandleMisbehaviorReport_SinglePenaltyReport(t *testing.T) {
	cfg := managerCfgFixture(t)
	consumer := mocknetwork.NewDisallowListNotificationConsumer(t)

	// create a new MisbehaviorReportManager
	var cache alsp.SpamRecordCache
	cfg.Opts = []alspmgr.MisbehaviorReportManagerOption{
		alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
			cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
			return cache
		}),
	}

	m, err := alspmgr.NewMisbehaviorReportManager(cfg, consumer)
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
		require.False(t, record.DisallowListed)                                                       // the peer should not be disallow listed yet
		require.Equal(t, uint64(0), record.CutoffCounter)                                             // with just reporting a misbehavior, the cutoff counter should not be incremented.
		require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay) // the decay should be the default decay value.

		return true
	}, 1*time.Second, 10*time.Millisecond, "ALSP manager did not handle the misbehavior report")
}

// TestHandleMisbehaviorReport_SinglePenaltyReport_PenaltyDisable tests the handling of a single misbehavior report when the penalty is disabled.
// The test ensures that the misbehavior is reported on metrics but the penalty is not applied to the peer in the cache.
func TestHandleMisbehaviorReport_SinglePenaltyReport_PenaltyDisable(t *testing.T) {
	cfg := managerCfgFixture(t)
	consumer := mocknetwork.NewDisallowListNotificationConsumer(t)

	cfg.DisablePenalty = true // disable penalty for misbehavior reports
	alspMetrics := mockmodule.NewAlspMetrics(t)
	cfg.AlspMetrics = alspMetrics

	// we use a mock cache but we do not expect any calls to the cache, since the penalty is disabled.
	var cache *mockalsp.SpamRecordCache
	cfg.Opts = []alspmgr.MisbehaviorReportManagerOption{
		alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
			cache = mockalsp.NewSpamRecordCache(t)
			return cache
		}),
	}
	m, err := alspmgr.NewMisbehaviorReportManager(cfg, consumer)
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

// TestHandleMisbehaviorReport_MultiplePenaltyReportsForSinglePeer_Sequentially tests the handling of multiple misbehavior reports for a single peer.
// Reports are coming in sequentially.
// The test ensures that each misbehavior report is handled correctly and the penalties are cumulatively applied to the peer in the cache.
func TestHandleMisbehaviorReport_MultiplePenaltyReportsForSinglePeer_Sequentially(t *testing.T) {
	cfg := managerCfgFixture(t)
	consumer := mocknetwork.NewDisallowListNotificationConsumer(t)

	// create a new MisbehaviorReportManager
	var cache alsp.SpamRecordCache
	cfg.Opts = []alspmgr.MisbehaviorReportManagerOption{
		alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
			cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
			return cache
		}),
	}
	m, err := alspmgr.NewMisbehaviorReportManager(cfg, consumer)
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

	require.Eventually(t, func() bool {
		// check if the misbehavior report has been processed by verifying that the Adjust method was called on the cache
		record, ok := cache.Get(originId)
		if !ok {
			return false
		}
		require.NotNil(t, record)

		if totalPenalty != record.Penalty {
			// all the misbehavior reports should be processed by now, so the penalty should be equal to the total penalty
			return false
		}
		require.False(t, record.DisallowListed) // the peer should not be disallow listed yet.
		// with just reporting a few misbehavior reports, the cutoff counter should not be incremented.
		require.Equal(t, uint64(0), record.CutoffCounter)
		// the decay should be the default decay value.
		require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)

		return true
	}, 1*time.Second, 10*time.Millisecond, "ALSP manager did not handle the misbehavior report")
}

// TestHandleMisbehaviorReport_MultiplePenaltyReportsForSinglePeer_Sequential tests the handling of multiple misbehavior reports for a single peer.
// Reports are coming in concurrently.
// The test ensures that each misbehavior report is handled correctly and the penalties are cumulatively applied to the peer in the cache.
func TestHandleMisbehaviorReport_MultiplePenaltyReportsForSinglePeer_Concurrently(t *testing.T) {
	cfg := managerCfgFixture(t)
	consumer := mocknetwork.NewDisallowListNotificationConsumer(t)

	var cache alsp.SpamRecordCache
	cfg.Opts = []alspmgr.MisbehaviorReportManagerOption{
		alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
			cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
			return cache
		}),
	}
	m, err := alspmgr.NewMisbehaviorReportManager(cfg, consumer)
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

	// creates a list of mock misbehavior reports with negative penalty values for a single peer
	originId := unittest.IdentifierFixture()
	reports := createRandomMisbehaviorReportsForOriginId(t, originId, 5)

	channel := channels.Channel("test-channel")

	wg := sync.WaitGroup{}
	wg.Add(len(reports))
	// handle the misbehavior reports
	totalPenalty := float64(0)
	for _, report := range reports {
		r := report // capture range variable
		totalPenalty += report.Penalty()
		go func() {
			defer wg.Done()

			m.HandleMisbehaviorReport(channel, r)
		}()
	}

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "not all misbehavior reports have been processed")

	require.Eventually(t, func() bool {
		// check if the misbehavior report has been processed by verifying that the Adjust method was called on the cache
		record, ok := cache.Get(originId)
		if !ok {
			return false
		}
		require.NotNil(t, record)

		if totalPenalty != record.Penalty {
			// all the misbehavior reports should be processed by now, so the penalty should be equal to the total penalty
			return false
		}
		require.False(t, record.DisallowListed) // the peer should not be disallow listed yet.
		// with just reporting a few misbehavior reports, the cutoff counter should not be incremented.
		require.Equal(t, uint64(0), record.CutoffCounter)
		// the decay should be the default decay value.
		require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)

		return true
	}, 1*time.Second, 10*time.Millisecond, "ALSP manager did not handle the misbehavior report")
}

// TestHandleMisbehaviorReport_SinglePenaltyReportsForMultiplePeers_Sequentially tests the handling of single misbehavior reports for multiple peers.
// Reports are coming in sequentially.
// The test ensures that each misbehavior report is handled correctly and the penalties are applied to the corresponding peers in the cache.
func TestHandleMisbehaviorReport_SinglePenaltyReportsForMultiplePeers_Sequentially(t *testing.T) {
	cfg := managerCfgFixture(t)
	consumer := mocknetwork.NewDisallowListNotificationConsumer(t)

	var cache alsp.SpamRecordCache
	cfg.Opts = []alspmgr.MisbehaviorReportManagerOption{
		alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
			cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
			return cache
		}),
	}
	m, err := alspmgr.NewMisbehaviorReportManager(cfg, consumer)
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

	// handle the misbehavior reports
	for _, report := range reports {
		m.HandleMisbehaviorReport(channel, report)
	}

	// check if the misbehavior reports have been processed by verifying that the Adjust method was called on the cache
	require.Eventually(t, func() bool {
		for _, report := range reports {
			originID := report.OriginId()
			record, ok := cache.Get(originID)
			if !ok {
				return false
			}
			require.NotNil(t, record)
			require.False(t, record.DisallowListed) // the peer should not be disallow listed yet.
			require.Equal(t, report.Penalty(), record.Penalty)
			// with just reporting a single misbehavior report, the cutoff counter should not be incremented.
			require.Equal(t, uint64(0), record.CutoffCounter)
			// the decay should be the default decay value.
			require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)
		}

		return true
	}, 1*time.Second, 10*time.Millisecond, "ALSP manager did not handle the misbehavior report")

}

// TestHandleMisbehaviorReport_SinglePenaltyReportsForMultiplePeers_Concurrently tests the handling of single misbehavior reports for multiple peers.
// Reports are coming in concurrently.
// The test ensures that each misbehavior report is handled correctly and the penalties are applied to the corresponding peers in the cache.
func TestHandleMisbehaviorReport_SinglePenaltyReportsForMultiplePeers_Concurrently(t *testing.T) {
	cfg := managerCfgFixture(t)
	consumer := mocknetwork.NewDisallowListNotificationConsumer(t)

	var cache alsp.SpamRecordCache
	cfg.Opts = []alspmgr.MisbehaviorReportManagerOption{
		alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
			cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
			return cache
		}),
	}
	m, err := alspmgr.NewMisbehaviorReportManager(cfg, consumer)
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
		r := report // capture range variable
		totalPenalty += report.Penalty()
		go func() {
			defer wg.Done()

			m.HandleMisbehaviorReport(channel, r)
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
			require.False(t, record.DisallowListed) // the peer should not be disallow listed yet.
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
	cfg := managerCfgFixture(t)
	consumer := mocknetwork.NewDisallowListNotificationConsumer(t)

	var cache alsp.SpamRecordCache
	cfg.Opts = []alspmgr.MisbehaviorReportManagerOption{
		alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
			cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
			return cache
		}),
	}
	m, err := alspmgr.NewMisbehaviorReportManager(cfg, consumer)
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
			r := report // capture range variable
			go func() {
				defer wg.Done()

				m.HandleMisbehaviorReport(channel, r)
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
			require.False(t, record.DisallowListed) // the peer should not be disallow listed yet.
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
	cfg := managerCfgFixture(t)
	consumer := mocknetwork.NewDisallowListNotificationConsumer(t)

	var cache alsp.SpamRecordCache
	cfg.Opts = []alspmgr.MisbehaviorReportManagerOption{
		alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
			cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
			return cache
		}),
	}
	m, err := alspmgr.NewMisbehaviorReportManager(cfg, consumer)
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
			require.False(t, record.DisallowListed) // the peer should not be disallow listed yet.
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
	cfg := managerCfgFixture(t)
	consumer := mocknetwork.NewDisallowListNotificationConsumer(t)

	var cache alsp.SpamRecordCache
	cfg.Opts = []alspmgr.MisbehaviorReportManagerOption{
		alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
			cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
			return cache
		}),
	}
	m, err := alspmgr.NewMisbehaviorReportManager(cfg, consumer)
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
	report := misbehaviorReportFixture(t, originId)

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
		require.False(t, record.DisallowListed) // the peer should not be disallow listed yet.
		// with just reporting a few misbehavior reports, the cutoff counter should not be incremented.
		require.Equal(t, uint64(0), record.CutoffCounter)
		// the decay should be the default decay value.
		require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)

		return true
	}, 1*time.Second, 10*time.Millisecond, "ALSP manager did not handle the misbehavior report")
}

// TestDecayMisbehaviorPenalty_SingleHeartbeat tests the decay of the misbehavior penalty. The test ensures that the misbehavior penalty
// is decayed after a single heartbeat. The test guarantees waiting for at least one heartbeat by waiting for the first decay to happen.
func TestDecayMisbehaviorPenalty_SingleHeartbeat(t *testing.T) {
	cfg := managerCfgFixture(t)
	consumer := mocknetwork.NewDisallowListNotificationConsumer(t)

	var cache alsp.SpamRecordCache
	cfg.Opts = []alspmgr.MisbehaviorReportManagerOption{
		alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
			cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
			return cache
		}),
	}
	m, err := alspmgr.NewMisbehaviorReportManager(cfg, consumer)
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
	report := misbehaviorReportFixtureWithDefaultPenalty(t, originId)
	require.Less(t, report.Penalty(), float64(0)) // ensure the penalty is negative

	channel := channels.Channel("test-channel")

	// number of times the duplicate misbehavior report is reported concurrently
	times := 10
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

	// phase-1: eventually all the misbehavior reports should be processed.
	penaltyBeforeDecay := float64(0)
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
		require.False(t, record.DisallowListed) // the peer should not be disallow listed yet.
		// with just reporting a few misbehavior reports, the cutoff counter should not be incremented.
		require.Equal(t, uint64(0), record.CutoffCounter)
		// the decay should be the default decay value.
		require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)

		penaltyBeforeDecay = record.Penalty
		return true
	}, 1*time.Second, 10*time.Millisecond, "ALSP manager did not handle the misbehavior report")

	// phase-2: wait enough for at least one heartbeat to be processed.
	time.Sleep(1 * time.Second)

	// phase-3: check if the penalty was decayed for at least one heartbeat.
	record, ok := cache.Get(originId)
	require.True(t, ok) // the record should be in the cache
	require.NotNil(t, record)

	// with at least a single heartbeat, the penalty should be greater than the penalty before the decay.
	require.Greater(t, record.Penalty, penaltyBeforeDecay)
	// we waited for at most one heartbeat, so the decayed penalty should be still less than the value after 2 heartbeats.
	require.Less(t, record.Penalty, penaltyBeforeDecay+2*record.Decay)
	// with just reporting a few misbehavior reports, the cutoff counter should not be incremented.
	require.Equal(t, uint64(0), record.CutoffCounter)
	// the decay should be the default decay value.
	require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)
}

// TestDecayMisbehaviorPenalty_MultipleHeartbeat tests the decay of the misbehavior penalty under multiple heartbeats.
// The test ensures that the misbehavior penalty is decayed with a linear progression within multiple heartbeats.
func TestDecayMisbehaviorPenalty_MultipleHeartbeats(t *testing.T) {
	cfg := managerCfgFixture(t)
	consumer := mocknetwork.NewDisallowListNotificationConsumer(t)

	var cache alsp.SpamRecordCache
	cfg.Opts = []alspmgr.MisbehaviorReportManagerOption{
		alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
			cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
			return cache
		}),
	}
	m, err := alspmgr.NewMisbehaviorReportManager(cfg, consumer)
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
	report := misbehaviorReportFixtureWithDefaultPenalty(t, originId)
	require.Less(t, report.Penalty(), float64(0)) // ensure the penalty is negative

	channel := channels.Channel("test-channel")

	// number of times the duplicate misbehavior report is reported concurrently
	times := 10
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

	// phase-1: eventually all the misbehavior reports should be processed.
	penaltyBeforeDecay := float64(0)
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

		penaltyBeforeDecay = record.Penalty
		return true
	}, 1*time.Second, 10*time.Millisecond, "ALSP manager did not handle the misbehavior report")

	// phase-2: wait for 3 heartbeats to be processed.
	time.Sleep(3 * time.Second)

	// phase-3: check if the penalty was decayed in a linear progression.
	record, ok := cache.Get(originId)
	require.True(t, ok) // the record should be in the cache
	require.NotNil(t, record)

	// with 3 heartbeats processed, the penalty should be greater than the penalty before the decay.
	require.Greater(t, record.Penalty, penaltyBeforeDecay)
	// with 3 heartbeats processed, the decayed penalty should be less than the value after 4 heartbeats.
	require.Less(t, record.Penalty, penaltyBeforeDecay+4*record.Decay)
	require.False(t, record.DisallowListed) // the peer should not be disallow listed yet.
	// with just reporting a few misbehavior reports, the cutoff counter should not be incremented.
	require.Equal(t, uint64(0), record.CutoffCounter)
	// the decay should be the default decay value.
	require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)
}

// TestDecayMisbehaviorPenalty_MultipleHeartbeat tests the decay of the misbehavior penalty under multiple heartbeats.
// The test ensures that the misbehavior penalty is decayed with a linear progression within multiple heartbeats.
func TestDecayMisbehaviorPenalty_DecayToZero(t *testing.T) {
	cfg := managerCfgFixture(t)
	consumer := mocknetwork.NewDisallowListNotificationConsumer(t)

	var cache alsp.SpamRecordCache
	cfg.Opts = []alspmgr.MisbehaviorReportManagerOption{
		alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
			cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
			return cache
		}),
	}
	m, err := alspmgr.NewMisbehaviorReportManager(cfg, consumer)
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
	report := misbehaviorReportFixture(t, originId) // penalties are between -1 and -10
	require.Less(t, report.Penalty(), float64(0))   // ensure the penalty is negative

	channel := channels.Channel("test-channel")

	// number of times the duplicate misbehavior report is reported concurrently
	times := 10
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

	// phase-1: eventually all the misbehavior reports should be processed.
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

	// phase-2: default decay speed is 1000 and with 10 penalties in range of [-1, -10], the penalty should be decayed to zero in
	// a single heartbeat.
	time.Sleep(1 * time.Second)

	// phase-3: check if the penalty was decayed to zero.
	record, ok := cache.Get(originId)
	require.True(t, ok) // the record should be in the cache
	require.NotNil(t, record)

	require.False(t, record.DisallowListed) // the peer should not be disallow listed yet.
	// with a single heartbeat and decay speed of 1000, the penalty should be decayed to zero.
	require.Equal(t, float64(0), record.Penalty)
	// the decay should be the default decay value.
	require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)
}

// TestDecayMisbehaviorPenalty_DecayToZero_AllowListing tests that when the misbehavior penalty of an already disallow-listed
// peer is decayed to zero, the peer is allow-listed back in the network, and its spam record cache is updated accordingly.
func TestDecayMisbehaviorPenalty_DecayToZero_AllowListing(t *testing.T) {
	cfg := managerCfgFixture(t)
	consumer := mocknetwork.NewDisallowListNotificationConsumer(t)

	var cache alsp.SpamRecordCache
	cfg.Opts = []alspmgr.MisbehaviorReportManagerOption{
		alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
			cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
			return cache
		}),
	}
	m, err := alspmgr.NewMisbehaviorReportManager(cfg, consumer)
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

	// simulates a disallow-listed peer in cache.
	originId := unittest.IdentifierFixture()
	penalty, err := cache.Adjust(originId, func(record model.ProtocolSpamRecord) (model.ProtocolSpamRecord, error) {
		record.Penalty = -10 // set the penalty to -10 to simulate that the penalty has already been decayed for a while.
		record.CutoffCounter = 1
		record.DisallowListed = true
		record.OriginId = originId
		record.Decay = model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay
		return record, nil
	})
	require.NoError(t, err)
	require.Equal(t, float64(-10), penalty)

	// sanity check
	record, ok := cache.Get(originId)
	require.True(t, ok) // the record should be in the cache
	require.NotNil(t, record)
	require.Equal(t, float64(-10), record.Penalty)
	require.True(t, record.DisallowListed)
	require.Equal(t, uint64(1), record.CutoffCounter)
	require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)

	// eventually, we expect the ALSP manager to emit an allow list notification to the network layer when the penalty is decayed to zero.
	consumer.On("OnAllowListNotification", &network.AllowListingUpdate{
		FlowIds: flow.IdentifierList{originId},
		Cause:   network.DisallowListedCauseAlsp,
	}).Return(nil).Once()

	// wait for at most two heartbeats; default decay speed is 1000 and with a penalty of -10, the penalty should be decayed to zero in a single heartbeat.
	require.Eventually(t, func() bool {
		record, ok = cache.Get(originId)
		if !ok {
			return false
		}
		if record.DisallowListed {
			return false // the peer should not be allow-listed yet.
		}
		if record.Penalty != float64(0) {
			return false // the penalty should be decayed to zero.
		}
		if record.CutoffCounter != 1 {
			return false // the cutoff counter should be incremented.
		}
		if record.Decay != model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay {
			return false // the decay should be the default decay value.
		}

		return true

	}, 2*time.Second, 10*time.Millisecond, "penalty was not decayed to zero")

}

// TestDisallowListNotification tests the emission of the allow list notification to the network layer when the misbehavior
// penalty of a node is dropped below the disallow-listing threshold. The test ensures that the disallow list notification is
// emitted to the network layer when the misbehavior penalty is dropped below the disallow-listing threshold and that the
// cutoff counter of the spam record for the misbehaving node is incremented indicating that the node is disallow-listed once.
func TestDisallowListNotification(t *testing.T) {
	cfg := managerCfgFixture(t)
	consumer := mocknetwork.NewDisallowListNotificationConsumer(t)

	var cache alsp.SpamRecordCache
	cfg.Opts = []alspmgr.MisbehaviorReportManagerOption{
		alspmgr.WithSpamRecordsCacheFactory(func(logger zerolog.Logger, size uint32, metrics module.HeroCacheMetrics) alsp.SpamRecordCache {
			cache = internal.NewSpamRecordCache(size, logger, metrics, model.SpamRecordFactory())
			return cache
		}),
	}
	m, err := alspmgr.NewMisbehaviorReportManager(cfg, consumer)
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
	report := misbehaviorReportFixtureWithDefaultPenalty(t, originId)
	require.Less(t, report.Penalty(), float64(0)) // ensure the penalty is negative

	channel := channels.Channel("test-channel")

	// reporting the same misbehavior 120 times, should result in a single disallow list notification, since each
	// misbehavior report is reported with the same penalty 0.01 * diallowlisting-threshold. We go over the threshold
	// to ensure that the disallow list notification is emitted only once.
	times := 120
	wg := sync.WaitGroup{}
	wg.Add(times)

	// concurrently reports the same misbehavior report twice
	for i := 0; i < times; i++ {
		go func() {
			defer wg.Done()

			m.HandleMisbehaviorReport(channel, report)
		}()
	}

	// at this point, we expect a single disallow list notification to be emitted to the network layer when all the misbehavior
	// reports are processed by the ALSP manager (the notification is emitted when at the next heartbeat).
	consumer.On("OnDisallowListNotification", &network.DisallowListingUpdate{
		FlowIds: flow.IdentifierList{report.OriginId()},
		Cause:   network.DisallowListedCauseAlsp,
	}).Return().Once()

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "not all misbehavior reports have been processed")

	require.Eventually(t, func() bool {
		// check if the misbehavior reports have been processed by verifying that the Adjust method was called on the cache
		record, ok := cache.Get(originId)
		if !ok {
			return false
		}
		require.NotNil(t, record)

		// eventually, the penalty should be the accumulated penalty of all the duplicate misbehavior reports (with the default decay).
		// the decay is added to the penalty as we allow for a single heartbeat before the disallow list notification is emitted.
		if record.Penalty != report.Penalty()*float64(times)+record.Decay {
			return false
		}
		require.True(t, record.DisallowListed) // the peer should be disallow-listed.
		// cuttoff counter should be incremented since the penalty is above the disallowlisting threshold.
		require.Equal(t, uint64(1), record.CutoffCounter)
		// the decay should be the default decay value.
		require.Equal(t, model.SpamRecordFactory()(unittest.IdentifierFixture()).Decay, record.Decay)

		return true
	}, 1*time.Second, 10*time.Millisecond, "ALSP manager did not handle the misbehavior report")
}

// misbehaviorReportFixture creates a mock misbehavior report for a single origin id.
// Args:
// - t: the testing.T instance
// - originID: the origin id of the misbehavior report
// Returns:
// - network.MisbehaviorReport: the misbehavior report
// Note: the penalty of the misbehavior report is randomly chosen between -1 and -10.
func misbehaviorReportFixture(t *testing.T, originID flow.Identifier) network.MisbehaviorReport {
	return misbehaviorReportFixtureWithPenalty(t, originID, math.Min(-1, float64(-1-rand.Intn(10))))
}

func misbehaviorReportFixtureWithDefaultPenalty(t *testing.T, originID flow.Identifier) network.MisbehaviorReport {
	return misbehaviorReportFixtureWithPenalty(t, originID, model.DefaultPenaltyValue)
}

func misbehaviorReportFixtureWithPenalty(t *testing.T, originID flow.Identifier, penalty float64) network.MisbehaviorReport {
	report := mocknetwork.NewMisbehaviorReport(t)
	report.On("OriginId").Return(originID)
	report.On("Reason").Return(alsp.AllMisbehaviorTypes()[rand.Intn(len(alsp.AllMisbehaviorTypes()))])
	report.On("Penalty").Return(penalty)

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
		reports[i] = misbehaviorReportFixture(t, originID)
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
		reports[i] = misbehaviorReportFixture(t, unittest.IdentifierFixture())
	}

	return reports
}

// managerCfgFixture creates a new MisbehaviorReportManagerConfig with default values for testing.
func managerCfgFixture(t *testing.T) *alspmgr.MisbehaviorReportManagerConfig {
	c, err := config.DefaultConfig()
	require.NoError(t, err)
	return &alspmgr.MisbehaviorReportManagerConfig{
		Logger:                  unittest.Logger(),
		SpamRecordCacheSize:     c.NetworkConfig.AlspConfig.SpamRecordCacheSize,
		SpamReportQueueSize:     c.NetworkConfig.AlspConfig.SpamReportQueueSize,
		HeartBeatInterval:       c.NetworkConfig.AlspConfig.HearBeatInterval,
		AlspMetrics:             metrics.NewNoopCollector(),
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
	}
}
