// +build relic

package verification

import "github.com/onflow/flow-go/crypto"

func AggregateBLSPublicKeys(keys []crypto.PublicKey) (crypto.PublicKey, error) {
	return crypto.AggregateBLSPublicKeys(keys)
}

func NeutralBLSPublicKey() crypto.PublicKey {
	return crypto.NeutralBLSPublicKey()
}

func RemoveBLSPublicKeys(aggKey crypto.PublicKey, keysToRemove []crypto.PublicKey) (crypto.PublicKey, error) {
	return crypto.RemoveBLSPublicKeys(aggKey, keysToRemove)
}
