package distributer

import (
	"fmt"
	"math/rand"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type NotificationDispatcher struct {
	log                 zerolog.Logger
	handler             *engine.MessageHandler
	queue               engine.MessageStore
	notificationChannel chan interface{}
}

type Notification struct {
	Content interface{}
	Nonce   uint64
}

func NewNotificationDispatcher(log zerolog.Logger, queue engine.MessageStore) *NotificationDispatcher {
	return &NotificationDispatcher{
		log:   log.With().Str("component", "notification_dispatcher").Logger(),
		queue: queue,
		handler: engine.NewMessageHandler(log, engine.NewNotifier(), engine.Pattern{
			Map: func(message *engine.Message) (*engine.Message, bool) {
				return &engine.Message{
					Payload: Notification{
						Content: message.Payload,
						Nonce:   rand.Uint64(),
					},
				}, true
			},
			Store: queue,
		}),
	}
}

// processNotificationShovlerWorker is constantly listening on the MessageHandler for new notifications,
// and pushes new notifications into the request channel to be picked by workers.
func (n *NotificationDispatcher) processNotificationShovlerWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	n.log.Debug().Msg("process notification shovler started")

	for {
		select {
		case <-n.handler.GetNotifier():
			// there is at least a single notification to process.
			n.processAvailableNotifications(ctx)
		case <-ctx.Done():
			// close the internal channel, the workers will drain the channel before exiting
			close(n.notificationChannel)
			n.log.Debug().Msg("processing notification worker terminated")
			return
		}
	}
}

// processAvailableNotifications processes all available notifications in the queue.
func (n *NotificationDispatcher) processAvailableNotifications(ctx irrecoverable.SignalerContext) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msg, ok := n.queue.Get()
		if !ok {
			// no more notifications to process
			return
		}

		notification, ok := msg.Payload.(Notification)
		if !ok {
			// should never happen.
			// if it does happen, it means there is a bug in the queue implementation.
			ctx.Throw(fmt.Errorf("invalid notification type in queue: %T", msg.Payload))
		}

		n.log.Trace().Msg("shovler is queuing notification for processing")
		n.notificationChannel <- notification.Content
		n.log.Trace().Msg("shovler queued up notification for processing")
	}
}
