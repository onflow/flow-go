package synchronization

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/events"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type ResponseSender interface {
	SendResponse(interface{}, flow.Identifier) error
}

type ResponseSenderImpl struct {
	con network.Conduit
}

func (r *ResponseSenderImpl) SendResponse(res interface{}, target flow.Identifier) error {
	switch res.(type) {
	case *messages.BlockResponse:
		err := r.con.Unicast(res, target)
		if err != nil {
			return fmt.Errorf("could not unicast block response to target %x: %w", target, err)
		}
	case *messages.SyncResponse:
		err := r.con.Unicast(res, target)
		if err != nil {
			return fmt.Errorf("could not unicast sync response to target %x: %w", target, err)
		}
	default:
		return fmt.Errorf("unable to unicast unexpected response %+v", res)
	}

	return nil
}

func NewResponseSender(con network.Conduit) *ResponseSenderImpl {
	return &ResponseSenderImpl{
		con: con,
	}
}

// RequestHandlerEngine is an engine which operates only the request-handling portion of the block sync protocol.
// It is used by Access/Observer nodes attached to the public network, enabling them
// to provide block synchronization data to nodes on the public network, but not
// requesting any data from these nodes. (Requests are sent only on the private network.)
type RequestHandlerEngine struct {
	component.Component
	hotstuff.FinalizationConsumer

	requestHandler *RequestHandler
}

var _ network.MessageProcessor = (*RequestHandlerEngine)(nil)
var _ component.Component = (*RequestHandlerEngine)(nil)
var _ hotstuff.FinalizationConsumer = (*RequestHandlerEngine)(nil)

func NewRequestHandlerEngine(
	logger zerolog.Logger,
	metrics module.EngineMetrics,
	net network.EngineRegistry,
	me module.Local,
	state protocol.State,
	blocks storage.Blocks,
	core module.SyncCore,
) (*RequestHandlerEngine, error) {
	e := &RequestHandlerEngine{}

	con, err := net.Register(channels.PublicSyncCommittee, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}

	finalizedHeaderCache, finalizedCacheWorker, err := events.NewFinalizedHeaderCache(state)
	if err != nil {
		return nil, fmt.Errorf("could not initialize finalized header cache: %w", err)
	}
	e.FinalizationConsumer = finalizedHeaderCache
	e.requestHandler = NewRequestHandler(
		logger,
		metrics,
		NewResponseSender(con),
		me,
		finalizedHeaderCache,
		blocks,
		core,
		false,
	)
	builder := component.NewComponentManagerBuilder().AddWorker(finalizedCacheWorker)
	for i := 0; i < defaultEngineRequestsWorkers; i++ {
		builder.AddWorker(e.requestHandler.requestProcessingWorker)
	}
	e.Component = builder.Build()

	return e, nil
}

func (r *RequestHandlerEngine) Process(channel channels.Channel, originID flow.Identifier, event interface{}) error {
	return r.requestHandler.Process(channel, originID, event)
}
