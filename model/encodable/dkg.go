package encodable

import (
	"github.com/onflow/flow-go/model/flow"
)

type DKG struct {
	Size         uint
	GroupKey     RandomBeaconPubKey
	Participants map[flow.Identifier]flow.DKGParticipant
}
