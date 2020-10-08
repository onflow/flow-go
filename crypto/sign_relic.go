// +build relic

package crypto

import (
	"fmt"
)

// newSigner chooses and initializes a signature scheme
func newSigner(algo SigningAlgorithm) (signer, error) {
	if algo == BLSBLS12381 {
		return blsInstance, nil
	}
	return newNonRelicSigner(algo)
}

// Initialize Relic with the BLS context on BLS 12-381
func init() {
	blsInstance = &blsBLS12381Algo{
		algo: BLSBLS12381,
	}
	if err := blsInstance.init(); err != nil {
		panic(fmt.Sprintf("initialization of BLS has failed: %s", err.Error()))
	}

	initNonRelic()
}
