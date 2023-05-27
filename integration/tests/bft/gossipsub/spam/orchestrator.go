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

// orchestrator represents a simple orchestrator that passes through all incoming events.
type orchestrator struct {
	*bft.BaseOrchestrator
	sync.Mutex
}

var _ insecure.AttackOrchestrator = &orchestrator{}

func NewSpamOrchestrator(t *testing.T, Logger zerolog.Logger) *orchestrator {
	o := &orchestrator{
		BaseOrchestrator: &bft.BaseOrchestrator{
			T:      t,
			Logger: Logger.With().Str("component", "gossipsub-orchestrator").Logger(),
		},
	}
	return o
}

func (s *orchestrator) sendEgressMessage(t *testing.T) {
	_, event, _ := insecure.EgressMessageFixture(t, unittest.NetworkCodec(), insecure.Protocol_MULTICAST, &message.TestMessage{
		Text: fmt.Sprintf("test message from spam orchestrator: %d", rand.Int()),
	})

	err := s.OrchestratorNetwork.SendEgress(event)
	require.NotNil(t, err)
}
