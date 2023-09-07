package testutils

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/alsp"
	"github.com/onflow/flow-go/utils/unittest"
)

// MisbehaviorReportFixture generates a random misbehavior report.
// Args:
//   - t: the test object.
//
// This is used in tests to generate random misbehavior reports.
// It fails the test if it cannot generate a valid report.
func MisbehaviorReportFixture(t *testing.T) network.MisbehaviorReport {

	// pick a random misbehavior type
	misbehaviorType := alsp.AllMisbehaviorTypes()[rand.Intn(len(alsp.AllMisbehaviorTypes()))]

	amplification := 100 * rand.Float64()
	report, err := alsp.NewMisbehaviorReport(
		unittest.IdentifierFixture(),
		misbehaviorType,
		alsp.WithPenaltyAmplification(amplification))
	require.NoError(t, err)
	return report
}

// MisbehaviorReportsFixture generates a slice of random misbehavior reports.
// Args:
//   - t: the test object.
//
// It fails the test if it cannot generate a valid report.
// This is used in tests to generate random misbehavior reports.
func MisbehaviorReportsFixture(t *testing.T, count int) []network.MisbehaviorReport {
	reports := make([]network.MisbehaviorReport, 0, count)
	for i := 0; i < count; i++ {
		reports = append(reports, MisbehaviorReportFixture(t))
	}

	return reports
}

// MisbehaviorTypeFixture generates a random misbehavior type.
// Args:
//   - t: the test object (used to emphasize that this is a test helper).
func MisbehaviorTypeFixture(_ *testing.T) network.Misbehavior {
	return alsp.AllMisbehaviorTypes()[rand.Intn(len(alsp.AllMisbehaviorTypes()))]
}
