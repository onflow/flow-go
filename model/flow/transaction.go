package flow

import (
	"fmt"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
	"golang.org/x/exp/slices"

	"github.com/onflow/flow-go/model/fingerprint"
)

// TransactionBody includes the main contents of a transaction
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

func (tb TransactionBody) Fingerprint() []byte {
	return fingerprint.Fingerprint(struct {
		Payload            interface{}
		PayloadSignatures  interface{}
		EnvelopeSignatures interface{}
	}{
		Payload:            tb.payloadCanonicalForm(),
		PayloadSignatures:  signaturesList(tb.PayloadSignatures).canonicalForm(),
		EnvelopeSignatures: signaturesList(tb.EnvelopeSignatures).canonicalForm(),
	})
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

func (tb TransactionBody) Checksum() Identifier {
	return MakeID(tb)
}

// SetScript sets the Cadence script for this transaction.
func (tb *TransactionBody) SetScript(script []byte) *TransactionBody {
	tb.Script = script
	return tb
}

// SetArguments sets the Cadence arguments list for this transaction.
func (tb *TransactionBody) SetArguments(args [][]byte) *TransactionBody {
	tb.Arguments = args
	return tb
}

// AddArgument adds an argument to the Cadence arguments list for this transaction.
func (tb *TransactionBody) AddArgument(arg []byte) *TransactionBody {
	tb.Arguments = append(tb.Arguments, arg)
	return tb
}

// SetReferenceBlockID sets the reference block ID for this transaction.
func (tb *TransactionBody) SetReferenceBlockID(blockID Identifier) *TransactionBody {
	tb.ReferenceBlockID = blockID
	return tb
}

// SetComputeLimit sets the gas limit for this transaction.
func (tb *TransactionBody) SetComputeLimit(limit uint64) *TransactionBody {
	tb.GasLimit = limit
	return tb
}

// SetProposalKey sets the proposal key and sequence number for this transaction.
//
// The first two arguments specify the account key to be used, and the last argument is the sequence
// number being declared.
func (tb *TransactionBody) SetProposalKey(address Address, keyID uint64, sequenceNum uint64) *TransactionBody {
	proposalKey := ProposalKey{
		Address:        address,
		KeyIndex:       keyID,
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

	if tb.ProposalKey.Address != EmptyAddress {
		addSigner(tb.ProposalKey.Address)
	}

	if tb.Payer != EmptyAddress {
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

// SignPayload signs the transaction payload (TransactionDomainTag + payload) with the specified account key using the default transaction domain tag.
//
// The resulting signature is combined with the account address and key ID before
// being added to the transaction.
//
// This function returns an error if the signature cannot be generated.
func (tb *TransactionBody) SignPayload(
	address Address,
	keyID uint64,
	privateKey crypto.PrivateKey,
	hasher hash.Hasher,
) error {
	sig, err := tb.Sign(tb.PayloadMessage(), privateKey, hasher)

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
func (tb *TransactionBody) SignEnvelope(
	address Address,
	keyID uint64,
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
func (tb *TransactionBody) Sign(
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

// AddPayloadSignature adds a payload signature to the transaction for the given address and key ID.
func (tb *TransactionBody) AddPayloadSignature(address Address, keyID uint64, sig []byte) *TransactionBody {
	s := tb.createSignature(address, keyID, sig)

	tb.PayloadSignatures = append(tb.PayloadSignatures, s)
	slices.SortFunc(tb.PayloadSignatures, compareSignatures)

	return tb
}

// AddEnvelopeSignature adds an envelope signature to the transaction for the given address and key ID.
func (tb *TransactionBody) AddEnvelopeSignature(address Address, keyID uint64, sig []byte) *TransactionBody {
	s := tb.createSignature(address, keyID, sig)

	tb.EnvelopeSignatures = append(tb.EnvelopeSignatures, s)
	slices.SortFunc(tb.EnvelopeSignatures, compareSignatures)

	return tb
}

func (tb *TransactionBody) createSignature(address Address, keyID uint64, sig []byte) TransactionSignature {
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
		ProposalKeyID             uint64
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

func (tx *Transaction) PayloadMessage() []byte {
	return fingerprint.Fingerprint(tx.TransactionBody.payloadCanonicalForm())
}

// Checksum provides a cryptographic commitment for a chunk content
func (tx *Transaction) Checksum() Identifier {
	return MakeID(tx)
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
	KeyIndex       uint64
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
	KeyIndex    uint64
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
