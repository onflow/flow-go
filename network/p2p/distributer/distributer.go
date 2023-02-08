package distributer

import (
	"math/rand"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
)

type NotificationDispatcher struct {
	log     zerolog.Logger
	handler *engine.MessageHandler
	queue   engine.MessageStore
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
