package flow

import (
	"fmt"
	"sort"

	"github.com/dapperlabs/flow-go/model/encoding"
)

// TransactionBody includes the main contents of a transaction
type TransactionBody struct {

	// A reference to a previous block
	// A transaction is expired after specific number of blocks (defined by network) counting from this block
	// for example, if block reference is pointing to a block with height of X and network limit is 10,
	// a block with x+10 height is the last block that is allowed to include this transaction.
	// user can adjust this reference to older blocks if he/she wants to make tx expire faster
	ReferenceBlockID Identifier

	// the script part of the transaction in Cadence Language
	Script []byte

	// Max amount of computation which is allowed to be done during this transaction
	GasLimit uint64

	// Account key used to propose the transaction
	ProposalKey ProposalKey

	// Account that pays for this transaction fees
	Payer Address

	// A ordered (ascending) list of addresses that scripts will touch their assets (including payer address)
	// Accounts listed here all have to provide signatures
	// Each account might provide multiple signatures (sum of weight should be at least 1)
	// If code touches accounts that is not listed here, tx fails
	Authorizers []Address

	// List of account signatures excluding signature of the payer account
	PayloadSignatures []TransactionSignature

	// payer signature over the envelope (payload + payload signatures)
	EnvelopeSignatures []TransactionSignature
}

// NewTransactionBody initializes and returns an empty transaction body
func NewTransactionBody() *TransactionBody {
	return &TransactionBody{}
}

func (tb TransactionBody) ID() Identifier {
	return MakeID(tb)
}

func (tb TransactionBody) Checksum() Identifier {
	return MakeID(tb)
}

// SetScript sets the Cadence script for this transaction.
func (tb *TransactionBody) SetScript(script []byte) *TransactionBody {
	tb.Script = script
	return tb
}

// SetReferenceBlockID sets the reference block ID for this transaction.
func (tb *TransactionBody) SetReferenceBlockID(blockID Identifier) *TransactionBody {
	tb.ReferenceBlockID = blockID
	return tb
}

// SetGasLimit sets the gas limit for this transaction.
func (tb *TransactionBody) SetGasLimit(limit uint64) *TransactionBody {
	tb.GasLimit = limit
	return tb
}

// SetProposalKey sets the proposal key and sequence number for this transaction.
//
// The first two arguments specify the account key to be used, and the last argument is the sequence
// number being declared.
func (tb *TransactionBody) SetProposalKey(address Address, keyID int, sequenceNum uint64) *TransactionBody {
	proposalKey := ProposalKey{
		Address:        address,
		KeyID:          keyID,
		SequenceNumber: sequenceNum,
	}
	tb.ProposalKey = proposalKey
	return tb
}

// SetPayer sets the payer account for this transaction.
func (tb *TransactionBody) SetPayer(address Address) *TransactionBody {
	tb.Payer = address
	return tb
}

// AddAuthorizer adds an authorizer account to this transaction.
func (tb *TransactionBody) AddAuthorizer(address Address) *TransactionBody {
	tb.Authorizers = append(tb.Authorizers, address)
	return tb
}

// Transaction is the smallest unit of task.
type Transaction struct {
	TransactionBody
	Status           TransactionStatus
	Events           []Event
	ComputationSpent uint64
	StartState       StateCommitment
	EndState         StateCommitment
}

func (tx *Transaction) PayloadMessage() []byte {
	body := tx.TransactionBody
	temp := body.payloadCanonicalForm()
	return encoding.DefaultEncoder.MustEncode(temp)
}

// Checksum provides a cryptographic commitment for a chunk content
func (tx *Transaction) Checksum() Identifier {
	return MakeID(tx)
}

func (tx *Transaction) String() string {
	return fmt.Sprintf("Transaction %v submitted by %v (block %v)",
		tx.ID(), tx.Payer.Hex(), tx.ReferenceBlockID)
}

// TransactionStatus represents the status of a Transaction.
type TransactionStatus int

const (
	// TransactionStatusUnknown indicates that the transaction status is not known.
	TransactionStatusUnknown TransactionStatus = iota
	// TransactionPending is the status of a pending transaction.
	TransactionPending
	// TransactionFinalized is the status of a finalized transaction.
	TransactionFinalized
	// TransactionReverted is the status of a reverted transaction.
	TransactionReverted
	// TransactionSealed is the status of a sealed transaction.
	TransactionSealed
)

// String returns the string representation of a transaction status.
func (s TransactionStatus) String() string {
	return [...]string{"PENDING", "FINALIZED", "REVERTED", "SEALED"}[s]
}

// TransactionField represents a required transaction field.
type TransactionField int

const (
	TransactionFieldUnknown TransactionField = iota
	TransactionFieldScript
	TransactionFieldRefBlockID
	TransactionFieldGasLimit
	TransactionFieldPayer
)

// String returns the string representation of a transaction field.
func (f TransactionField) String() string {
	return [...]string{"Unknown", "Script", "ReferenceBlockHash", "GasLimit", "Payer"}[f]
}

// MissingFields checks if a transaction is missing any required fields and returns those that are missing.
func (tb *TransactionBody) MissingFields() []string {
	// Required fields are Script, ReferenceBlockHash, Nonce, GasLimit, Payer
	missingFields := make([]string, 0)

	if len(tb.Script) == 0 {
		missingFields = append(missingFields, TransactionFieldScript.String())
	}

	if tb.ReferenceBlockID == ZeroID {
		missingFields = append(missingFields, TransactionFieldRefBlockID.String())
	}

	if tb.Payer == ZeroAddress {
		missingFields = append(missingFields, TransactionFieldPayer.String())
	}

	return missingFields
}

// A ProposalKey is the key that specifies the proposal key and sequence number for a transaction.
type ProposalKey struct {
	Address        Address
	KeyID          int
	SequenceNumber uint64
}

// A TransactionSignature is a signature associated with a specific account key.
type TransactionSignature struct {
	Address     Address
	SignerIndex int
	KeyID       int
	Signature   []byte
}

func (s TransactionSignature) canonicalForm() interface{} {
	return struct {
		SignerIndex uint
		KeyID       uint
		Signature   []byte
	}{
		SignerIndex: uint(s.SignerIndex), // int is not RLP-serializable
		KeyID:       uint(s.KeyID),       // int is not RLP-serializable
		Signature:   s.Signature,
	}
}

func compareSignatures(signatures []TransactionSignature) func(i, j int) bool {
	return func(i, j int) bool {
		sigA := signatures[i]
		sigB := signatures[j]
		return sigA.SignerIndex < sigB.SignerIndex || sigA.KeyID < sigB.KeyID
	}
}

type signaturesList []TransactionSignature

func (s signaturesList) canonicalForm() interface{} {
	signatures := make([]interface{}, len(s))

	for i, signature := range s {
		signatures[i] = signature.canonicalForm()
	}

	return signatures
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
func (tb *TransactionBody) signerList() []Address {
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

	if tb.ProposalKey.Address != ZeroAddress {
		addSigner(tb.ProposalKey.Address)
	}

	if tb.Payer != ZeroAddress {
		addSigner(tb.Payer)
	}

	for _, authorizer := range tb.Authorizers {
		addSigner(authorizer)
	}

	return signers
}

// signerMap returns a mapping from address to signer index.
func (tb *TransactionBody) signerMap() map[Address]int {
	signers := make(map[Address]int)

	for i, signer := range tb.signerList() {
		signers[signer] = i
	}

	return signers
}

// SignPayload signs the transaction payload with the specified account key.
//
// The resulting signature is combined with the account address and key ID before
// being added to the transaction.
//
// This function returns an error if the signature cannot be generated.
//func (t *Transaction) SignPayload(address Address, keyID int, signer crypto.Signer) error {
//	sig, err := signer.Sign(t.PayloadMessage())
//	if err != nil {
//		// TODO: wrap error
//		return err
//	}
//
//	t.AddPayloadSignature(address, keyID, sig)
//
//	return nil
//}

// SignEnvelope signs the full transaction (payload + payload signatures) with the specified account key.
//
// The resulting signature is combined with the account address and key ID before
// being added to the transaction.
//
// This function returns an error if the signature cannot be generated.
//func (t *Transaction) SignEnvelope(address Address, keyID int, signer crypto.Signer) error {
//	sig, err := signer.Sign(t.EnvelopeMessage())
//	if err != nil {
//		// TODO: wrap error
//		return err
//	}
//
//	t.AddEnvelopeSignature(address, keyID, sig)
//
//	return nil
//}

// AddPayloadSignature adds a payload signature to the transaction for the given address and key ID.
func (tb *TransactionBody) AddPayloadSignature(address Address, keyID int, sig []byte) *TransactionBody {
	s := tb.createSignature(address, keyID, sig)

	tb.PayloadSignatures = append(tb.PayloadSignatures, s)
	sort.Slice(tb.PayloadSignatures, compareSignatures(tb.PayloadSignatures))

	return tb
}

// AddEnvelopeSignature adds an envelope signature to the transaction for the given address and key ID.
func (tb *TransactionBody) AddEnvelopeSignature(address Address, keyID int, sig []byte) *TransactionBody {
	s := tb.createSignature(address, keyID, sig)

	tb.EnvelopeSignatures = append(tb.EnvelopeSignatures, s)
	sort.Slice(tb.EnvelopeSignatures, compareSignatures(tb.EnvelopeSignatures))

	return tb
}

func (tb *TransactionBody) createSignature(address Address, keyID int, sig []byte) TransactionSignature {
	signerIndex, signerExists := tb.signerMap()[address]
	if !signerExists {
		signerIndex = -1
	}

	return TransactionSignature{
		Address:     address,
		SignerIndex: signerIndex,
		KeyID:       keyID,
		Signature:   sig,
	}
}

func (tb *TransactionBody) PayloadMessage() []byte {
	temp := tb.payloadCanonicalForm()
	return encoding.DefaultEncoder.MustEncode(temp)
}

func (tb *TransactionBody) payloadCanonicalForm() interface{} {
	authorizers := make([][]byte, len(tb.Authorizers))
	for i, auth := range tb.Authorizers {
		authorizers[i] = auth.Bytes()
	}

	return struct {
		Script                    []byte
		ReferenceBlockID          []byte
		GasLimit                  uint64
		ProposalKeyAddress        []byte
		ProposalKeyID             uint64
		ProposalKeySequenceNumber uint64
		Payer                     []byte
		Authorizers               [][]byte
	}{
		tb.Script,
		tb.ReferenceBlockID[:],
		tb.GasLimit,
		tb.ProposalKey.Address.Bytes(),
		uint64(tb.ProposalKey.KeyID),
		tb.ProposalKey.SequenceNumber,
		tb.Payer.Bytes(),
		authorizers,
	}
}

// EnvelopeMessage returns the signable message for transaction envelope.
//
// This message is only signed by the payer account.
func (tb *TransactionBody) EnvelopeMessage() []byte {
	temp := tb.envelopeCanonicalForm()
	return encoding.DefaultEncoder.MustEncode(temp)
}

func (tb *TransactionBody) envelopeCanonicalForm() interface{} {
	return struct {
		Payload           interface{}
		PayloadSignatures interface{}
	}{
		tb.payloadCanonicalForm(),
		signaturesList(tb.PayloadSignatures).canonicalForm(),
	}
}

// Encode serializes the full transaction data including the payload and all signatures.
func (tb *TransactionBody) Encode() []byte {
	temp := struct {
		Payload            interface{}
		PayloadSignatures  interface{}
		EnvelopeSignatures interface{}
	}{
		tb.payloadCanonicalForm(),
		signaturesList(tb.PayloadSignatures).canonicalForm(),
		signaturesList(tb.EnvelopeSignatures).canonicalForm(),
	}

	return encoding.DefaultEncoder.MustEncode(temp)
}
