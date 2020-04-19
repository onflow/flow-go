package validators

import (
	"bytes"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/message"
)

var _ MessageValidator = &TargetValidator{}

// TargetValidator filters out messages by target ID
type TargetValidator struct {
	target []byte
	log    zerolog.Logger
}

// NewTargetValidator returns a new TargetValidator for the given target id
func NewTargetValidator(log zerolog.Logger, target flow.Identifier) *TargetValidator {
	tv := &TargetValidator{
		target: target[:],
		log:    log,
	}
	return tv
}

// Validate returns true if the message is intended for the given target ID else it returns false
func (tv *TargetValidator) Validate(msg message.Message) bool {
	for _, t := range msg.TargetIDs {
		if bytes.Equal(tv.target, t) {
			fmt.Printf("Target Valid %x | %x\n", tv.target, t)
			return true
		}
		fmt.Printf("Target Checked %x | %x\n", tv.target, t)
	}
	fmt.Printf("Target not valid %x\n", tv.target)
	tv.log.Debug().Hex("target", tv.target).Hex("event_id", msg.EventID).Msg("message not intended for target")
	return false
}
