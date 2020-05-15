package virtualmachine

import (
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"

	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/model/flow"
)

const scriptGasLimit = 100000

type CheckerFunc func([]byte, runtime.Location) error

type TransactionContext struct {
	ledger          LedgerDAL
	astCache        ASTCache
	signingAccounts []runtime.Address
	checker         CheckerFunc
	logs            []string
	events          []cadence.Event
	tx              *flow.TransactionBody
	gasLimit        uint64
	uuid            uint64 // TODO: implement proper UUID
}

type TransactionContextOption func(*TransactionContext)

// GetSigningAccounts gets the signing accounts for this context.
//
// Signing accounts are the accounts that signed the transaction executing
// inside this context.
func (r *TransactionContext) GetSigningAccounts() []runtime.Address {
	return r.signingAccounts
}

// SetChecker sets the semantic checker function for this context.
func (r *TransactionContext) SetChecker(checker CheckerFunc) {
	r.checker = checker
}

// Events returns all events emitted by the runtime to this context.
func (r *TransactionContext) Events() []cadence.Event {
	return r.events
}

// Logs returns all logs emitted by the runtime to this context.
func (r *TransactionContext) Logs() []string {
	return r.logs
}

// GetValue gets a register value from the world state.
func (r *TransactionContext) GetValue(owner, controller, key []byte) ([]byte, error) {
	v, _ := r.ledger.Get(fullKeyHash(string(owner), string(controller), string(key)))
	return v, nil
}

// SetValue sets a register value in the world state.
func (r *TransactionContext) SetValue(owner, controller, key, value []byte) error {
	r.ledger.Set(fullKeyHash(string(owner), string(controller), string(key)), value)
	return nil
}

func (r *TransactionContext) ValueExists(owner, controller, key []byte) (exists bool, err error) {
	v, err := r.GetValue(owner, controller, key)
	if err != nil {
		return false, err
	}

	return len(v) > 0, nil
}

// CreateAccount creates a new account and inserts it into the world state.
//
// This function returns an error if the input is invalid.
//
// After creating the account, this function calls the onAccountCreated callback registered
// with this context.
func (r *TransactionContext) CreateAccount(publicKeysBytes [][]byte) (runtime.Address, error) {

	publicKeys := make([]flow.AccountPublicKey, len(publicKeysBytes))
	var err error
	for i, keyBytes := range publicKeysBytes {
		publicKeys[i], err = flow.DecodeRuntimeAccountPublicKey(keyBytes, 0)
		if err != nil {
			return runtime.Address{}, fmt.Errorf("cannot decode public key %d: %w", i, err)
		}
	}

	accountAddress, err := r.ledger.CreateAccount(publicKeys, nil)
	r.Log(fmt.Sprintf("Created new account with address: %x", accountAddress))

	return runtime.Address(accountAddress), err
}

// AddAccountKey adds a public key to an existing account.
//
// This function returns an error if the specified account does not exist or
// if the key insertion fails.
func (r *TransactionContext) AddAccountKey(address runtime.Address, publicKey []byte) error {
	accountAddress := address.Bytes()

	err := r.ledger.CheckAccountExists(accountAddress)
	if err != nil {
		return err
	}

	runtimePublicKey, err := flow.DecodeRuntimeAccountPublicKey(publicKey, 0)
	if err != nil {
		return fmt.Errorf("cannot decode runtime public account key: %w", err)
	}

	publicKeys, err := r.ledger.GetAccountPublicKeys(accountAddress)
	if err != nil {
		return err
	}

	publicKeys = append(publicKeys, runtimePublicKey)

	return r.ledger.SetAccountPublicKeys(accountAddress, publicKeys)
}

// RemoveAccountKey removes a public key by index from an existing account.
//
// This function returns an error if the specified account does not exist, the
// provided key is invalid, or if key deletion fails.
func (r *TransactionContext) RemoveAccountKey(address runtime.Address, index int) (publicKey []byte, err error) {
	accountAddress := address.Bytes()

	err = r.ledger.CheckAccountExists(accountAddress)
	if err != nil {
		return nil, err
	}

	publicKeys, err := r.ledger.GetAccountPublicKeys(accountAddress)
	if err != nil {
		return publicKey, err
	}

	if index < 0 || index > len(publicKeys)-1 {
		return publicKey, fmt.Errorf("invalid key index %d, account has %d keys", index, len(publicKeys))
	}

	removedKey := publicKeys[index]

	publicKeys = append(publicKeys[:index], publicKeys[index+1:]...)

	err = r.ledger.SetAccountPublicKeys(accountAddress, publicKeys)
	if err != nil {
		return publicKey, err
	}

	removedKeyBytes, err := flow.EncodeRuntimeAccountPublicKey(removedKey)
	if err != nil {
		return nil, fmt.Errorf("cannot encode removed runtime account key: %w", err)
	}
	return removedKeyBytes, nil
}

// CheckCode checks the code for its validity.
func (r *TransactionContext) CheckCode(address runtime.Address, code []byte) (err error) {
	return r.checkProgram(code, address)
}

// UpdateAccountCode updates the deployed code on an existing account.
//
// This function returns an error if the specified account does not exist or is
// not a valid signing account.
func (r *TransactionContext) UpdateAccountCode(address runtime.Address, code []byte, checkPermission bool) (err error) {
	accountAddress := address.Bytes()

	if checkPermission && !r.isValidSigningAccount(address) {
		return fmt.Errorf("not permitted to update account with ID %s", address)
	}

	err = r.ledger.CheckAccountExists(accountAddress)
	if err != nil {
		return err
	}

	r.ledger.Set(fullKeyHash(string(accountAddress), string(accountAddress), keyCode), code)

	return nil
}

// ResolveImport imports code for the provided import location.
//
// This function returns an error if the import location is not an account address,
// or if there is no code deployed at the specified address.
func (r *TransactionContext) ResolveImport(location runtime.Location) ([]byte, error) {
	addressLocation, ok := location.(runtime.AddressLocation)
	if !ok {
		return nil, fmt.Errorf("import location must be an account address")
	}

	address := flow.BytesToAddress(addressLocation)

	accountAddress := address.Bytes()

	code, err := r.ledger.Get(fullKeyHash(string(accountAddress), string(accountAddress), keyCode))
	if err != nil {
		return nil, err
	}

	if code == nil {
		return nil, fmt.Errorf("no code deployed at address %x", accountAddress)
	}

	return code, nil
}

// GetCachedProgram attempts to get a parsed program from a cache.
func (r *TransactionContext) GetCachedProgram(location ast.Location) (*ast.Program, error) {
	return r.astCache.GetProgram(location)
}

// CacheProgram adds a parsed program to a cache.
func (r *TransactionContext) CacheProgram(location ast.Location, program *ast.Program) error {
	return r.astCache.SetProgram(location, program)
}

// Log captures a log message from the runtime.
func (r *TransactionContext) Log(message string) {
	r.logs = append(r.logs, message)
}

// EmitEvent is called when an event is emitted by the runtime.
func (r *TransactionContext) EmitEvent(event cadence.Event) {
	r.events = append(r.events, event)
}

func (r *TransactionContext) GenerateUUID() uint64 {
	defer func() { r.uuid++ }()
	return r.uuid
}

func (r *TransactionContext) GetComputationLimit() uint64 {
	return r.gasLimit
}

func (r *TransactionContext) DecodeArgument(b []byte, t cadence.Type) (cadence.Value, error) {
	return jsoncdc.Decode(b)
}

func (r *TransactionContext) GetCurrentBlockHeight() uint64 {
	panic("implement me")
}

func (r *TransactionContext) GetBlockAtHeight(height uint64) (hash runtime.BlockHash, timestamp int64, exists bool) {
	panic("implement me")
}

// GetAccount gets an account by address.

func (r *TransactionContext) isValidSigningAccount(address runtime.Address) bool {
	for _, accountAddress := range r.GetSigningAccounts() {
		if accountAddress == address {
			return true
		}
	}

	return false
}

// checkProgram checks the given code for syntactic and semantic correctness.
func (r *TransactionContext) checkProgram(code []byte, address runtime.Address) error {
	if code == nil {
		return nil
	}

	location := runtime.AddressLocation(address[:])

	return r.checker(code, location)
}

// verifySignatures verifies that a transaction contains the necessary signatures.
//
// An error is returned if any of the expected signatures are invalid or missing.
func (r *TransactionContext) verifySignatures() FlowError {

	if r.tx.Payer == flow.ZeroAddress() {
		return &MissingPayerError{}
	}

	payloadWeights, proposalKeyVerifiedInPayload, err := r.aggregateAccountSignatures(
		r.tx.PayloadSignatures,
		r.tx.PayloadMessage(),
		r.tx.ProposalKey,
	)
	if err != nil {
		return err
	}

	envelopeWeights, proposalKeyVerifiedInEnvelope, err := r.aggregateAccountSignatures(
		r.tx.EnvelopeSignatures,
		r.tx.EnvelopeMessage(),
		r.tx.ProposalKey,
	)
	if err != nil {
		return err
	}

	proposalKeyVerified := proposalKeyVerifiedInPayload || proposalKeyVerifiedInEnvelope

	if !proposalKeyVerified {
		return &MissingSignatureForProposalKeyError{
			Address: r.tx.ProposalKey.Address,
			KeyID:   r.tx.ProposalKey.KeyID,
		}
	}

	for _, addr := range r.tx.Authorizers {
		// Skip this authorizer if it is also the payer. In the case where an account is
		// both a PAYER as well as an AUTHORIZER or PROPOSER, that account is required
		// to sign only the envelope.
		if addr == r.tx.Payer {
			continue
		}

		if !hasSufficientKeyWeight(payloadWeights, addr) {
			return &MissingSignatureError{addr}
		}
	}

	if !hasSufficientKeyWeight(envelopeWeights, r.tx.Payer) {
		return &MissingSignatureError{r.tx.Payer}
	}

	return nil
}

// CheckAndIncrementSequenceNumber validates and increments a sequence number for an account key.
//
// This function first checks that the provided sequence number matches the version stored on-chain.
// If they are equal, the on-chain sequence number is incremented.
// If they are not equal, the on-chain sequence number is not incremented.
//
// This function returns an error if any problem occurred during checking or the check failed
func (r *TransactionContext) checkAndIncrementSequenceNumber() (FlowError, error) {

	proposalKey := r.tx.ProposalKey

	account := r.ledger.GetAccount(proposalKey.Address)

	if int(proposalKey.KeyID) >= len(account.Keys) {
		return &InvalidProposalKeyError{
			Address: proposalKey.Address,
			KeyID:   proposalKey.KeyID,
		}, nil
	}

	accountKey := account.Keys[proposalKey.KeyID]

	valid := accountKey.SeqNumber == proposalKey.SequenceNumber

	if !valid {
		return &InvalidProposalSequenceNumberError{
			Address:           proposalKey.Address,
			KeyID:             proposalKey.KeyID,
			CurrentSeqNumber:  accountKey.SeqNumber,
			ProvidedSeqNumber: proposalKey.SequenceNumber,
		}, nil
	}

	accountKey.SeqNumber++

	updatedAccountBytes, err := flow.EncodeAccountPublicKey(accountKey)
	if err != nil {
		return nil, err
	}
	r.ledger.setAccountPublicKey(account.Address.Bytes(), proposalKey.KeyID, updatedAccountBytes)

	return nil, nil
}

func (r *TransactionContext) aggregateAccountSignatures(
	signatures []flow.TransactionSignature,
	message []byte,
	proposalKey flow.ProposalKey,
) (
	weights map[flow.Address]int,
	proposalKeyVerified bool,
	err FlowError,
) {
	weights = make(map[flow.Address]int)

	for _, txSig := range signatures {
		accountKey, err := r.verifyAccountSignature(txSig, message)
		if err != nil {
			return nil, false, err
		}

		if sigIsForProposalKey(txSig, proposalKey) {
			proposalKeyVerified = true
		}

		weights[txSig.Address] += accountKey.Weight
	}

	return
}

// verifyAccountSignature verifies that an account signature is valid for the
// account and given message.
//
// If the signature is valid, this function returns the associated account key.
//
// An error is returned if the account does not contain a public key that
// correctly verifies the signature against the given message.
func (r *TransactionContext) verifyAccountSignature(
	txSig flow.TransactionSignature,
	message []byte,
) (*flow.AccountPublicKey, FlowError) {
	account := r.ledger.GetAccount(txSig.Address)
	if account == nil {
		return nil, &InvalidSignatureAccountError{Address: txSig.Address}
	}

	if int(txSig.KeyID) >= len(account.Keys) {
		return nil, &InvalidSignatureAccountError{Address: txSig.Address}
	}

	accountKey := &account.Keys[txSig.KeyID]

	hasher, err := hash.NewHasher(accountKey.HashAlgo)
	if err != nil {
		return accountKey, &InvalidHashingAlgorithmError{
			Address:          txSig.Address,
			KeyID:            txSig.KeyID,
			HashingAlgorithm: accountKey.HashAlgo,
		}
	}

	valid, err := accountKey.PublicKey.Verify(txSig.Signature, message, hasher)
	if err != nil {
		return accountKey, &PublicKeyVerificationError{
			Address: txSig.Address,
			KeyID:   txSig.KeyID,
			Err:     err,
		}
	}

	if !valid {
		return accountKey, &InvalidSignaturePublicKeyError{Address: txSig.Address, KeyID: txSig.KeyID}
	}

	return accountKey, nil
}

func sigIsForProposalKey(txSig flow.TransactionSignature, proposalKey flow.ProposalKey) bool {
	return txSig.Address == proposalKey.Address && txSig.KeyID == proposalKey.KeyID
}

func hasSufficientKeyWeight(weights map[flow.Address]int, address flow.Address) bool {
	return weights[address] >= AccountKeyWeightThreshold
}
