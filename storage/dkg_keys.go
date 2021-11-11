package storage

import (
	"github.com/onflow/flow-go/model/dkg"
)

type DKGKeys interface {
	// insert the random beacon private key to database when DKG didn't fail locally (the node completed DKG  and a private key was successfully generated)
	InsertMyDKGPrivateInfo(epochCounter uint64, key *dkg.DKGParticipantPriv) error

	// insert a record in database to indicate that the DKG was completed, 
	// but the node failed to properly participate, i.e. there is no valid private random beacon key.
	InsertNoDKGPrivateInfo(epochCounter uint64) error

	// look up the random beacon private key for the given Epoch, it returns:
	// - (key, true, nil) when DKG succeeded locally ( DKG completed locally, and a non-nil private key was saved)
	// - (nil, false, nil) when DKG completed locally, but no non-nil private key was saved
	// - (nil, false, storage.ErrNotFound) no private key is found in database
	// - (nil, false, error) for other exceptions
	RetrieveMyDKGPrivateInfo(epochCounter uint64) (*dkg.DKGParticipantPriv, bool, error)
}
