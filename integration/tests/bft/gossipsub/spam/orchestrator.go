package spam

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/integration/tests/bft"
	"github.com/onflow/flow-go/utils/logging"
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

// HandleIngressEvent implements logic of processing the incoming (ingress) events to a corrupt node.
func (o *orchestrator) HandleIngressEvent(event *insecure.IngressEvent) error {
	lg := o.Logger.With().
		Hex("origin_id", logging.ID(event.OriginID)).
		Str("channel", event.Channel.String()).
		Str("corrupt_target_id", fmt.Sprintf("%v", event.CorruptTargetID)).
		Str("flow_protocol_event", fmt.Sprintf("%T", event.FlowProtocolEvent)).Logger()

	err := o.OrchestratorNetwork.SendIngress(event)

	if err != nil {
		if errors.Is(err, io.EOF) {
			// log a warning and continue for EOF errors
			lg.Err(err).Msg("could not pass through ingress event")
			return nil
		}
		// since this is used for testing, if we encounter any RPC send error, crash the orchestrator.
		lg.Error().Err(err).Msg("could not pass through ingress event")
		return err
	}
	lg.Info().Msg("ingress event passed through successfully")
	return nil
}
