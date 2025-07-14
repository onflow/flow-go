package flow

import "golang.org/x/exp/slices"

// TransactionBodyBuilder constructs a validated, immutable TransactionBody in two phases:
// first by setting individual fields using fluent WithX methods, then by calling Build()
// to perform minimal validity and sanity checks and return the final [TransactionBody].
type TransactionBodyBuilder struct {
	u UntrustedTransactionBody
}

// NewTransactionBodyBuilder help to build a new Transaction
func NewTransactionBodyBuilder() *TransactionBodyBuilder {
	return &TransactionBodyBuilder{}
}

// Build validates and returns an immutable TransactionBody. All required fields must be explicitly set (even if they are zero).
// All errors indicate that a valid TransactionBody cannot be created from the current builder state.
func (tb *TransactionBodyBuilder) Build() *TransactionBody {
	return NewTransactionBody(tb.u)
}

// SetReferenceBlockID sets the reference block ID for this transaction.
func (tb *TransactionBodyBuilder) SetReferenceBlockID(blockID Identifier) *TransactionBodyBuilder {
	tb.u.ReferenceBlockID = blockID
	return tb
}

// SetScript sets the Cadence script for this transaction.
func (tb *TransactionBodyBuilder) SetScript(script []byte) *TransactionBodyBuilder {
	tb.u.Script = script
	return tb
}

// SetArguments sets the Cadence arguments list for this transaction.
func (tb *TransactionBodyBuilder) SetArguments(args [][]byte) *TransactionBodyBuilder {
	tb.u.Arguments = args
	return tb
}

// AddArgument adds an argument to the Cadence arguments list for this transaction.
func (tb *TransactionBodyBuilder) AddArgument(arg []byte) *TransactionBodyBuilder {
	tb.u.Arguments = append(tb.u.Arguments, arg)
	return tb
}

// SetComputeLimit sets the gas limit for this transaction.
func (tb *TransactionBodyBuilder) SetComputeLimit(gasLimit uint64) *TransactionBodyBuilder {
	tb.u.GasLimit = gasLimit
	return tb
}

// SetProposalKey sets the proposal key and sequence number for this transaction.
//
// The first two arguments specify the account key to be used, and the last argument is the sequence
// number being declared.
func (tb *TransactionBodyBuilder) SetProposalKey(address Address, keyID uint32, sequenceNum uint64) *TransactionBodyBuilder {
	tb.u.ProposalKey = ProposalKey{
		Address:        address,
		KeyIndex:       keyID,
		SequenceNumber: sequenceNum,
	}
	return tb
}

// SetPayer sets the payer account for this transaction.
func (tb *TransactionBodyBuilder) SetPayer(payer Address) *TransactionBodyBuilder {
	tb.u.Payer = payer
	return tb
}

// SetAuthorizers sets authorizers accounts to this transaction.
func (tb *TransactionBodyBuilder) SetAuthorizers(authorizers []Address) *TransactionBodyBuilder {
	tb.u.Authorizers = authorizers
	return tb
}

// AddAuthorizer adds an authorizer account to this transaction.
func (tb *TransactionBodyBuilder) AddAuthorizer(authorizer Address) *TransactionBodyBuilder {
	tb.u.Authorizers = append(tb.u.Authorizers, authorizer)
	return tb
}

// AddPayloadSignature adds a payload signature to the transaction for the given address and key ID.
func (tb *TransactionBodyBuilder) AddPayloadSignature(address Address, keyID uint32, sig []byte) *TransactionBodyBuilder {
	s := tb.createSignature(address, keyID, sig)

	tb.u.PayloadSignatures = append(tb.u.EnvelopeSignatures, s)
	slices.SortFunc(tb.u.PayloadSignatures, compareSignatures)

	return tb
}

// AddEnvelopeSignature adds an envelope signature to the transaction for the given address and key ID.
func (tb *TransactionBodyBuilder) AddEnvelopeSignature(address Address, keyID uint32, sig []byte) *TransactionBodyBuilder {
	s := tb.createSignature(address, keyID, sig)

	tb.u.EnvelopeSignatures = append(tb.u.EnvelopeSignatures, s)
	slices.SortFunc(tb.u.EnvelopeSignatures, compareSignatures)

	return tb
}

func (tb *TransactionBodyBuilder) createSignature(address Address, keyID uint32, sig []byte) TransactionSignature {
	signerIndex, signerExists := tb.signerMap()[address]
	if !signerExists {
		signerIndex = -1
	}

	return TransactionSignature{
		Address:     address,
		SignerIndex: signerIndex,
		KeyIndex:    keyID,
		Signature:   sig,
	}
}

// signerMap returns a mapping from address to signer index.
func (tb *TransactionBodyBuilder) signerMap() map[Address]int {
	signers := make(map[Address]int)

	for i, signer := range tb.signerList() {
		signers[signer] = i
	}

	return signers
}

// signerList returns a list of unique accounts required to sign this transaction.
//
// The list is returned in the following order:
// 1. PROPOSER
// 2. PAYER
// 2. AUTHORIZERS (in insertion order)
//
// The only exception to the above ordering is for deduplication; if the same account
// is used in multiple signing roles, only the first occurrence is included in the list.
func (tb *TransactionBodyBuilder) signerList() []Address {
	signers := make([]Address, 0)
	seen := make(map[Address]struct{})

	var addSigner = func(address Address) {
		_, ok := seen[address]
		if ok {
			return
		}

		signers = append(signers, address)
		seen[address] = struct{}{}
	}

	if tb.u.ProposalKey.Address != EmptyAddress {
		addSigner(tb.u.ProposalKey.Address)
	}

	if tb.u.Payer != EmptyAddress {
		addSigner(tb.u.Payer)
	}

	for _, authorizer := range tb.u.Authorizers {
		addSigner(authorizer)
	}

	return signers
}
