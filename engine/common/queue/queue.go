package queue

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/fifoqueue"
)

type Event struct {
	OriginID flow.Identifier
	Msg      interface{}
}

type Handler struct {
	// return true if the event can be processed by the handler
	Match func(event interface{}) bool
	// process the matched event
	Process func(originID flow.Identifier, evt interface{})
}

type Sink struct {
	Handler *Handler
	Queue   *fifoqueue.FifoQueue
	Channel chan *Event
}

func ReceiveEvent(incomingEvents chan *Event, originID flow.Identifier, evt interface{}) {
	incomingEvents <- &Event{
		OriginID: originID,
		Msg:      evt,
	}
}

func popEvent(sinks []*Sink) (*Event, *Sink, bool) {
	for _, sink := range sinks {
		evt, ok := sink.Queue.Pop()
		if ok {
			return evt.(*Event), sink, true
		}
	}
	return nil, nil, false
}

func createSinks(handlers []*Handler) ([]*Sink, error) {
	sinks := make([]*Sink, len(handlers))
	for i, pipe := range handlers {
		queue, err := fifoqueue.NewFifoQueue()
		if err != nil {
			return nil, err
		}
		sinks[i] = &Sink{
			Handler: pipe,
			Queue:   queue,
			Channel: make(chan *Event),
		}
	}
	return sinks, nil
}

func ConsumeEvents(incomingEvents <-chan *Event, handlers []*Handler, exit <-chan struct{}, internalEvents <-chan struct{}, internalHandler func()) error {
	sinks, err := createSinks(handlers)
	if err != nil {
		return err
	}

	for {

		select {
		case evt := <-incomingEvents:
			// push incoming events to coresponding queue
			// or block and wait until there is an incoming event
			for _, sink := range sinks {
				matched := sink.Handler.Match(evt.Msg)
				if matched {
					break
				}
				sink.Queue.Push(evt)
			}
		case <-internalEvents:
			// when there is an internal event, call internal handler
			// useful for periodic function calls. i.e. check sealing
			internalHandler()
		case <-exit:
			return nil
		}

		// read from the coresponding queue
		evt, sink, ok := popEvent(sinks)
		if !ok {
			continue
		}

		sink.Handler.Process(evt.OriginID, evt.Msg)
	}
}
