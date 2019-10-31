// +build !relic

package crypto

// NewSigner chooses and initializes a signature scheme
func NewSigner(algo SigningAlgorithm) (signer, error) {
	return newNonRelicSigner(algo)
}
