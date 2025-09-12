package fixtures

import (
	"github.com/onflow/flow-go/model/flow"
)

// TransactionGenerator generates transactions with consistent randomness.
type TransactionGenerator struct {
	identifierGen     *IdentifierGenerator
	proposalKeyGen    *ProposalKeyGenerator
	addressGen        *AddressGenerator
	transactionSigGen *TransactionSignatureGenerator
}

func NewTransactionGenerator(
	identifierGen *IdentifierGenerator,
	proposalKeyGen *ProposalKeyGenerator,
	addressGen *AddressGenerator,
	transactionSigGen *TransactionSignatureGenerator,
) *TransactionGenerator {
	return &TransactionGenerator{
		identifierGen:     identifierGen,
		proposalKeyGen:    proposalKeyGen,
		addressGen:        addressGen,
		transactionSigGen: transactionSigGen,
	}
}

// WithScript is an option that sets the script for the transaction.
func (g *TransactionGenerator) WithScript(script []byte) func(*flow.TransactionBody) {
	return func(tx *flow.TransactionBody) {
		tx.Script = script
	}
}

// WithReferenceBlockID is an option that sets the reference block ID for the transaction.
func (g *TransactionGenerator) WithReferenceBlockID(blockID flow.Identifier) func(*flow.TransactionBody) {
	return func(tx *flow.TransactionBody) {
		tx.ReferenceBlockID = blockID
	}
}

// WithGasLimit is an option that sets the gas limit for the transaction.
func (g *TransactionGenerator) WithGasLimit(gasLimit uint64) func(*flow.TransactionBody) {
	return func(tx *flow.TransactionBody) {
		tx.GasLimit = gasLimit
	}
}

// WithProposalKey is an option that sets the proposal key for the transaction.
func (g *TransactionGenerator) WithProposalKey(proposalKey flow.ProposalKey) func(*flow.TransactionBody) {
	return func(tx *flow.TransactionBody) {
		tx.ProposalKey = proposalKey
	}
}

// WithPayer is an option that sets the payer for the transaction.
func (g *TransactionGenerator) WithPayer(payer flow.Address) func(*flow.TransactionBody) {
	return func(tx *flow.TransactionBody) {
		tx.Payer = payer
	}
}

// WithAuthorizers is an option that sets the authorizers for the transaction.
func (g *TransactionGenerator) WithAuthorizers(authorizers []flow.Address) func(*flow.TransactionBody) {
	return func(tx *flow.TransactionBody) {
		tx.Authorizers = authorizers
	}
}

// WithEnvelopeSignatures is an option that sets the envelope signatures for the transaction.
func (g *TransactionGenerator) WithEnvelopeSignatures(signatures []flow.TransactionSignature) func(*flow.TransactionBody) {
	return func(tx *flow.TransactionBody) {
		tx.EnvelopeSignatures = signatures
	}
}

// Fixture generates a [flow.TransactionBody] with random data based on the provided options.
func (g *TransactionGenerator) Fixture(opts ...func(*flow.TransactionBody)) *flow.TransactionBody {
	tx := &flow.TransactionBody{
		ReferenceBlockID: g.identifierGen.Fixture(),
		Script:           []byte("access(all) fun main() {}"),
		GasLimit:         10,
		ProposalKey:      g.proposalKeyGen.Fixture(),
		Payer:            g.addressGen.Fixture(),
		Authorizers:      []flow.Address{g.addressGen.Fixture()},
	}

	for _, opt := range opts {
		opt(tx)
	}

	if len(tx.EnvelopeSignatures) == 0 {
		// use the proposer to sign the envelope. this ensures the metadata is consistent,
		// allowing the transaction to be properly serialized and deserialized over protobuf.
		// if the signature does not match the proposer, the signer index resolved during
		// deserialization will be incorrect.
		tx.EnvelopeSignatures = []flow.TransactionSignature{
			g.transactionSigGen.Fixture(
				g.transactionSigGen.WithAddress(tx.ProposalKey.Address),
				g.transactionSigGen.WithSignerIndex(0), // proposer should be index 0
			),
		}
	}

	return tx
}

// List generates a list of [flow.TransactionBody].
func (g *TransactionGenerator) List(n int, opts ...func(*flow.TransactionBody)) []*flow.TransactionBody {
	list := make([]*flow.TransactionBody, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}
