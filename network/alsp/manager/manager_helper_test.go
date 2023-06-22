package alspmgr_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/alsp"
	alspmgr "github.com/onflow/flow-go/network/alsp/manager"
	"github.com/onflow/flow-go/network/alsp/model"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

// This file contains helper functions for testing the ALSP manager.

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

// WithDecayFunc sets the decay function for the MisbehaviorReportManager. Useful for testing purposes to simulate the decay of the penalty without waiting for the actual decay.
// Args:
//
//	f: the decay function.
//
// Returns:
//
//	a MisbehaviorReportManagerOption that sets the decay function for the MisbehaviorReportManager.
//
// Note: this option is useful primarily for testing purposes. The default decay function should be used for production.
func WithDecayFunc(f alspmgr.SpamRecordDecayFunc) alspmgr.MisbehaviorReportManagerOption {
	return func(m *alspmgr.MisbehaviorReportManager) {
		m.DecayFunc = f
	}
}

// WithSpamRecordsCacheFactory sets the spam record cache factory for the MisbehaviorReportManager.
// Args:
//
//	f: the spam record cache factory.
//
// Returns:
//
//	a MisbehaviorReportManagerOption that sets the spam record cache for the MisbehaviorReportManager.
//
// Note: this option is useful primarily for testing purposes. The default factory should be sufficient for production.
func WithSpamRecordsCacheFactory(f alspmgr.SpamRecordCacheFactory) alspmgr.MisbehaviorReportManagerOption {
	return func(m *alspmgr.MisbehaviorReportManager) {
		m.CacheFactory = f
	}
}
