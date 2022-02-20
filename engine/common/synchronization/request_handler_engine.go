package synchronization

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/finalized_cache"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/storage"
)

type RequestHandlerEngine struct {
	requestHandler *RequestHandler
	con            network.Conduit
	requests       chan *Request
	cm             *component.ComponentManager
	component.Component
}

type Request struct {
	OriginID flow.Identifier
	Payload  interface{}
}

var _ network.MessageProcessor = (*RequestHandlerEngine)(nil)

const defaultRequestQueueSize = 1000
const defaultNumQueueWorkers = 8

func NewRequestHandlerEngine(
	logger zerolog.Logger,
	metrics module.SyncMetrics,
	net network.Network,
	me module.Local,
	blocks storage.Blocks,
	core module.SyncCore,
	finalizedHeader *finalized_cache.FinalizedHeaderCache,
	config *HandlerConfig,
) (*RequestHandlerEngine, error) {
	e := &RequestHandlerEngine{}

	con, err := net.Register(engine.PublicSyncCommittee, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}

	requestHandler := NewRequestHandler(
		blocks,
		logger,
		finalizedHeader,
		metrics,
		core,
		config,
	)

	e.requestHandler = requestHandler
	e.con = con
	e.requests = make(chan *Request, defaultRequestQueueSize)

	builder := component.NewComponentManagerBuilder()

	for i := 0; i < defaultNumQueueWorkers; i++ {
		builder.AddWorker(e.loop)
	}

	e.cm = builder.Build()
	e.Component = e.cm

	return e, nil
}

func (r *RequestHandlerEngine) cleanup(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	<-r.requestHandler.finalizedHeader.Ready()

	ready()

	<-ctx.Done()

	err := r.con.Close()

	if err != nil {
		ctx.Throw(fmt.Errorf("could not close conduit: %w", err))
	}

	// TODO: is this safe?
	r.requests = nil
}

func (r *RequestHandlerEngine) loop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case req := <-r.requests:
			r.processRequest(ctx, req)
		}
	}
}

func (r *RequestHandlerEngine) processRequest(ctx irrecoverable.SignalerContext, req *Request) {
	response, err := r.requestHandler.HandleRequest(req.Payload, req.OriginID)
	if err != nil {
		// log error
		return
	}

	err = r.con.Unicast(response, req.OriginID)

	if err != nil {
		// log error
		return
	}

	// TODO: log success

}

func (r *RequestHandlerEngine) Process(channel network.Channel, originID flow.Identifier, event interface{}) error {
	select {
	case <-r.cm.ShutdownSignal():
		// TODO:
		return fmt.Errorf("shutdown signal received")
	default:
	}

	switch event.(type) {
	case *messages.SyncRequest:
	case *messages.RangeRequest:
	case *messages.BatchRequest:
	default:
		// TODO: wrong type
		return fmt.Errorf("TODO")
	}

	select {
	case r.requests <- &Request{
		OriginID: originID,
		Payload:  event,
	}:
		return nil
	default:
		// TODO: queue is full
		return fmt.Errorf("TODO")
	}
}
