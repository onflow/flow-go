package synchronization

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
)

// defaultSyncRequestQueueCapacity maximum capacity of sync requests queue
const defaultSyncRequestQueueCapacity = 500

// defaultSyncRequestQueueCapacity maximum capacity of range requests queue
const defaultRangeRequestQueueCapacity = 500

// defaultSyncRequestQueueCapacity maximum capacity of batch requests queue
const defaultBatchRequestQueueCapacity = 500

// defaultEngineRequestsWorkers number of workers to dispatch events for requests
const defaultEngineRequestsWorkers = 8

type RequestHandlerEngine struct {
	unit *engine.Unit
	lm   *lifecycle.LifecycleManager

	me      module.Local
	log     zerolog.Logger
	metrics SyncEngineMetrics

	core            RequestHandlerCore
	finalizedHeader *FinalizedHeaderCache
	con             network.Conduit // used for sending responses to requesters

	pendingSyncRequests   engine.MessageStore    // message store for *message.SyncRequest
	pendingBatchRequests  engine.MessageStore    // message store for *message.BatchRequest
	pendingRangeRequests  engine.MessageStore    // message store for *message.RangeRequest
	requestMessageHandler *engine.MessageHandler // message handler responsible for request processing
}

func NewRequestHandlerEngine(
	log zerolog.Logger,
	metrics SyncEngineMetrics,
	con network.Conduit,
	me module.Local,
	core RequestHandlerCore,
	finalizedHeader *FinalizedHeaderCache,
) *RequestHandlerEngine {
	r := &RequestHandlerEngine{
		unit:            engine.NewUnit(),
		lm:              lifecycle.NewLifecycleManager(),
		me:              me,
		log:             log.With().Str("engine", "synchronization").Logger(),
		metrics:         metrics,
		core:            core,
		finalizedHeader: finalizedHeader,
		con:             con,
	}

	r.setupRequestMessageHandler()

	return r
}

// SubmitLocal submits an event originating on the local node.
func (r *RequestHandlerEngine) SubmitLocal(event interface{}) {
	err := r.process(r.me.NodeID(), event)
	if err != nil {
		// receiving an input of incompatible type from a trusted internal component is fatal
		r.log.Fatal().Err(err).Msg("internal error processing event")
	}
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (r *RequestHandlerEngine) Submit(channel network.Channel, originID flow.Identifier, event interface{}) {
	err := r.process(originID, event)
	if err != nil {
		lg := r.log.With().
			Err(err).
			Str("channel", channel.String()).
			Str("origin", originID.String()).
			Logger()
		if errors.Is(err, engine.IncompatibleInputTypeError) {
			lg.Error().Msg("received message with incompatible type")
			return
		}
		lg.Fatal().Msg("internal error processing message")
	}
}

// ProcessLocal processes an event originating on the local node.
func (r *RequestHandlerEngine) ProcessLocal(event interface{}) error {
	return r.process(r.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (r *RequestHandlerEngine) Process(channel network.Channel, originID flow.Identifier, event interface{}) error {
	return r.process(originID, event)
}

// process processes events for the synchronization request handler engine.
// Error returns:
//  * IncompatibleInputTypeError if input has unexpected type
//  * All other errors are potential symptoms of internal state corruption or bugs (fatal).
func (r *RequestHandlerEngine) process(originID flow.Identifier, event interface{}) error {
	switch event.(type) {
	case *messages.RangeRequest, *messages.BatchRequest, *messages.SyncRequest:
		return r.requestMessageHandler.Process(originID, event)
	default:
		return fmt.Errorf("received input with type %T from %x: %w", event, originID[:], engine.IncompatibleInputTypeError)
	}
}

// setupRequestMessageHandler initializes the inbound queues and the MessageHandler for UNTRUSTED requests.
func (r *RequestHandlerEngine) setupRequestMessageHandler() {
	// RequestHeap deduplicates requests by keeping only one sync request for each requester.
	r.pendingSyncRequests = NewRequestHeap(defaultSyncRequestQueueCapacity)
	r.pendingRangeRequests = NewRequestHeap(defaultRangeRequestQueueCapacity)
	r.pendingBatchRequests = NewRequestHeap(defaultBatchRequestQueueCapacity)

	// define message queueing behaviour
	r.requestMessageHandler = engine.NewMessageHandler(
		r.log,
		engine.NewNotifier(),
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.SyncRequest)
				if ok {
					r.metrics.MessageReceived(metrics.MessageSyncRequest)
				}
				return ok
			},
			Store: r.pendingSyncRequests,
		},
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.RangeRequest)
				if ok {
					r.metrics.MessageReceived(metrics.MessageRangeRequest)
				}
				return ok
			},
			Store: r.pendingRangeRequests,
		},
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.BatchRequest)
				if ok {
					r.metrics.MessageReceived(metrics.MessageBatchRequest)
				}
				return ok
			},
			Store: r.pendingBatchRequests,
		},
	)
}

// onSyncRequest processes an outgoing handshake; if we have a higher height, we
// inform the other node of it, so they can organize their block downloads. If
// we have a lower height, we add the difference to our own download queue.
func (r *RequestHandlerEngine) onSyncRequest(originID flow.Identifier, req *messages.SyncRequest) error {

	res, err := r.core.HandleSyncRequest(req, r.finalizedHeader.Get())
	if err != nil {
		return err
	}

	if res != nil {
		err = r.con.Unicast(res, originID)
		if err != nil {
			r.log.Warn().Err(err).Msg("sending sync response failed")
			return nil
		}
		r.metrics.MessageSent(metrics.MessageSyncResponse)
	}

	return nil
}

// onRangeRequest processes a request for a range of blocks by height.
func (r *RequestHandlerEngine) onRangeRequest(originID flow.Identifier, req *messages.RangeRequest) error {

	// get the latest final state to know if we can fulfill the request
	res, err := r.core.HandleRangeRequest(req, r.finalizedHeader.Get())
	if err != nil {
		return err
	}

	if res != nil {
		err = r.con.Unicast(res, originID)
		if err != nil {
			r.log.Warn().Err(err).Hex("origin_id", originID[:]).Msg("sending range response failed")
			return nil
		}
		r.metrics.MessageSent(metrics.MessageBlockResponse)
	}

	return nil
}

// onBatchRequest processes a request for a specific block by block ID.
func (r *RequestHandlerEngine) onBatchRequest(originID flow.Identifier, req *messages.BatchRequest) error {
	// we should bail and send nothing on empty request
	if len(req.BlockIDs) == 0 {
		return nil
	}

	res, err := r.core.HandleBatchRequest(req)
	if err != nil {
		return err
	}

	if res != nil {
		err = r.con.Unicast(res, originID)
		if err != nil {
			r.log.Warn().Err(err).Hex("origin_id", originID[:]).Msg("sending batch response failed")
			return nil
		}
		r.metrics.MessageSent(metrics.MessageBlockResponse)
	}

	return nil
}

// processAvailableRequests is processor of pending events which drives events from networking layer to business logic.
func (r *RequestHandlerEngine) processAvailableRequests() error {
	for {
		select {
		case <-r.unit.Quit():
			return nil
		default:
		}

		msg, ok := r.pendingSyncRequests.Get()
		if ok {
			err := r.onSyncRequest(msg.OriginID, msg.Payload.(*messages.SyncRequest))
			if err != nil {
				return fmt.Errorf("processing sync request failed: %w", err)
			}
			continue
		}

		msg, ok = r.pendingRangeRequests.Get()
		if ok {
			err := r.onRangeRequest(msg.OriginID, msg.Payload.(*messages.RangeRequest))
			if err != nil {
				return fmt.Errorf("processing range request failed: %w", err)
			}
			continue
		}

		msg, ok = r.pendingBatchRequests.Get()
		if ok {
			err := r.onBatchRequest(msg.OriginID, msg.Payload.(*messages.BatchRequest))
			if err != nil {
				return fmt.Errorf("processing batch request failed: %w", err)
			}
			continue
		}

		// when there is no more messages in the queue, back to the loop to wait
		// for the next incoming message to arrive.
		return nil
	}
}

// requestProcessingLoop is a separate goroutine that performs processing of queued requests
func (r *RequestHandlerEngine) requestProcessingLoop() {
	notifier := r.requestMessageHandler.GetNotifier()
	for {
		select {
		case <-r.unit.Quit():
			return
		case <-notifier:
			err := r.processAvailableRequests()
			if err != nil {
				r.log.Fatal().Err(err).Msg("internal error processing queued requests")
			}
		}
	}
}

// Ready returns a ready channel that is closed once the engine has fully started.
func (r *RequestHandlerEngine) Ready() <-chan struct{} {
	r.lm.OnStart(func() {
		<-r.finalizedHeader.Ready()
		for i := 0; i < defaultEngineRequestsWorkers; i++ {
			r.unit.Launch(r.requestProcessingLoop)
		}
	})
	return r.lm.Started()
}

// Done returns a done channel that is closed once the engine has fully stopped.
func (r *RequestHandlerEngine) Done() <-chan struct{} {
	r.lm.OnStop(func() {
		// wait for all request processing workers to exit
		<-r.unit.Done()
		<-r.finalizedHeader.Done()
	})
	return r.lm.Stopped()
}
