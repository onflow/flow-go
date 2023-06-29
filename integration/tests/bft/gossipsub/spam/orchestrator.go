package spam

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/integration/tests/bft"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/utils/unittest"
)

// orchestrator represents a gossipsub spam orchestrator.
type orchestrator struct {
	*bft.BaseOrchestrator
	egressEventReceivedWg sync.WaitGroup
}

var _ insecure.AttackOrchestrator = &orchestrator{}

func NewOrchestrator(t *testing.T, Logger zerolog.Logger) *orchestrator {
	o := &orchestrator{
		BaseOrchestrator: &bft.BaseOrchestrator{
			T:      t,
			Logger: Logger.With().Str("component", "gossipsub-orchestrator").Logger(),
		},
		egressEventReceivedWg: sync.WaitGroup{},
	}
	// track single event received by victim node
	o.OnIngressEvent = append(o.OnIngressEvent, o.trackOneIngressEvent)
	return o
}

func (s *orchestrator) sendOneEgressMessage(t *testing.T) {
	_, event, _ := insecure.EgressMessageFixture(t, unittest.NetworkCodec(), insecure.Protocol_MULTICAST, &message.TestMessage{
		Text: fmt.Sprintf("test message from spam orchestrator: %d", rand.Int()),
	})

	err := s.OrchestratorNetwork.SendEgress(event)
	require.NotNil(t, err)
	s.egressEventReceivedWg.Add(1)
	s.Logger.Info().Str("event_id", event.FlowProtocolEventID.String()).Msg("egress event sent")
}

func (s *orchestrator) sendOneGossipSubMessage(t *testing.T) {

}

// trackOneIngressEvent callback that will track any egress messages that are expected to be received by a victim node
func (s *orchestrator) trackOneIngressEvent(event *insecure.IngressEvent) error {
	s.Logger.Info().Str("event_id", event.FlowProtocolEventID.String()).Msg("ingress event received")
	s.egressEventReceivedWg.Done()
	return nil
}
