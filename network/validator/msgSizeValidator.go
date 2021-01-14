package validator

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/message"
)

var _ network.MessageValidator = &MsgSizeValidator{}

// maxMsgSizeLimitLookupFunc a function to lookup the maximum permissible size for a given message
type maxMsgSizeLimitLookupFunc func(msg *message.Message) int

// MsgSizeValidator validates the incoming unicast message size
type MsgSizeValidator struct {
	log        zerolog.Logger
	lookupFunc maxMsgSizeLimitLookupFunc
}

// NewMsgSizeValidator creates and returns a new MsgSizeValidator
func NewMsgSizeValidator(log zerolog.Logger, lookupFunc maxMsgSizeLimitLookupFunc) *MsgSizeValidator {
	msgSizeValidator := &MsgSizeValidator{
		log:        log,
		lookupFunc: lookupFunc,
	}
	return msgSizeValidator
}

// Validate returns true if the message size is less than or equal to the maximum permissible message size
func (msv *MsgSizeValidator) Validate(msg message.Message) bool {
	maxSize := msv.lookupFunc(&msg)
	if msg.Size() <= maxSize {
		return true
	}

	msv.log.Error().
		Hex("sender", msg.OriginID).
		Hex("event_id", msg.EventID).
		Str("event_type", msg.Type).
		Str("channel_id", msg.ChannelID).
		Int("maxSize", maxSize).
		Msg("received message exceeded permissible message maxSize")

	return false
}
