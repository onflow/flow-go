package fvm

import (
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
)

// This part should be part of the cadence runtime

// Runnable can be run by a runner (including transactions, system operations, or read only scripts)
// includes all the read-only data provided for the execution
type Runnable interface {
	// Source returns the cadence script to be executed
	Source() string
	// returns argument i and cast it to the given type
	// Ramtin: we might be able to change this to just return values and types be in the encoding
	Argument(i uint, t cadence.Type) (cadence.Value, error)
	// ComputationLimit returns the max computation limit allowed while running
	// Ramtin: (we might not need this to be passed and just be enforced in the Results)
	ComputationLimit() uint64
	// IsAuthorizer returns true if the address is an authorizer of this transaction
	IsAuthorizer(address runtime.Address) bool
	// Authorizers returns a list address who authorized this script
	// TODO ideally we should only use IsAuthorizer for authorization, this way
	// we can return true for the system operations and return false for all the read-only scripts
	// Ramtin: Similarly this might not be passed to cadence and be enforced in the accounts
	Authorizers() []runtime.Address
}

// Runner runs a "Runnable" and stores result into "Results".
// It returns fatal errors
// A runner augments the Cadence runtime with the host functionality.
//
// note that non-fatal runtime errors are captured inside the result
// and error should only be used for returning fatal errors.
// TODO add injectable methods
type Runner interface {
	Run(Runnable, Accounts, Results, Metrics) error
	RunScript(Runnable, Accounts, Results, Metrics) (cadence.Value, error)
}

// Questions?
// should accounts, results, metrics be part of context?
// where should we put authorizers()  - read only content
// maybe put the encoding to outside of the cadence ?

// coverageReport should be part of vm creation process and not a method of FVM

// TODO Account meta data instead of exists register
// 		suspend info, storage info, keys, ...

type Accounts interface {

	// creates a new account address and set the exists flag for this account
	CreateAccount(caller common.Location) (address Address, err error)
	// Exists returns true if the account exists
	Exists(address runtime.Address) (exists bool, err error)
	// SuspendAccount suspends an account
	SuspendAccount(address runtime.Address, caller common.Location) error
	// UnsuspendAccount unsuspend an account
	UnsuspendAccount(address runtime.Address, caller common.Location) error

	// TODO ramtin merge this with 	Code(location Location) error
	// AccountContractCode returns the code associated with an account contract.
	ContractCode(address common.AddressLocation) (code []byte, err error)
	// UpdateAccountContractCode updates the code associated with an account contract.
	UpdateContractCode(address common.AddressLocation, code []byte, caller common.Location) (err error)
	// RemoveContractCode removes the code associated with an account contract.
	RemoveContractCode(address common.AddressLocation, caller common.Location) (err error)

	// AddAccountKey appends a key to an account.
	AddAccountKey(address runtime.Address, publicKey []byte, caller common.Location) error
	// RemoveAccountKey removes a key from an account by index.
	RevokeAccountKey(address runtime.Address, index uint, caller common.Location) (publicKey []byte, err error)
	// TODO RAMTIN do I need the tag here?
	VerifyAccountSignature(address runtime.Address, index uint, signature []byte, tag string, signedData []byte) (bool, error)

	// StoredKeys returns list of keys and their sizes owned by this account
	StoredKeys(caller common.AddressLocation)
	// StorageUsed gets storage used in bytes by the address at the moment of the function call.
	StorageUsed(address runtime.Address, caller common.AddressLocation) (value uint64, err error)
	// StorageCapacity gets storage capacity in bytes on the address.
	StorageCapacity(address Address, caller common.AddressLocation) (value uint64, err error) // Do we need this?

	// Value gets a value for the given key in the storage, owned by the given account.
	Value(owner, key []byte, caller common.AddressLocation) (value []byte, err error)
	// SetValue sets a value for the given key in the storage, owned by the given account.
	SetValue(owner, key, value []byte, caller common.AddressLocation) (err error)
	// ValueExists returns true if the given key exists in the storage, owned by the given account.
	ValueExists(owner, key []byte, caller common.AddressLocation) (exists bool, err error)
}

type Results interface {
	AppendLog(string) error
	Logs() ([]string, error)

	AppendEvent(cadence.Event) error
	Events() ([]cadence.Event, error)

	AppendError(error)
	Errors() multierror.Error

	AddComputationUsed(uint64)
	ComputationSpent() uint64
}

type Metrics interface {
	ProgramParsed(location common.Location, duration time.Duration)
	ProgramChecked(location common.Location, duration time.Duration)
	ProgramInterpreted(location common.Location, duration time.Duration)

	ValueEncoded(duration time.Duration)
	ValueDecoded(duration time.Duration)
}
