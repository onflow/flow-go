package dkg

import (
	"github.com/onflow/crypto"
)

// ThresholdKeySet represents all the output data from the KG process needed for a threshold signature scheme that
// is used in random beacon protocol, including private information.
// Typically, the ThresholdKeySet is used with a trusted setup during bootstrapping.
type ThresholdKeySet struct {
	PrivKeyShares []crypto.PrivateKey
	PubGroupKey   crypto.PublicKey
	PubKeyShares  []crypto.PublicKey
}
