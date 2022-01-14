package synchronization

import (
	"fmt"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/storage"
	"github.com/rs/zerolog"
)

type RequestHandlerEngine struct {
	requestHandler *RequestHandler
}

var _ network.MessageProcessor = (*RequestHandlerEngine)(nil)

func NewRequestHandlerEngine(
	logger zerolog.Logger,
	metrics module.EngineMetrics,
	net network.Network,
	me module.Local,
	blocks storage.Blocks,
	core module.SyncCore,
	finalizedHeader *FinalizedHeaderCache,
) (*RequestHandlerEngine, error) {
	e := &RequestHandlerEngine{}

	con, err := net.Register(engine.PublicSyncCommittee, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}

	e.requestHandler = NewRequestHandler(
		logger,
		metrics,
		con.Unicast,
		me,
		blocks,
		core,
		finalizedHeader,
		false,
	)

	return e, nil
}

func (r *RequestHandlerEngine) Process(channel network.Channel, originID flow.Identifier, event interface{}) error {
	return r.requestHandler.Process(channel, originID, event)
}

func (r *RequestHandlerEngine) Ready() <-chan struct{} {
	return r.requestHandler.Ready()
}

func (r *RequestHandlerEngine) Done() <-chan struct{} {
	return r.requestHandler.Done()
}
