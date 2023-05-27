package spam

import (
	"sync"
	"testing"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/integration/tests/bft"
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
