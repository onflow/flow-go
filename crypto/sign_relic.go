// +build relic

package crypto

import "sync"

var BLS_BLS12381Instance *BLS_BLS12381Algo

//  Once variables to make sure each Signer is instantiated only once
var BLS_BLS12381Once sync.Once

// NewSigner chooses and initializes a signature scheme
func NewSigner(algo SigningAlgorithm) (signer, error) {
	if algo == BLS_BLS12381 {
		BLS_BLS12381Once.Do(func() {
			BLS_BLS12381Instance = &(BLS_BLS12381Algo{
				commonSigner: &commonSigner{
					algo,
					prKeyLengthBLS_BLS12381,
					pubKeyLengthBLS_BLS12381,
					signatureLengthBLS_BLS12381,
				},
			})
			BLS_BLS12381Instance.init()
		})
		return BLS_BLS12381Instance, nil
	}

	return newNonRelicSigner(algo)
}
