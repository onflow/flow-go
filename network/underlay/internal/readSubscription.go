package internal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
	validator "github.com/onflow/flow-go/network/validator/pubsub"
	"github.com/onflow/flow-go/utils/logging"
)

// ReadSubscriptionCallBackFunction the callback called when a new message is received on the read subscription
type ReadSubscriptionCallBackFunction func(msg *message.Message, peerID peer.ID)

// ReadSubscription reads the messages coming in on the subscription and calls the given callback until
// the context of the subscription is cancelled.
type ReadSubscription struct {
	log      zerolog.Logger
	sub      p2p.Subscription
	callback ReadSubscriptionCallBackFunction
}

// NewReadSubscription reads the messages coming in on the subscription
func NewReadSubscription(sub p2p.Subscription, callback ReadSubscriptionCallBackFunction, log zerolog.Logger) *ReadSubscription {
	r := ReadSubscription{
		log:      log.With().Str("channel", sub.Topic()).Logger(),
		sub:      sub,
		callback: callback,
	}

	return &r
}

// ReceiveLoop must be run in a goroutine. It continuously receives
// messages for the topic and calls the callback synchronously
func (r *ReadSubscription) ReceiveLoop(ctx context.Context) {
	defer r.log.Debug().Msg("exiting receive routine")

	for {
		// read the next message from libp2p's subscription (blocking call)
		rawMsg, err := r.sub.Next(ctx)

		if err != nil {
			// network may have cancelled the context
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}

			// subscription may have just been cancelled if node is being stopped or the topic has been unsubscribed,
			// don't log error in that case
			// (https://github.com/ipsn/go-ipfs/blob/master/gxlibs/github.com/libp2p/go-libp2p-pubsub/pubsub.go#L435)
			if strings.Contains(err.Error(), "subscription cancelled") {
				return
			}

			// log any other error
			r.log.Err(err).Msg("failed to read subscription message")

			return
		}

		validatorData, ok := rawMsg.ValidatorData.(validator.TopicValidatorData)
		if !ok {
			r.log.Error().
				Str("raw_msg", rawMsg.String()).
				Bool(logging.KeySuspicious, true).
				Str("received_validator_data_type", fmt.Sprintf("%T", rawMsg.ValidatorData)).
				Msg("[BUG] validator data missing!")
			return
		}

		// call the callback
		r.callback(validatorData.Message, validatorData.From)
	}
}
