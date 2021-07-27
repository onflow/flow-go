package storage

import (
	"github.com/onflow/flow-go/model/dkg"
)

type DKGKeys interface {
	InsertMyDKGPrivateInfo(epochCounter uint64, key *dkg.DKGParticipantPriv) error
	RetrieveMyDKGPrivateInfo(epochCounter uint64) (*dkg.DKGParticipantPriv, error)
}
