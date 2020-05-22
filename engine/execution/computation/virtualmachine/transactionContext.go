package virtualmachine

import (
	"encoding/hex"
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
	bc               BlockContext
	ledger           LedgerDAL
	astCache         ASTCache
	signingAccounts  []runtime.Address
	checker          CheckerFunc
	logs             []string
	events           []cadence.Event
	tx               *flow.TransactionBody
	gasLimit         uint64
	uuid             uint64 // TODO: implement proper UUID
	skipVerification bool
}

type TransactionContextOption func(*TransactionContext)

func SkipVerification(ctx *TransactionContext) {
	ctx.skipVerification = true
}

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
func (r *TransactionContext) CreateAccount(payer runtime.Address) (runtime.Address, error) {

	flowErr, fatalErr := r.deductAccountCreationFee(flow.Address(payer))
	if fatalErr != nil {
		return runtime.Address{}, fatalErr
	}

	if flowErr != nil {
		// TODO: properly propagate this error

		switch err := flowErr.(type) {
		case *CodeExecutionError:
			return runtime.Address{}, err.RuntimeError.Unwrap()
		default:
			// Account creation should fail due to insufficient balance, which is reported in `flowErr`.
			// Should we tree other FlowErrors as fatal?
			return runtime.Address{}, fmt.Errorf(
				"failed to deduct account creation fee: %s",
				err.ErrorMessage(),
			)
		}
	}

	var err error

	addr, err := r.ledger.CreateAccount(nil)
	if err != nil {
		return runtime.Address{}, err
	}

	flowErr, fatalErr = r.initDefaultToken(addr)
	if fatalErr != nil {
		return runtime.Address{}, fatalErr
	}

	if flowErr != nil {
		// TODO: properly propagate this error

		switch err := flowErr.(type) {
		case *CodeExecutionError:
			return runtime.Address{}, err.RuntimeError.Unwrap()
		default:
			return runtime.Address{}, fmt.Errorf(
				"failed to initialize default token: %s",
				err.ErrorMessage(),
			)
		}
	}

	r.Log(fmt.Sprintf("Created new account with address: 0x%s", addr))

	return runtime.Address(addr), nil
}

func (r *TransactionContext) initDefaultToken(addr flow.Address) (FlowError, error) {
	tx := flow.NewTransactionBody().
		SetScript(InitDefaultTokenTransaction).
		AddAuthorizer(addr)

	// TODO: propagate computation limit
	result, err := r.bc.ExecuteTransaction(r.ledger, tx, SkipVerification)
	if err != nil {
		return nil, err
	}

	if result.Error != nil {
		return result.Error, nil
	}

	return nil, nil
}

func (r *TransactionContext) deductTransactionFee(addr flow.Address) (FlowError, error) {
	tx := flow.NewTransactionBody().
		SetScript(DeductTransactionFeeTransaction).
		AddAuthorizer(addr)

	// TODO: propagate computation limit
	result, err := r.bc.ExecuteTransaction(r.ledger, tx, SkipVerification)
	if err != nil {
		return nil, err
	}

	if result.Error != nil {
		return result.Error, nil
	}

	return nil, nil
}

func (r *TransactionContext) deductAccountCreationFee(addr flow.Address) (FlowError, error) {
	tx := flow.NewTransactionBody().
		SetScript(DeductAccountCreationFeeTransaction).
		AddAuthorizer(addr)

	// TODO: propagate computation limit
	result, err := r.bc.ExecuteTransaction(r.ledger, tx, SkipVerification)
	if err != nil {
		return nil, err
	}

	if result.Error != nil {
		return result.Error, nil
	}

	return nil, nil
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

	// currently, every transaction that sets account code (deploys/updates contracts)
	// must be signed by the service account
	if !r.isValidSigningAccount(runtime.Address(flow.ServiceAddress())) {
		return fmt.Errorf("code deployment requires authorization from the service account")
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
	if r.skipVerification {
		return nil
	}

	if r.tx.Payer == flow.EmptyAddress {
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

// checkAndIncrementSequenceNumber validates and increments a sequence number for an account key.
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

var InitDefaultTokenTransaction = []byte(fmt.Sprintf(`
    import FlowServiceAccount from 0x%s

    transaction {
        prepare(acct: AuthAccount) {
            FlowServiceAccount.initDefaultToken(acct)
        }
    }
`, flow.ServiceAddress()))

func DefaultTokenBalanceScript(addr flow.Address) []byte {
	return []byte(fmt.Sprintf(`
        import FlowServiceAccount from 0x%s
    
        pub fun main(): UFix64 {
            let acct = getAccount(0x%s)
            return FlowServiceAccount.defaultTokenBalance(acct)
        }
    `, flow.ServiceAddress(), addr))
}

var DeductAccountCreationFeeTransaction = []byte(fmt.Sprintf(`
    import FlowServiceAccount from 0x%s

    transaction {
        prepare(acct: AuthAccount) {
			if !FlowServiceAccount.isAccountCreator(acct.address) {
				panic("Account not authorized to create accounts")
			}

            FlowServiceAccount.deductAccountCreationFee(acct)
        }
    }
`, flow.ServiceAddress()))

var DeductTransactionFeeTransaction = []byte(fmt.Sprintf(`
    import FlowServiceAccount from 0x%s

    transaction {
        prepare(acct: AuthAccount) {
            FlowServiceAccount.deductTransactionFee(acct)
        }
    }
`, flow.ServiceAddress()))

func DeployDefaultTokenTransaction(contract []byte) []byte {
	return []byte(fmt.Sprintf(`
        transaction {
          prepare(flowTokenAcct: AuthAccount, serviceAcct: AuthAccount) {
            let adminAcct = serviceAcct
            flowTokenAcct.setCode("%s".decodeHex(), adminAcct)
          }
        }
    `, hex.EncodeToString(contract)))
}

var MintDefaultTokenTransaction = []byte(fmt.Sprintf(`
    import FungibleToken from 0x%s
    import FlowToken from 0x%s

    transaction(amount: UFix64) {

      let tokenAdmin: &FlowToken.Administrator
      let tokenReceiver: &FlowToken.Vault{FungibleToken.Receiver}

      prepare(signer: AuthAccount) {
        self.tokenAdmin = signer
          .borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin) 
          ?? panic("Signer is not the token admin")

        self.tokenReceiver = signer
          .getCapability(/public/flowTokenReceiver)!
          .borrow<&FlowToken.Vault{FungibleToken.Receiver}>()
          ?? panic("Unable to borrow receiver reference for recipient")
      }

      execute {
        let minter <- self.tokenAdmin.createNewMinter(allowedAmount: amount)
        let mintedVault <- minter.mintTokens(amount: amount)

        self.tokenReceiver.deposit(from: <-mintedVault)
    
        destroy minter
      }
    }
`, FungibleTokenAddress, FlowTokenAddress))

// TODO: assign these values after bootstrapping
var FungibleTokenAddress, _, _ = flow.AccountAddress(flow.AddressState(1))
var FlowTokenAddress, _, _ = flow.AccountAddress(flow.AddressState(2))
