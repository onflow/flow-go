package fixtures

import (
	"testing"

	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
)

// TransactionSignatureGenerator generates transaction signatures with consistent randomness.
type TransactionSignatureGenerator struct {
	randomGen  *RandomGenerator
	addressGen *AddressGenerator
}

// transactionSignatureConfig holds the configuration for transaction signature generation.
type transactionSignatureConfig struct {
	address     flow.Address
	signerIndex int
	signature   crypto.Signature
	keyIndex    uint32
}

// WithAddress returns an option to set the address for the transaction signature.
func (g *TransactionSignatureGenerator) WithAddress(address flow.Address) func(*transactionSignatureConfig) {
	return func(config *transactionSignatureConfig) {
		config.address = address
	}
}

// WithSignerIndex returns an option to set the signer index for the transaction signature.
func (g *TransactionSignatureGenerator) WithSignerIndex(signerIndex int) func(*transactionSignatureConfig) {
	return func(config *transactionSignatureConfig) {
		config.signerIndex = signerIndex
	}
}

// WithSignature returns an option to set the signature for the transaction signature.
func (g *TransactionSignatureGenerator) WithSignature(signature crypto.Signature) func(*transactionSignatureConfig) {
	return func(config *transactionSignatureConfig) {
		config.signature = signature
	}
}

// WithKeyIndex returns an option to set the key index for the transaction signature.
func (g *TransactionSignatureGenerator) WithKeyIndex(keyIndex uint32) func(*transactionSignatureConfig) {
	return func(config *transactionSignatureConfig) {
		config.keyIndex = keyIndex
	}
}

// Fixture generates a transaction signature with optional configuration.
func (g *TransactionSignatureGenerator) Fixture(t testing.TB, opts ...func(*transactionSignatureConfig)) flow.TransactionSignature {
	config := &transactionSignatureConfig{
		address:     g.addressGen.Fixture(t),
		signerIndex: 0,
		signature:   g.generateValidSignature(t),
		keyIndex:    1,
	}

	for _, opt := range opts {
		opt(config)
	}

	return flow.TransactionSignature{
		Address:     config.address,
		SignerIndex: config.signerIndex,
		Signature:   config.signature,
		KeyIndex:    config.keyIndex,
	}
}

// List generates a list of transaction signatures.
func (g *TransactionSignatureGenerator) List(t testing.TB, n int, opts ...func(*transactionSignatureConfig)) []flow.TransactionSignature {
	list := make([]flow.TransactionSignature, n)
	for i := range n {
		list[i] = g.Fixture(t, opts...)
	}
	return list
}

// generateValidSignature generates a valid ECDSA signature for the given transaction.
func (g *TransactionSignatureGenerator) generateValidSignature(t testing.TB) crypto.Signature {
	sigLen := crypto.SignatureLenECDSAP256
	signature := g.randomGen.RandomBytes(t, sigLen)

	// Make sure the ECDSA signature passes the format check
	signature[sigLen/2] = 0
	signature[0] = 0
	signature[sigLen/2-1] |= 1
	signature[sigLen-1] |= 1

	return signature
}
