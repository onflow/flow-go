package fixtures

import (
	"testing"

	"github.com/onflow/flow-go/model/flow"
)

// TransactionGenerator generates transactions with consistent randomness.
type TransactionGenerator struct {
	identifierGen     *IdentifierGenerator
	proposalKeyGen    *ProposalKeyGenerator
	addressGen        *AddressGenerator
	transactionSigGen *TransactionSignatureGenerator
}

// transactionConfig holds the configuration for transaction generation.
type transactionConfig struct {
	script             []byte
	referenceBlockID   flow.Identifier
	gasLimit           uint64
	proposalKey        flow.ProposalKey
	payer              flow.Address
	authorizers        []flow.Address
	envelopeSignatures []flow.TransactionSignature
}

// WithScript returns an option to set the script for the transaction.
func (g *TransactionGenerator) WithScript(script []byte) func(*transactionConfig) {
	return func(config *transactionConfig) {
		config.script = script
	}
}

// WithReferenceBlockID returns an option to set the reference block ID for the transaction.
func (g *TransactionGenerator) WithReferenceBlockID(blockID flow.Identifier) func(*transactionConfig) {
	return func(config *transactionConfig) {
		config.referenceBlockID = blockID
	}
}

// WithGasLimit returns an option to set the gas limit for the transaction.
func (g *TransactionGenerator) WithGasLimit(gasLimit uint64) func(*transactionConfig) {
	return func(config *transactionConfig) {
		config.gasLimit = gasLimit
	}
}

// WithProposalKey returns an option to set the proposal key for the transaction.
func (g *TransactionGenerator) WithProposalKey(proposalKey flow.ProposalKey) func(*transactionConfig) {
	return func(config *transactionConfig) {
		config.proposalKey = proposalKey
	}
}

// WithPayer returns an option to set the payer for the transaction.
func (g *TransactionGenerator) WithPayer(payer flow.Address) func(*transactionConfig) {
	return func(config *transactionConfig) {
		config.payer = payer
	}
}

// WithAuthorizers returns an option to set the authorizers for the transaction.
func (g *TransactionGenerator) WithAuthorizers(authorizers []flow.Address) func(*transactionConfig) {
	return func(config *transactionConfig) {
		config.authorizers = authorizers
	}
}

// WithEnvelopeSignatures returns an option to set the envelope signatures for the transaction.
func (g *TransactionGenerator) WithEnvelopeSignatures(signatures []flow.TransactionSignature) func(*transactionConfig) {
	return func(config *transactionConfig) {
		config.envelopeSignatures = signatures
	}
}

// Fixture generates a transaction body with optional configuration.
func (g *TransactionGenerator) Fixture(t testing.TB, opts ...func(*transactionConfig)) *flow.TransactionBody {
	config := &transactionConfig{
		script:           []byte("access(all) fun main() {}"),
		referenceBlockID: g.identifierGen.Fixture(t),
		gasLimit:         10,
		proposalKey:      g.proposalKeyGen.Fixture(t),
		payer:            g.addressGen.Fixture(t),
		authorizers:      []flow.Address{g.addressGen.Fixture(t)},
	}

	for _, opt := range opts {
		opt(config)
	}

	if len(config.envelopeSignatures) == 0 {
		// use the proposer to sign the envelope. this ensures the metadata is consistent,
		// allowing the transaction to be properly serialized and deserialized over protobuf.
		// if the signature does not match the proposer, the signer index resolved during
		// deserialization will be incorrect.
		config.envelopeSignatures = []flow.TransactionSignature{
			g.transactionSigGen.Fixture(t,
				g.transactionSigGen.WithAddress(config.proposalKey.Address),
				g.transactionSigGen.WithSignerIndex(0), // proposer should be index 0
			),
		}
	}

	return &flow.TransactionBody{
		Script:             config.script,
		ReferenceBlockID:   config.referenceBlockID,
		GasLimit:           config.gasLimit,
		ProposalKey:        config.proposalKey,
		Payer:              config.payer,
		Authorizers:        config.authorizers,
		EnvelopeSignatures: config.envelopeSignatures,
	}
}

// List generates a list of transaction bodies.
func (g *TransactionGenerator) List(t testing.TB, n int, opts ...func(*transactionConfig)) []flow.TransactionBody {
	list := make([]flow.TransactionBody, n)
	for i := range n {
		list[i] = *g.Fixture(t, opts...)
	}
	return list
}

// FullTransactionGenerator generates transactions with consistent randomness.
type FullTransactionGenerator struct {
	*TransactionGenerator
}

// Transaction generates a complete transaction with optional configuration.
func (g *FullTransactionGenerator) Fixture(t testing.TB, opts ...func(*transactionConfig)) flow.Transaction {
	tb := g.TransactionGenerator.Fixture(t, opts...)
	return flow.Transaction{TransactionBody: *tb}
}

// TransactionList generates a list of complete transactions.
func (g *FullTransactionGenerator) List(t testing.TB, n int, opts ...func(*transactionConfig)) []flow.Transaction {
	list := make([]flow.Transaction, n)
	for i := range n {
		list[i] = g.Fixture(t, opts...)
	}
	return list
}
