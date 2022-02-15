// +build relic

package crypto

import (
	"fmt"
)

// newSigner chooses and initializes a signature scheme
func newSigner(algo SigningAlgorithm) (signer, error) {
	// try Relic algos
	if signer := relicSigner(algo); signer != nil {
		return signer, nil
	}
	// return a non-Relic algo
	return newNonRelicSigner(algo)
}

// relicSigner returns a signer that depends on Relic library.
func relicSigner(algo SigningAlgorithm) signer {
	if algo == BLSBLS12381 {
		return blsInstance
	}
	return nil
}

// Initialize Relic with the BLS context on BLS 12-381
func init() {
	initRelic()
	initNonRelic()
}

// Initialize the context of all algos requiring Relic
func initRelic() {
	blsInstance = &blsBLS12381Algo{
		algo: BLSBLS12381,
	}
	if err := blsInstance.init(); err != nil {
		panic(fmt.Sprintf("initialization of BLS failed: %s", err.Error()))
	}
}
