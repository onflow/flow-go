package validator

import (
	"bytes"
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/message"
)

var _ network.MessageValidator = &SenderValidator{}

// SenderValidator validates messages by sender ID
type SenderValidator struct {
	sender []byte
}

// ValidateSender creates and returns a new SenderValidator for the given sender ID
func ValidateSender(sender flow.Identifier) network.MessageValidator {
	sv := &SenderValidator{}
	sv.sender = sender[:]
	return sv
}

// Validate returns true if the message origin id is the same as the sender ID.
func (sv *SenderValidator) Validate(msg message.Message) bool {
	return bytes.Equal(sv.sender, msg.OriginID)
}

// ValidateNotSender creates and returns a validator which validates that the message origin id is different from
// sender id
func ValidateNotSender(sender flow.Identifier) network.MessageValidator {
	return NewNotValidator(ValidateSender(sender))
}

func (v *StakedValidator) Validate(ctx context.Context, receivedFrom peer.ID, rawMsg *pubsub.Message) pubsub.ValidationResult {
	var msg message.Message
	// convert the incoming raw message payload to Message type
	//bs := binstat.EnterTimeVal(binstat.BinNet+":wire>1protobuf2message", int64(len(rawMsg.Data)))
	err := msg.Unmarshal(rawMsg.Data)
	//binstat.Leave(bs)
	if err != nil {
		r.log.Err(err).Str("topic_message", msg.String()).Msg("failed to unmarshal message")
		return pubsub.ValidationReject
	}

	from, err := messageSigningID(rawMsg)

	if err != nil {
		return pubsub.ValidationReject
	}

	// check that message contains a valid sender ID
	if from, err := messageSigningID(msg); err == nil {
		// check that the sender peer ID matches a staked Flow key
		if _, exists := v.getIdentity(from); exists {
			return pubsub.ValidationAccept
		}
	}

}
