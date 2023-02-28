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

// RandomBeaconKeyStore provides access to the node's locally computed random beacon for a given epoch.
// We determine which epoch to use based on the view, each view belongs to exactly one epoch.
// Beacon keys are only returned once:
//   - the DKG has completed successfully locally AND
//   - the DKG has completed successfully globally (EpochCommit event sealed) with a consistent result
//
// Therefore keys returned by this module are guaranteed to be safe for use.
type RandomBeaconKeyStore interface {
	// ByView returns the node's locally computed beacon private key for the epoch containing the given view.
	// It returns:
	//  - (key, nil) if the node has beacon keys in the epoch of the view
	//  - (nil, DKGFailError) if the node doesn't have beacon keys in the epoch of the view
	//  - (nil, error) if there is any exception
	ByView(view uint64) (crypto.PrivateKey, error)
}
