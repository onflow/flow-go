package flow

import (
	"fmt"
	"slices"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"

	"github.com/onflow/flow-go/model/fingerprint"
)

// TransactionBodyBuilder constructs a validated, immutable [TransactionBody] in two phases:
// first by setting individual fields using fluent SetX methods, then by calling Build()
// to perform minimal validity and sanity checks and return the final [TransactionBody].
// Caution: TransactionBodyBuilder is not safe for concurrent use by multiple goroutines.
type TransactionBodyBuilder struct {
	u UntrustedTransactionBody
}

// NewTransactionBodyBuilder constructs an empty transaction builder.
func NewTransactionBodyBuilder() *TransactionBodyBuilder {
	return &TransactionBodyBuilder{}
}

// Build validates and returns an immutable TransactionBody.
// All errors indicate that a valid TransactionBody cannot be created from the current builder state.
func (tb *TransactionBodyBuilder) Build() (*TransactionBody, error) {
	slices.SortFunc(tb.u.PayloadSignatures, compareSignatures)
	slices.SortFunc(tb.u.EnvelopeSignatures, compareSignatures)
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

// AddAuthorizer adds an authorizer account to this transaction.
func (tb *TransactionBodyBuilder) AddAuthorizer(authorizer Address) *TransactionBodyBuilder {
	tb.u.Authorizers = append(tb.u.Authorizers, authorizer)
	return tb
}

// AddPayloadSignature adds a payload signature to the transaction for the given address and key ID.
func (tb *TransactionBodyBuilder) AddPayloadSignature(address Address, keyID uint32, sig []byte) *TransactionBodyBuilder {
	s := tb.createSignature(address, keyID, sig, nil)
	tb.u.PayloadSignatures = append(tb.u.PayloadSignatures, s)
	return tb
}

// AddEnvelopeSignature adds an envelope signature to the transaction for the given address and key ID.
func (tb *TransactionBodyBuilder) AddEnvelopeSignature(address Address, keyID uint32, sig []byte) *TransactionBodyBuilder {
	s := tb.createSignature(address, keyID, sig, nil)
	tb.u.EnvelopeSignatures = append(tb.u.EnvelopeSignatures, s)
	return tb
}

// AddPayloadSignature adds a payload signature to the transaction for the given address and key ID.
func (tb *TransactionBodyBuilder) AddPayloadSignatureWithExtensionData(address Address, keyID uint32, sig []byte, extensionData []byte) *TransactionBodyBuilder {
	s := tb.createSignature(address, keyID, sig, extensionData)
	tb.u.PayloadSignatures = append(tb.u.PayloadSignatures, s)
	return tb
}

// AddEnvelopeSignature adds an envelope signature to the transaction for the given address and key ID.
func (tb *TransactionBodyBuilder) AddEnvelopeSignatureWithExtensionData(address Address, keyID uint32, sig []byte, extensionData []byte) *TransactionBodyBuilder {
	s := tb.createSignature(address, keyID, sig, extensionData)
	tb.u.EnvelopeSignatures = append(tb.u.EnvelopeSignatures, s)
	return tb
}

func (tb *TransactionBodyBuilder) payloadMessage() []byte {
	return fingerprint.Fingerprint(tb.payloadCanonicalForm())
}

func (tb *TransactionBodyBuilder) payloadCanonicalForm() any {
	authorizers := make([][]byte, len(tb.u.Authorizers))
	for i, auth := range tb.u.Authorizers {
		authorizers[i] = auth.Bytes()
	}

	return struct {
		Script                    []byte
		Arguments                 [][]byte
		ReferenceBlockID          []byte
		GasLimit                  uint64
		ProposalKeyAddress        []byte
		ProposalKeyID             uint32
		ProposalKeySequenceNumber uint64
		Payer                     []byte
		Authorizers               [][]byte
	}{
		Script:                    tb.u.Script,
		Arguments:                 tb.u.Arguments,
		ReferenceBlockID:          tb.u.ReferenceBlockID[:],
		GasLimit:                  tb.u.GasLimit,
		ProposalKeyAddress:        tb.u.ProposalKey.Address.Bytes(),
		ProposalKeyID:             tb.u.ProposalKey.KeyIndex,
		ProposalKeySequenceNumber: tb.u.ProposalKey.SequenceNumber,
		Payer:                     tb.u.Payer.Bytes(),
		Authorizers:               authorizers,
	}
}

// EnvelopeMessage returns the signable message for transaction envelope.
//
// This message is only signed by the payer account.
func (tb *TransactionBodyBuilder) EnvelopeMessage() []byte {
	return fingerprint.Fingerprint(tb.envelopeCanonicalForm())
}

func (tb *TransactionBodyBuilder) envelopeCanonicalForm() any {
	return struct {
		Payload           any
		PayloadSignatures any
	}{
		tb.payloadCanonicalForm(),
		signaturesList(tb.u.PayloadSignatures).canonicalForm(),
	}
}

func (tb *TransactionBodyBuilder) createSignature(address Address, keyID uint32, sig []byte, extensionData []byte) TransactionSignature {
	signerIndex, signerExists := tb.signerMap()[address]
	if !signerExists {
		signerIndex = -1
	}

	return TransactionSignature{
		Address:       address,
		SignerIndex:   signerIndex,
		KeyIndex:      keyID,
		Signature:     sig,
		ExtensionData: extensionData,
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

// SignPayload signs the transaction payload (TransactionDomainTag + payload) with the specified account key using the default transaction domain tag.
//
// The resulting signature is combined with the account address and key ID before
// being added to the transaction.
//
// This function returns an error if the signature cannot be generated.
func (tb *TransactionBodyBuilder) SignPayload(
	address Address,
	keyID uint32,
	privateKey crypto.PrivateKey,
	hasher hash.Hasher,
) error {
	sig, err := tb.Sign(tb.payloadMessage(), privateKey, hasher)

	if err != nil {
		return fmt.Errorf("failed to sign transaction payload with given key: %w", err)
	}

	tb.AddPayloadSignature(address, keyID, sig)

	return nil
}

// SignEnvelope signs the full transaction (TransactionDomainTag + payload + payload signatures) with the specified account key using the default transaction domain tag.
//
// The resulting signature is combined with the account address and key ID before
// being added to the transaction.
//
// This function returns an error if the signature cannot be generated.
func (tb *TransactionBodyBuilder) SignEnvelope(
	address Address,
	keyID uint32,
	privateKey crypto.PrivateKey,
	hasher hash.Hasher,
) error {
	sig, err := tb.Sign(tb.EnvelopeMessage(), privateKey, hasher)

	if err != nil {
		return fmt.Errorf("failed to sign transaction envelope with given key: %w", err)
	}

	tb.AddEnvelopeSignature(address, keyID, sig)

	return nil
}

// Sign signs the data (transaction_tag + message) with the specified private key
// and hasher.
//
// This function returns an error if:
//   - crypto.InvalidInputsError if the private key cannot sign with the given hasher
//   - other error if an unexpected error occurs
func (tb *TransactionBodyBuilder) Sign(
	message []byte,
	privateKey crypto.PrivateKey,
	hasher hash.Hasher,
) ([]byte, error) {
	message = append(TransactionDomainTag[:], message...)
	sig, err := privateKey.Sign(message, hasher)
	if err != nil {
		return nil, fmt.Errorf("failed to sign message with given key: %w", err)
	}

	return sig, nil
}
