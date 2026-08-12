package slashing_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
	mocknetwork "github.com/onflow/flow-go/network/mock"
	"github.com/onflow/flow-go/network/slashing"
	"github.com/onflow/flow-go/utils/unittest"
)

// newConsumerFixture returns a slashing violations consumer whose metrics and misbehavior
// report consumer tolerate any number of calls.
func newConsumerFixture(t *testing.T) *slashing.Consumer {
	metrics := mockmodule.NewNetworkSecurityMetrics(t)
	metrics.On("OnUnauthorizedMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Maybe()
	metrics.On("OnViolationReportSkipped").Return().Maybe()

	misbehaviorReportConsumer := mocknetwork.NewMisbehaviorReportConsumer(t)
	misbehaviorReportConsumer.On("ReportMisbehaviorOnChannel", mock.Anything, mock.Anything).Return().Maybe()

	return slashing.NewSlashingViolationsConsumer(unittest.Logger(), metrics, misbehaviorReportConsumer)
}

// violationFixture returns a violation without an identity (a violation from an unknown peer)
// and without a message type (a violation raised before the message could be decoded).
func violationFixture() *network.Violation {
	return &network.Violation{
		Identity: nil,
		PeerID:   "peer-id",
		OriginID: flow.ZeroID,
		MsgType:  "", // simulates a violation raised before the message type is known
		Channel:  channels.TestNetworkChannel,
		Protocol: message.ProtocolTypeUnicast,
		Err:      errors.New("unauthorized"),
	}
}

// TestConsumer_DoesNotMutateViolation is a regression test verifying that the consumer never
// mutates the violation passed to it: callers may share one violation object across goroutines
// (or reuse it for multiple notifications), so an in-place default (e.g. setting MsgType to
// "unknown") would be a data race and would leak into subsequent uses.
func TestConsumer_DoesNotMutateViolation(t *testing.T) {
	consumer := newConsumerFixture(t)

	violation := violationFixture()
	consumer.OnUnauthorizedSenderError(violation)

	require.Empty(t, violation.MsgType, "consumer must not mutate the violation's MsgType")
	require.Nil(t, violation.Identity, "consumer must not mutate the violation's Identity")
}

// TestConsumer_ConcurrentNotifications is a regression test verifying that a single violation
// object can be reported concurrently through all consumer entry points without a data race
// (run with -race). Before the fix, logOffense wrote a default MsgType into the shared
// violation, racing the reads of concurrent notifications.
func TestConsumer_ConcurrentNotifications(t *testing.T) {
	consumer := newConsumerFixture(t)

	violation := violationFixture()

	notify := []func(*network.Violation){
		consumer.OnUnauthorizedSenderError,
		consumer.OnUnknownMsgTypeError,
		consumer.OnInvalidMsgError,
		consumer.OnSenderEjectedError,
		consumer.OnUnauthorizedUnicastOnChannel,
		consumer.OnUnauthorizedPublishOnChannel,
	}

	workers := 4
	iterations := 50
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for i := range iterations {
				notify[i%len(notify)](violation)
			}
		})
	}
	unittest.RequireReturnsBefore(t, wg.Wait, 10*time.Second, "concurrent notifications did not finish on time")

	require.Empty(t, violation.MsgType, "consumer must not mutate the violation's MsgType")
}
