package fixtures

import (
	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
)

// TransactionSignatureGenerator generates transaction signatures with consistent randomness.
type TransactionSignatureGenerator struct {
	randomGen  *RandomGenerator
	addressGen *AddressGenerator
}

func NewTransactionSignatureGenerator(
	randomGen *RandomGenerator,
	addressGen *AddressGenerator,
) *TransactionSignatureGenerator {
	return &TransactionSignatureGenerator{
		randomGen:  randomGen,
		addressGen: addressGen,
	}
}

// WithAddress is an option that sets the address for the transaction signature.
func (g *TransactionSignatureGenerator) WithAddress(address flow.Address) func(*flow.TransactionSignature) {
	return func(signature *flow.TransactionSignature) {
		signature.Address = address
	}
}

// WithSignerIndex is an option that sets the signer index for the transaction signature.
func (g *TransactionSignatureGenerator) WithSignerIndex(signerIndex int) func(*flow.TransactionSignature) {
	return func(signature *flow.TransactionSignature) {
		signature.SignerIndex = signerIndex
	}
}

// WithSignature is an option that sets the signature for the transaction signature.
func (g *TransactionSignatureGenerator) WithSignature(signature crypto.Signature) func(*flow.TransactionSignature) {
	return func(signature *flow.TransactionSignature) {
		signature.Signature = signature.Signature
	}
}

// WithKeyIndex is an option that sets the key index for the transaction signature.
func (g *TransactionSignatureGenerator) WithKeyIndex(keyIndex uint32) func(*flow.TransactionSignature) {
	return func(signature *flow.TransactionSignature) {
		signature.KeyIndex = keyIndex
	}
}

// Fixture generates a [flow.TransactionSignature] with random data based on the provided options.
func (g *TransactionSignatureGenerator) Fixture(opts ...func(*flow.TransactionSignature)) flow.TransactionSignature {
	signature := flow.TransactionSignature{
		Address:     g.addressGen.Fixture(),
		SignerIndex: 0,
		Signature:   g.generateValidSignature(),
		KeyIndex:    1,
	}

	for _, opt := range opts {
		opt(&signature)
	}

	return signature
}

// List generates a list of [flow.TransactionSignature].
func (g *TransactionSignatureGenerator) List(n int, opts ...func(*flow.TransactionSignature)) []flow.TransactionSignature {
	list := make([]flow.TransactionSignature, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}

// generateValidSignature generates a valid ECDSA signature for the given transaction.
func (g *TransactionSignatureGenerator) generateValidSignature() crypto.Signature {
	sigLen := crypto.SignatureLenECDSAP256
	signature := g.randomGen.RandomBytes(sigLen)

	// Make sure the ECDSA signature passes the format check
	signature[sigLen/2] = 0
	signature[0] = 0
	signature[sigLen/2-1] |= 1
	signature[sigLen-1] |= 1

	return signature
}
