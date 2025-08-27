package flow

import (
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/rlp"

	flowrlp "github.com/onflow/flow-go/model/encoding/rlp"
	"github.com/onflow/flow-go/model/fingerprint"
)

// TransactionBody includes the main contents of a transaction
//
//structwrite:immutable - mutations allowed only within the constructor
type TransactionBody struct {

	// A reference to a previous block
	// A transaction is expired after specific number of blocks (defined by network) counting from this block
	// for example, if block reference is pointing to a block with height of X and network limit is 10,
	// a block with x+10 height is the last block that is allowed to include this transaction.
	// user can adjust this reference to older blocks if he/she wants to make tx expire faster
	ReferenceBlockID Identifier

	// the transaction script as UTF-8 encoded Cadence source code
	Script []byte

	// arguments passed to the Cadence transaction
	Arguments [][]byte

	// Max amount of computation which is allowed to be done during this transaction
	GasLimit uint64

	// Account key used to propose the transaction
	ProposalKey ProposalKey

	// Account that pays for this transaction fees
	Payer Address

	// An ordered (ascending) list of addresses that scripts will touch their assets (including payer address)
	// Accounts listed here all have to provide signatures
	// Each account might provide multiple signatures (sum of weight should be at least 1)
	// If code touches accounts that is not listed here, tx fails
	Authorizers []Address

	// List of account signatures excluding signature of the payer account
	PayloadSignatures []TransactionSignature

	// payer signature over the envelope (payload + payload signatures)
	EnvelopeSignatures []TransactionSignature
}

// UntrustedTransactionBody is an untrusted input-only representation of a TransactionBody,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedTransactionBody should be validated and converted into
// a trusted TransactionBody using NewTransactionBody constructor.
type UntrustedTransactionBody TransactionBody

// NewTransactionBody creates a new instance of TransactionBody.
// Construction of TransactionBody is allowed only within the constructor.
//
// All errors indicate a valid TransactionBody cannot be constructed from the input.
func NewTransactionBody(untrusted UntrustedTransactionBody) (*TransactionBody, error) {

	if len(untrusted.Script) == 0 {
		return nil, fmt.Errorf("Script must not be empty")
	}

	return &TransactionBody{
		ReferenceBlockID:   untrusted.ReferenceBlockID,
		Script:             untrusted.Script,
		Arguments:          untrusted.Arguments,
		GasLimit:           untrusted.GasLimit,
		ProposalKey:        untrusted.ProposalKey,
		Payer:              untrusted.Payer,
		Authorizers:        untrusted.Authorizers,
		PayloadSignatures:  untrusted.PayloadSignatures,
		EnvelopeSignatures: untrusted.EnvelopeSignatures,
	}, nil
}

// Fingerprint returns the canonical, unique byte representation for the TransactionBody.
// As RLP encoding logic for TransactionBody is over-ridden by EncodeRLP below, this is
// equivalent to directly RLP encoding the TransactionBody.
// This public function is retained primarily for backward compatibility.
func (tb TransactionBody) Fingerprint() []byte {
	return flowrlp.NewMarshaler().MustMarshal(tb)
}

// EncodeRLP defines RLP encoding behaviour for TransactionBody.
func (tb TransactionBody) EncodeRLP(w io.Writer) error {
	encodingCanonicalForm := struct {
		Payload            interface{}
		PayloadSignatures  interface{}
		EnvelopeSignatures interface{}
	}{
		Payload:            tb.payloadCanonicalForm(),
		PayloadSignatures:  signaturesList(tb.PayloadSignatures).canonicalForm(),
		EnvelopeSignatures: signaturesList(tb.EnvelopeSignatures).canonicalForm(),
	}
	return rlp.Encode(w, encodingCanonicalForm)
}

func (tb TransactionBody) ByteSize() uint {
	size := 0
	size += len(tb.ReferenceBlockID)
	size += len(tb.Script)
	for _, arg := range tb.Arguments {
		size += len(arg)
	}
	size += 8 // gas size
	size += tb.ProposalKey.ByteSize()
	size += AddressLength                       // payer address
	size += len(tb.Authorizers) * AddressLength // Authorizers
	for _, s := range tb.PayloadSignatures {
		size += s.ByteSize()
	}
	for _, s := range tb.EnvelopeSignatures {
		size += s.ByteSize()
	}
	return uint(size)
}

// InclusionEffort returns the inclusion effort of the transaction
func (tb TransactionBody) InclusionEffort() uint64 {
	// Hardcoded inclusion effort (of 1.0 UFix).
	// Eventually this will be dynamic and will depend on the transaction properties
	inclusionEffort := uint64(100_000_000)
	return inclusionEffort
}

func (tb TransactionBody) ID() Identifier {
	return MakeID(tb)
}

// Transaction is the smallest unit of task.
//
//structwrite:immutable - mutations allowed only within the constructor
type Transaction struct {
	TransactionBody
	Status           TransactionStatus
	Events           []Event
	ComputationSpent uint64
	StartState       StateCommitment
	EndState         StateCommitment
}

// MissingFields checks if a transaction is missing any required fields and returns those that are missing.
func (tb *TransactionBody) MissingFields() []string {
	// Required fields are Script, ReferenceBlockID, Payer
	missingFields := make([]string, 0)

	if len(tb.Script) == 0 {
		missingFields = append(missingFields, TransactionFieldScript.String())
	}

	if tb.ReferenceBlockID == ZeroID {
		missingFields = append(missingFields, TransactionFieldRefBlockID.String())
	}

	if tb.Payer == EmptyAddress {
		missingFields = append(missingFields, TransactionFieldPayer.String())
	}

	return missingFields
}

func (tb *TransactionBody) PayloadMessage() []byte {
	return fingerprint.Fingerprint(tb.payloadCanonicalForm())
}

func (tb *TransactionBody) payloadCanonicalForm() interface{} {
	authorizers := make([][]byte, len(tb.Authorizers))
	for i, auth := range tb.Authorizers {
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
		Script:                    tb.Script,
		Arguments:                 tb.Arguments,
		ReferenceBlockID:          tb.ReferenceBlockID[:],
		GasLimit:                  tb.GasLimit,
		ProposalKeyAddress:        tb.ProposalKey.Address.Bytes(),
		ProposalKeyID:             tb.ProposalKey.KeyIndex,
		ProposalKeySequenceNumber: tb.ProposalKey.SequenceNumber,
		Payer:                     tb.Payer.Bytes(),
		Authorizers:               authorizers,
	}
}

// EnvelopeMessage returns the signable message for transaction envelope.
//
// This message is only signed by the payer account.
func (tb *TransactionBody) EnvelopeMessage() []byte {
	return fingerprint.Fingerprint(tb.envelopeCanonicalForm())
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

func (tx *Transaction) String() string {
	return fmt.Sprintf("Transaction %v submitted by %v (block %v)",
		tx.ID(), tx.Payer.Hex(), tx.ReferenceBlockID)
}

// TransactionStatus represents the status of a transaction.
type TransactionStatus int

const (
	// TransactionStatusUnknown indicates that the transaction status is not known.
	TransactionStatusUnknown TransactionStatus = iota
	// TransactionStatusPending is the status of a pending transaction.
	TransactionStatusPending
	// TransactionStatusFinalized is the status of a finalized transaction.
	TransactionStatusFinalized
	// TransactionStatusExecuted is the status of an executed transaction.
	TransactionStatusExecuted
	// TransactionStatusSealed is the status of a sealed transaction.
	TransactionStatusSealed
	// TransactionStatusExpired is the status of an expired transaction.
	TransactionStatusExpired
)

// String returns the string representation of a transaction status.
func (s TransactionStatus) String() string {
	return [...]string{"UNKNOWN", "PENDING", "FINALIZED", "EXECUTED", "SEALED", "EXPIRED"}[s]
}

// TransactionField represents a required transaction field.
type TransactionField int

const (
	TransactionFieldUnknown TransactionField = iota
	TransactionFieldScript
	TransactionFieldRefBlockID
	TransactionFieldPayer
)

// String returns the string representation of a transaction field.
func (f TransactionField) String() string {
	return [...]string{"Unknown", "Script", "ReferenceBlockID", "Payer"}[f]
}

// A ProposalKey is the key that specifies the proposal key and sequence number for a transaction.
type ProposalKey struct {
	Address        Address
	KeyIndex       uint32
	SequenceNumber uint64
}

// ByteSize returns the byte size of the proposal key
func (p ProposalKey) ByteSize() int {
	keyIDLen := 8
	sequenceNumberLen := 8
	return len(p.Address) + keyIDLen + sequenceNumberLen
}

// A TransactionSignature is a signature associated with a specific account key.
type TransactionSignature struct {
	Address     Address
	SignerIndex int
	KeyIndex    uint32
	Signature   []byte
}

// String returns the string representation of a transaction signature.
func (s TransactionSignature) String() string {
	return fmt.Sprintf("Address: %s. SignerIndex: %d. KeyID: %d. Signature: %s",
		s.Address, s.SignerIndex, s.KeyIndex, s.Signature)
}

// ByteSize returns the byte size of the transaction signature
func (s TransactionSignature) ByteSize() int {
	signerIndexLen := 8
	keyIDLen := 8
	return len(s.Address) + signerIndexLen + keyIDLen + len(s.Signature)
}

func (s TransactionSignature) Fingerprint() []byte {
	return fingerprint.Fingerprint(s.canonicalForm())
}

func (s TransactionSignature) canonicalForm() interface{} {
	return struct {
		SignerIndex uint
		KeyID       uint
		Signature   []byte
	}{
		SignerIndex: uint(s.SignerIndex), // int is not RLP-serializable
		KeyID:       uint(s.KeyIndex),    // int is not RLP-serializable
		Signature:   s.Signature,
	}
}

func compareSignatures(sigA, sigB TransactionSignature) int {
	if sigA.SignerIndex == sigB.SignerIndex {
		return int(sigA.KeyIndex) - int(sigB.KeyIndex)
	}

	return sigA.SignerIndex - sigB.SignerIndex
}

type signaturesList []TransactionSignature

func (s signaturesList) canonicalForm() interface{} {
	signatures := make([]interface{}, len(s))

	for i, signature := range s {
		signatures[i] = signature.canonicalForm()
	}

	return signatures
}
