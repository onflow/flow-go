package synchronization

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
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
		NewResponseSender(con),
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
