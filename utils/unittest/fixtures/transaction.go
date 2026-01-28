package fixtures

import (
	"github.com/onflow/flow-go/model/flow"
)

// Transaction is the default options factory for [flow.TransactionBody] generation.
var Transaction transactionFactory

type transactionFactory struct{}

type TransactionOption func(*TransactionGenerator, *flow.TransactionBody)

// WithScript is an option that sets the script for the transaction.
func (f transactionFactory) WithScript(script []byte) TransactionOption {
	return func(g *TransactionGenerator, tx *flow.TransactionBody) {
		tx.Script = script
	}
}

// WithReferenceBlockID is an option that sets the reference block ID for the transaction.
func (f transactionFactory) WithReferenceBlockID(blockID flow.Identifier) TransactionOption {
	return func(g *TransactionGenerator, tx *flow.TransactionBody) {
		tx.ReferenceBlockID = blockID
	}
}

// WithGasLimit is an option that sets the gas limit for the transaction.
func (f transactionFactory) WithGasLimit(gasLimit uint64) TransactionOption {
	return func(g *TransactionGenerator, tx *flow.TransactionBody) {
		tx.GasLimit = gasLimit
	}
}

// WithProposalKey is an option that sets the proposal key for the transaction.
func (f transactionFactory) WithProposalKey(proposalKey flow.ProposalKey) TransactionOption {
	return func(g *TransactionGenerator, tx *flow.TransactionBody) {
		tx.ProposalKey = proposalKey
	}
}

// WithPayer is an option that sets the payer for the transaction.
func (f transactionFactory) WithPayer(payer flow.Address) TransactionOption {
	return func(g *TransactionGenerator, tx *flow.TransactionBody) {
		tx.Payer = payer
	}
}

// WithAuthorizers is an option that sets the authorizers for the transaction.
func (f transactionFactory) WithAuthorizers(authorizers ...flow.Address) TransactionOption {
	return func(g *TransactionGenerator, tx *flow.TransactionBody) {
		tx.Authorizers = authorizers
	}
}

// WithEnvelopeSignatures is an option that sets the envelope signatures for the transaction.
func (f transactionFactory) WithEnvelopeSignatures(signatures ...flow.TransactionSignature) TransactionOption {
	return func(g *TransactionGenerator, tx *flow.TransactionBody) {
		tx.EnvelopeSignatures = signatures
	}
}

// TransactionGenerator generates transactions with consistent randomness.
type TransactionGenerator struct {
	transactionFactory

	identifiers     *IdentifierGenerator
	proposalKeys    *ProposalKeyGenerator
	addresses       *AddressGenerator
	transactionSigs *TransactionSignatureGenerator
}

func NewTransactionGenerator(
	identifiers *IdentifierGenerator,
	proposalKeys *ProposalKeyGenerator,
	addresses *AddressGenerator,
	transactionSigs *TransactionSignatureGenerator,
) *TransactionGenerator {
	return &TransactionGenerator{
		identifiers:     identifiers,
		proposalKeys:    proposalKeys,
		addresses:       addresses,
		transactionSigs: transactionSigs,
	}
}

// Fixture generates a [flow.TransactionBody] with random data based on the provided options.
func (g *TransactionGenerator) Fixture(opts ...TransactionOption) *flow.TransactionBody {
	// simplify the transaction by using the same address for the proposer, payer and authorizers
	proposalKey := g.proposalKeys.Fixture()
	proposalAddress := proposalKey.Address

	tx := &flow.TransactionBody{
		ReferenceBlockID: g.identifiers.Fixture(),
		Script:           []byte("access(all) fun main() {}"),
		GasLimit:         10,
		ProposalKey:      proposalKey,
		Payer:            proposalAddress,
		Authorizers:      []flow.Address{proposalAddress},
	}

	for _, opt := range opts {
		opt(g, tx)
	}

	if len(tx.EnvelopeSignatures) == 0 {
		// use the proposer to sign the envelope. this ensures the metadata is consistent,
		// allowing the transaction to be properly serialized and deserialized over protobuf.
		// if the signature does not match the proposer, the signer index resolved during
		// deserialization will be incorrect.

		// this would also pass the sanity checks on the transaction signatures
		// by the access TransactionValidator
		tx.EnvelopeSignatures = []flow.TransactionSignature{
			g.transactionSigs.Fixture(
				TransactionSignature.WithAddress(tx.ProposalKey.Address),
				TransactionSignature.WithSignerIndex(0), // proposer should be index 0
			),
		}
	}

	return tx
}

// List generates a list of [flow.TransactionBody].
func (g *TransactionGenerator) List(n int, opts ...TransactionOption) []*flow.TransactionBody {
	list := make([]*flow.TransactionBody, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}
