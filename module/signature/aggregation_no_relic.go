//go:build !relic
// +build !relic

package signature

import (
	"github.com/onflow/flow-go/crypto"
)

const panic_relic = "function only supported with the relic build tag"

// These functions are the non-relic versions of some public functions from the package.
// The functions are here to allow the build of flow-emulator, since the emulator is built
// without the "relic" build tag, and does not run the functions below.
type SignatureAggregatorSameMessage struct{}

func NewSignatureAggregatorSameMessage(
	message []byte,
	dsTag string,
	publicKeys []crypto.PublicKey,
) (*SignatureAggregatorSameMessage, error) {
	panic(panic_relic)
}

func (s *SignatureAggregatorSameMessage) Verify(signer int, sig crypto.Signature) (bool, error) {
	panic(panic_relic)
}
func (s *SignatureAggregatorSameMessage) TrustedAdd(signer int, sig crypto.Signature) error {
	panic(panic_relic)
}

func (s *SignatureAggregatorSameMessage) Aggregate() ([]int, crypto.Signature, error) {
	panic(panic_relic)
}
