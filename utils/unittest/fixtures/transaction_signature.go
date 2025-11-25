package fixtures

import (
	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
)

// TransactionSignature is the default options factory for [flow.TransactionSignature] generation.
var TransactionSignature transactionSignatureFactory

type transactionSignatureFactory struct{}

type TransactionSignatureOption func(*TransactionSignatureGenerator, *flow.TransactionSignature)

// WithAddress is an option that sets the address for the transaction signature.
func (f transactionSignatureFactory) WithAddress(address flow.Address) TransactionSignatureOption {
	return func(g *TransactionSignatureGenerator, signature *flow.TransactionSignature) {
		signature.Address = address
	}
}

// WithSignerIndex is an option that sets the signer index for the transaction signature.
func (f transactionSignatureFactory) WithSignerIndex(signerIndex int) TransactionSignatureOption {
	return func(g *TransactionSignatureGenerator, signature *flow.TransactionSignature) {
		signature.SignerIndex = signerIndex
	}
}

// WithSignature is an option that sets the signature for the transaction signature.
func (f transactionSignatureFactory) WithSignature(sig crypto.Signature) TransactionSignatureOption {
	return func(g *TransactionSignatureGenerator, signature *flow.TransactionSignature) {
		signature.Signature = sig
	}
}

// WithKeyIndex is an option that sets the key index for the transaction signature.
func (f transactionSignatureFactory) WithKeyIndex(keyIndex uint32) TransactionSignatureOption {
	return func(g *TransactionSignatureGenerator, signature *flow.TransactionSignature) {
		signature.KeyIndex = keyIndex
	}
}

// TransactionSignatureGenerator generates transaction signatures with consistent randomness.
type TransactionSignatureGenerator struct {
	transactionSignatureFactory

	random    *RandomGenerator
	addresses *AddressGenerator
}

func NewTransactionSignatureGenerator(
	random *RandomGenerator,
	addresses *AddressGenerator,
) *TransactionSignatureGenerator {
	return &TransactionSignatureGenerator{
		random:    random,
		addresses: addresses,
	}
}

// Fixture generates a [flow.TransactionSignature] with random data based on the provided options.
func (g *TransactionSignatureGenerator) Fixture(opts ...TransactionSignatureOption) flow.TransactionSignature {
	signature := flow.TransactionSignature{
		Address:     g.addresses.Fixture(),
		SignerIndex: 0,
		Signature:   g.generateValidSignature(),
		KeyIndex:    1,
	}

	for _, opt := range opts {
		opt(g, &signature)
	}

	return signature
}

// List generates a list of [flow.TransactionSignature].
func (g *TransactionSignatureGenerator) List(n int, opts ...TransactionSignatureOption) []flow.TransactionSignature {
	list := make([]flow.TransactionSignature, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}

// generateValidSignature generates a valid ECDSA signature for the given transaction.
func (g *TransactionSignatureGenerator) generateValidSignature() crypto.Signature {
	sigLen := crypto.SignatureLenECDSAP256
	signature := g.random.RandomBytes(sigLen)

	// Make sure the ECDSA signature passes the format check
	signature[sigLen/2] = 0
	signature[0] = 0
	signature[sigLen/2-1] |= 1
	signature[sigLen-1] |= 1

	return signature
}
