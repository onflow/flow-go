package consensus

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
)

func NewParticipant(log zerolog.Logger) (*hotstuff.EventLoop, error) {

	// initialize and return the event loop
	loop, err := hotstuff.NewEventLoop(log)
	if err != nil {
		return nil, fmt.Errorf("could not initialize event loop: %w", err)
	}

	return loop, nil
}
