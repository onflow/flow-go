package libp2p

import (
	"context"
	"sync"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/network/gossip/libp2p/message"
)

type ReadSubscription struct {
	log     zerolog.Logger
	sub     *pubsub.Subscription
	inbound chan *message.Message
	once    *sync.Once
	done    chan struct{}
}

// NewReadSubscription reads the messages coming in on the subscription
// TODO: Make read subscription, read connection and write connection implement a common interface Collection
func NewReadSubscription(log zerolog.Logger, sub *pubsub.Subscription) *ReadSubscription {

	log = log.With().
		Str("channelid", sub.Topic()).
		Logger()

	c := ReadSubscription{
		log:     log,
		sub:     sub,
		inbound: make(chan *message.Message),
		once:    &sync.Once{},
		done:    make(chan struct{}),
	}

	return &c
}

// Stop will Stop by closing the done channel and closing the connection.
func (r *ReadSubscription) stop() {
	r.once.Do(func() {
		close(r.done)
		r.sub.Cancel()
	})
}

// recv must be run in a goroutine and takes care of continuously receiving
// messages from the peer connection until the connection fails.
func (r *ReadSubscription) ReceiveLoop() {
	defer r.stop()
	// close and drain the inbound channel
	defer close(r.inbound)

	c := context.Background()

RecvLoop:
	for {
		// check if we should stop
		select {
		case <-r.done:
			r.log.Debug().Msg("exiting receive routine")
			break RecvLoop
		default:
		}
		var msg message.Message

		rawMsg, err := r.sub.Next(c)
		if err != nil {
			r.log.Err(err).Msg("failed to read subscription message")
			break RecvLoop
		}

		r.log.Debug().Str("topic_message", msg.String()).Msg("received message")

		err = msg.Unmarshal(rawMsg.Data)
		if err != nil {
			r.log.Err(err).Str("topic_message", msg.String()).Msg("failed to unmarshal message")
			break RecvLoop
		}

		r.log.Debug().
			Bytes("sender", msg.OriginID).
			Hex("eventID", msg.EventID).
			Msg("received message")

		// stash the received message into the inbound queue for handling
		r.inbound <- &msg
	}
}
