package disallowlisting

import (
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/integration/tests/bft"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	// numOfAuthorizedEvents number of events to send before disallow-listing the sender node via the disallow-list command.
	// This threshold must be set to a low value to make the test conclude faster
	// by waiting for fewer events, which is beneficial when running the test
	numOfAuthorizedEvents = 5

	// numOfUnauthorizedEvents number of events to send after disallow-listing the sender node via the disallow-list command.
	// This threshold must be set to a low value to make the test conclude faster
	// by waiting for fewer events, which is beneficial when running the test
	// on an asynchronous network where event delivery can be unpredictable.
	numOfUnauthorizedEvents = 5
)

// Orchestrator represents a simple `insecure.AttackOrchestrator` that tracks messages received before and after the
// senderVN is disallow-listed by the receiverEN via the admin disallow-list command.
type Orchestrator struct {
	*bft.BaseOrchestrator
	sync.Mutex
	codec                         network.Codec
	expectedBlockedEventsReceived *atomic.Int64
	authorizedEventsReceived      *atomic.Int64
	expectedBlockedEvents         map[flow.Identifier]*insecure.EgressEvent
	authorizedEvents              map[flow.Identifier]*insecure.EgressEvent
	authorizedEventsReceivedWg    sync.WaitGroup
	senderVN                      flow.Identifier
	receiverEN                    flow.Identifier
}

var _ insecure.AttackOrchestrator = &Orchestrator{}

func NewOrchestrator(t *testing.T, logger zerolog.Logger, senderVN, receiverEN flow.Identifier) *Orchestrator {
	orchestrator := &Orchestrator{
		BaseOrchestrator: &bft.BaseOrchestrator{
			T:      t,
			Logger: logger,
		},
		codec:                         unittest.NetworkCodec(),
		expectedBlockedEventsReceived: atomic.NewInt64(0),
		authorizedEventsReceived:      atomic.NewInt64(0),
		expectedBlockedEvents:         make(map[flow.Identifier]*insecure.EgressEvent),
		authorizedEvents:              make(map[flow.Identifier]*insecure.EgressEvent),
		authorizedEventsReceivedWg:    sync.WaitGroup{},
		senderVN:                      senderVN,
		receiverEN:                    receiverEN,
	}

	orchestrator.OnIngressEvent = append(orchestrator.OnIngressEvent, orchestrator.trackIngressEvents)

	return orchestrator
}

// trackIngressEvents callback that will track authorized messages that are expected to be received by the receiverEN before we disallow-list the sender.
// It also tracks unauthorized messages received if any that are expected to be blocked after the senderVN is disallow-listed via the admin disallow-list command.
func (a *Orchestrator) trackIngressEvents(event *insecure.IngressEvent) error {
	// Track any unauthorized events that are received, these events are sent after the admin disallow-list command
	// is used to disallow-list the sender node.
	if _, ok := a.expectedBlockedEvents[event.FlowProtocolEventID]; ok {
		if event.OriginID == a.senderVN {
			a.expectedBlockedEventsReceived.Inc()
			a.Logger.Warn().Str("event_id", event.FlowProtocolEventID.String()).Msg("unauthorized ingress event received")
		}
	}

	// track all authorized events sent before the sender node is disallow-listed.
	if _, ok := a.authorizedEvents[event.FlowProtocolEventID]; ok {
		// ensure event received intact no changes have been made to the underlying message
		//a.assertEventsEqual(expectedEvent, event)
		a.authorizedEventsReceived.Inc()
		a.authorizedEventsReceivedWg.Done()
	}

	return nil
}

// sendAuthorizedMsgs publishes a number of authorized messages from the senderVN. Authorized messages are messages
// that are sent before the senderVN is disallow-listed.
func (a *Orchestrator) sendAuthorizedMsgs(t *testing.T) {
	for i := 0; i < numOfAuthorizedEvents; i++ {
		event := bft.RequestChunkDataPackEgressFixture(a.T, a.senderVN, a.receiverEN, insecure.Protocol_PUBLISH)
		err := a.OrchestratorNetwork.SendEgress(event)
		require.NoError(t, err)
		a.authorizedEvents[event.FlowProtocolEventID] = event
		a.authorizedEventsReceivedWg.Add(1)
	}
}

// sendExpectedBlockedMsgs publishes a number of unauthorized messages. Unauthorized messages are messages that are sent
// after the senderVN is blocked via the admin disallow-listed command. These messages are not expected to be received.
func (a *Orchestrator) sendExpectedBlockedMsgs(t *testing.T) {
	for i := 0; i < numOfUnauthorizedEvents; i++ {
		event := bft.RequestChunkDataPackEgressFixture(a.T, a.senderVN, a.receiverEN, insecure.Protocol_PUBLISH)
		err := a.OrchestratorNetwork.SendEgress(event)
		require.NoError(t, err)
		a.expectedBlockedEvents[event.FlowProtocolEventID] = event
	}
}
