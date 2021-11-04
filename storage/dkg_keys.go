package storage

import (
	"github.com/onflow/flow-go/model/dkg"
)

type DKGKeys interface {
	// insert the DKG private key to database when DKG was completed, and a DKG private key was successfully generated.
	InsertMyDKGPrivateInfo(epochCounter uint64, key *dkg.DKGParticipantPriv) error

	// insert a record in database to indicate that when DKG was completed, the node failed DKG, there is no DKG key generated.
	InsertNoDKGPrivateInfo(epochCounter uint64) error

	// receive the DKG key for the given Epoch, it returns:
	// - (key, true, nil) when DKG was completed, and a DKG private key was saved
	// - (nil, false, nil) when DKG was completed, but no DKG private key was saved
	// - (nil, false, storage.ErrNotFound) no DKG private key is found in database, it's unknown whether a DKG key was generated
	// - (nil, false, error) for other exceptions
	RetrieveMyDKGPrivateInfo(epochCounter uint64) (*dkg.DKGParticipantPriv, bool, error)
}
