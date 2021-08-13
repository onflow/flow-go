// +build !relic

package verification

import "github.com/onflow/flow-go/crypto"

func AggregateBLSPublicKeys(_ []crypto.PublicKey) (crypto.PublicKey, error) {
	panic("AggregateBLSPublicKeys not supported with non-relic build")
}

func NeutralBLSPublicKey() crypto.PublicKey {
	panic("NeutralBLSPublicKey not supported with non-relic build")
}

func RemoveBLSPublicKeys(_ crypto.PublicKey, _ []crypto.PublicKey) (crypto.PublicKey, error) {
	panic("RemoveBLSPublicKeys not supported with non-relic build")
}
