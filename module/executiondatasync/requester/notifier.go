package requester

import (
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
	"github.com/rs/zerolog"
)

type notificationSub struct {
	consumer BlockExecutionDataConsumer
}

type notification struct {
	blockHeight   uint64
	executionData *execution_data.BlockExecutionData
}

type subRequest struct {
	sub  *notificationSub
	done chan struct{}
}

type notifier struct {
	notificationsIn  chan<- interface{}
	notificationsOut <-chan interface{}

	subs   chan *subRequest
	unsubs chan *subRequest

	subscriptions map[*notificationSub]struct{}

	logger zerolog.Logger

	cm *component.ComponentManager
	component.Component
}

func newNotifier(logger zerolog.Logger) *notifier {
	notificationsIn, notificationsOut := util.UnboundedChannel()
	n := &notifier{
		notificationsIn:  notificationsIn,
		notificationsOut: notificationsOut,
		subs:             make(chan *subRequest),
		unsubs:           make(chan *subRequest),
		subscriptions:    make(map[*notificationSub]struct{}),
		logger:           logger.With().Str("subcomponent", "notifier").Logger(),
	}

	n.cm = component.NewComponentManagerBuilder().
		AddWorker(n.loop).
		Build()
	n.Component = n.cm

	return n
}

func (n *notifier) notify(blockHeight uint64, executionData *execution_data.BlockExecutionData) {
	n.notificationsIn <- &notification{blockHeight, executionData}
}

func (n *notifier) subscribe(sub *notificationSub) (func() error, error) {
	done := make(chan struct{})
	select {
	case n.subs <- &subRequest{sub, done}:
		select {
		case <-done:
			return n.unsubFunc(sub), nil
		case <-n.cm.ShutdownSignal():
			return nil, component.ErrComponentShutdown
		}
	case <-n.cm.ShutdownSignal():
		return nil, component.ErrComponentShutdown
	}
}

func (n *notifier) unsubFunc(sub *notificationSub) func() error {
	return func() error {
		done := make(chan struct{})
		select {
		case n.unsubs <- &subRequest{sub, done}:
			select {
			case <-done:
				return nil
			case <-n.cm.ShutdownSignal():
				return component.ErrComponentShutdown
			}
		case <-n.cm.ShutdownSignal():
			return component.ErrComponentShutdown
		}
	}
}

func (n *notifier) loop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case sub := <-n.subs:
			n.subscriptions[sub.sub] = struct{}{}
			close(sub.done)
		case sub := <-n.unsubs:
			delete(n.subscriptions, sub.sub)
			close(sub.done)
		case notif := <-n.notificationsOut:
			notification := notif.(*notification)
			n.logger.Debug().
				Uint64("block_height", notification.blockHeight).
				Str("block_id", notification.executionData.BlockID.String()).
				Msg("sending notifications")
			for sub := range n.subscriptions {
				sub.consumer(notification.blockHeight, notification.executionData)
			}
		}
	}
}
