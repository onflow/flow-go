package module

import (
	"errors"

	"github.com/onflow/flow-go/crypto"
)

var (
	// DKGFailError indicates that the node has completed DKG, but failed to generate private key
	// in the given epoch.
	DKGFailError = errors.New("dkg failed, no DKG private key generated")
)

// RandomBeaconKeyStore returns the random beacon private key for the given view,
type RandomBeaconKeyStore interface {
	// ByView returns the node's locally computed beacon private key for the epoch containing the given view.
	// It returns:
	//  - (key, nil) if the node has beacon keys in the epoch of the view
	//  - (nil, DKGFailError) if the node doesn't have beacon keys in the epoch of the view
	//  - (nil, error) if there is any exception
	ByView(view uint64) (crypto.PrivateKey, error)
}
