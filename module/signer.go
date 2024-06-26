package module

import (
	"errors"

	"github.com/onflow/crypto"
)

var (
	// ErrNoBeaconKeyForEpoch indicates that no beacon key is available for the given epoch.
	// This can happen for several reasons:
	//   1. The DKG for the epoch has not completed yet, and the beacon key may be available later.
	//   2. The DKG succeeded globally, but this node failed to generate a local beacon key.
	//      This can happen if the node is unavailable or behind during the DKG.
	//      In this case, no beacon key will ever be available for this epoch.
	//   3. The DKG failed globally, so no nodes generated a local beacon key.
	//
	// Regardless of the reason, beacon key users should fall back to signing with
	// only their staking key - hence these situations are not differentiated.
	ErrNoBeaconKeyForEpoch = errors.New("no beacon key available for epoch")
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
	//   - (key, nil) if the node has beacon keys in the epoch of the view
	//   - (nil, model.ErrViewForUnknownEpoch) if no epoch found for given view
	//   - (nil, module.ErrNoBeaconKeyForEpoch) if beacon key for epoch is unavailable
	//   - (nil, error) if there is any exception
	ByView(view uint64) (crypto.PrivateKey, error)
}
