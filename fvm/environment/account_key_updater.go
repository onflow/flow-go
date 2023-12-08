package environment

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"

	fgcrypto "github.com/onflow/crypto"
	fghash "github.com/onflow/crypto/hash"
	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// NewAccountPublicKey construct an account public key given a runtime
// public key.
func NewAccountPublicKey(publicKey *runtime.PublicKey,
	hashAlgo sema.HashAlgorithm,
	keyIndex int,
	weight int,
) (
	*flow.AccountPublicKey,
	error,
) {

	var err error
	signAlgorithm := crypto.RuntimeToCryptoSigningAlgorithm(publicKey.SignAlgo)
	if signAlgorithm != fgcrypto.ECDSAP256 &&
		signAlgorithm != fgcrypto.ECDSASecp256k1 {

		return nil, fmt.Errorf(
			"adding account key failed: %w",
			errors.NewValueErrorf(
				fmt.Sprintf("%d", publicKey.SignAlgo),
				"signature algorithm type not supported"))
	}

	hashAlgorithm := crypto.RuntimeToCryptoHashingAlgorithm(hashAlgo)
	if hashAlgorithm != fghash.SHA2_256 &&
		hashAlgorithm != fghash.SHA3_256 {

		return nil, fmt.Errorf(
			"adding account key failed: %w",
			errors.NewValueErrorf(
				fmt.Sprintf("%d", hashAlgo),
				"hashing algorithm type not supported"))
	}

	decodedPublicKey, err := fgcrypto.DecodePublicKey(
		signAlgorithm,
		publicKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf(
			"adding account key failed: %w",
			errors.NewValueErrorf(
				hex.EncodeToString(publicKey.PublicKey),
				"cannot decode public key: %w",
				err))
	}

	return &flow.AccountPublicKey{
		Index:     keyIndex,
		PublicKey: decodedPublicKey,
		SignAlgo:  signAlgorithm,
		HashAlgo:  hashAlgorithm,
		SeqNumber: 0,
		Weight:    weight,
		Revoked:   false,
	}, nil
}

// AccountKeyUpdater handles all account keys modification.
//
// Note that scripts cannot modify account keys, but must expose the API in
// compliance with the runtime environment interface.
type AccountKeyUpdater interface {
	// AddEncodedAccountKey adds an encoded public key to an existing account.
	//
	// This function returns an error if the specified account does not exist or
	// if the key insertion fails.
	//
	// Note that the script variant will return OperationNotSupportedError.
	AddEncodedAccountKey(runtimeAddress common.Address, publicKey []byte) error

	// RevokeEncodedAccountKey revokes a public key by index from an existing
	// account.
	//
	// This function returns an error if the specified account does not exist,
	// the provided key is invalid, or if key revoking fails.
	//
	// Note that the script variant will return OperationNotSupportedError.
	RevokeEncodedAccountKey(
		runtimeAddress common.Address,
		index int,
	) (
		[]byte,
		error,
	)

	// AddAccountKey adds a public key to an existing account.
	//
	// This function returns an error if the specified account does not exist or
	// if the key insertion fails.
	//
	// Note that the script variant will return OperationNotSupportedError.
	AddAccountKey(
		runtimeAddress common.Address,
		publicKey *runtime.PublicKey,
		hashAlgo runtime.HashAlgorithm,
		weight int,
	) (
		*runtime.AccountKey,
		error,
	)

	// RevokeAccountKey revokes a public key by index from an existing account,
	// and returns the revoked key.
	//
	// This function returns a nil key with no errors, if a key doesn't exist
	// at the given index.  An error is returned if the specified account does
	// not exist, the provided index is not valid, or if the key revoking
	// fails.
	//
	// Note that the script variant will return OperationNotSupportedError.
	RevokeAccountKey(
		runtimeAddress common.Address,
		keyIndex int,
	) (
		*runtime.AccountKey,
		error,
	)
}

type ParseRestrictedAccountKeyUpdater struct {
	txnState state.NestedTransactionPreparer
	impl     AccountKeyUpdater
}

func NewParseRestrictedAccountKeyUpdater(
	txnState state.NestedTransactionPreparer,
	impl AccountKeyUpdater,
) ParseRestrictedAccountKeyUpdater {
	return ParseRestrictedAccountKeyUpdater{
		txnState: txnState,
		impl:     impl,
	}
}

func (updater ParseRestrictedAccountKeyUpdater) AddEncodedAccountKey(
	runtimeAddress common.Address,
	publicKey []byte,
) error {
	return parseRestrict2Arg(
		updater.txnState,
		trace.FVMEnvAddEncodedAccountKey,
		updater.impl.AddEncodedAccountKey,
		runtimeAddress,
		publicKey)
}

func (updater ParseRestrictedAccountKeyUpdater) RevokeEncodedAccountKey(
	runtimeAddress common.Address,
	index int,
) (
	[]byte,
	error,
) {
	return parseRestrict2Arg1Ret(
		updater.txnState,
		trace.FVMEnvRevokeEncodedAccountKey,
		updater.impl.RevokeEncodedAccountKey,
		runtimeAddress,
		index)
}

func (updater ParseRestrictedAccountKeyUpdater) AddAccountKey(
	runtimeAddress common.Address,
	publicKey *runtime.PublicKey,
	hashAlgo runtime.HashAlgorithm,
	weight int,
) (
	*runtime.AccountKey,
	error,
) {
	return parseRestrict4Arg1Ret(
		updater.txnState,
		trace.FVMEnvAddAccountKey,
		updater.impl.AddAccountKey,
		runtimeAddress,
		publicKey,
		hashAlgo,
		weight)
}

func (updater ParseRestrictedAccountKeyUpdater) RevokeAccountKey(
	runtimeAddress common.Address,
	keyIndex int,
) (
	*runtime.AccountKey,
	error,
) {
	return parseRestrict2Arg1Ret(
		updater.txnState,
		trace.FVMEnvRevokeAccountKey,
		updater.impl.RevokeAccountKey,
		runtimeAddress,
		keyIndex)
}

type NoAccountKeyUpdater struct{}

func (NoAccountKeyUpdater) AddEncodedAccountKey(
	runtimeAddress common.Address,
	publicKey []byte,
) error {
	return errors.NewOperationNotSupportedError("AddEncodedAccountKey")
}

func (NoAccountKeyUpdater) RevokeEncodedAccountKey(
	runtimeAddress common.Address,
	index int,
) (
	[]byte,
	error,
) {
	return nil, errors.NewOperationNotSupportedError("RevokeEncodedAccountKey")
}

func (NoAccountKeyUpdater) AddAccountKey(
	runtimeAddress common.Address,
	publicKey *runtime.PublicKey,
	hashAlgo runtime.HashAlgorithm,
	weight int,
) (
	*runtime.AccountKey,
	error,
) {
	return nil, errors.NewOperationNotSupportedError("AddAccountKey")
}

func (NoAccountKeyUpdater) RevokeAccountKey(
	runtimeAddress common.Address,
	keyIndex int,
) (
	*runtime.AccountKey,
	error,
) {
	return nil, errors.NewOperationNotSupportedError("RevokeAccountKey")
}

type accountKeyUpdater struct {
	tracer tracing.TracerSpan
	meter  Meter

	accounts Accounts
	txnState state.NestedTransactionPreparer
	env      Environment
}

func NewAccountKeyUpdater(
	tracer tracing.TracerSpan,
	meter Meter,
	accounts Accounts,
	txnState state.NestedTransactionPreparer,
	env Environment,
) *accountKeyUpdater {
	return &accountKeyUpdater{
		tracer:   tracer,
		meter:    meter,
		accounts: accounts,
		txnState: txnState,
		env:      env,
	}
}

// AddAccountKey adds a public key to an existing account.
//
// This function returns an error if the specified account does not exist or
// if the key insertion fails.
func (updater *accountKeyUpdater) addAccountKey(
	address flow.Address,
	publicKey *runtime.PublicKey,
	hashAlgo runtime.HashAlgorithm,
	weight int,
) (
	*runtime.AccountKey,
	error,
) {
	ok, err := updater.accounts.Exists(address)
	if err != nil {
		return nil, fmt.Errorf("adding account key failed: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf(
			"adding account key failed: %w",
			errors.NewAccountNotFoundError(address))
	}

	keyIndex, err := updater.accounts.GetPublicKeyCount(address)
	if err != nil {
		return nil, fmt.Errorf("adding account key failed: %w", err)
	}

	accountPublicKey, err := NewAccountPublicKey(
		publicKey,
		hashAlgo,
		int(keyIndex),
		weight)
	if err != nil {
		return nil, fmt.Errorf("adding account key failed: %w", err)
	}

	err = updater.accounts.AppendPublicKey(address, *accountPublicKey)
	if err != nil {
		return nil, fmt.Errorf("adding account key failed: %w", err)
	}

	return &runtime.AccountKey{
		KeyIndex:  accountPublicKey.Index,
		PublicKey: publicKey,
		HashAlgo:  hashAlgo,
		Weight:    accountPublicKey.Weight,
		IsRevoked: accountPublicKey.Revoked,
	}, nil
}

// RevokeAccountKey revokes a public key by index from an existing account,
// and returns the revoked key.
//
// This function returns a nil key with no errors, if a key doesn't exist at
// the given index. An error is returned if the specified account does not
// exist, the provided index is not valid, or if the key revoking fails.
//
// TODO (ramtin) do we have to return runtime.AccountKey for this method or
// can be separated into another method
func (updater *accountKeyUpdater) revokeAccountKey(
	address flow.Address,
	keyIndex int,
) (
	*runtime.AccountKey,
	error,
) {
	ok, err := updater.accounts.Exists(address)
	if err != nil {
		return nil, fmt.Errorf("revoking account key failed: %w", err)
	}

	if !ok {
		return nil, fmt.Errorf(
			"revoking account key failed: %w",
			errors.NewAccountNotFoundError(address))
	}

	// Don't return an error for invalid key indices
	if keyIndex < 0 {
		return nil, nil
	}

	var publicKey flow.AccountPublicKey
	publicKey, err = updater.accounts.GetPublicKey(
		address,
		uint64(keyIndex))
	if err != nil {
		// If a key is not found at a given index, then return a nil key with
		// no errors.  This is to be inline with the Cadence runtime. Otherwise
		// Cadence runtime cannot distinguish between a 'key not found error'
		// vs other internal errors.
		if errors.IsAccountPublicKeyNotFoundError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("revoking account key failed: %w", err)
	}

	// mark this key as revoked
	publicKey.Revoked = true

	_, err = updater.accounts.SetPublicKey(
		address,
		uint64(keyIndex),
		publicKey)
	if err != nil {
		return nil, fmt.Errorf("revoking account key failed: %w", err)
	}

	// Prepare account key to return
	signAlgo := crypto.CryptoToRuntimeSigningAlgorithm(publicKey.SignAlgo)
	if signAlgo == runtime.SignatureAlgorithmUnknown {
		return nil, fmt.Errorf(
			"revoking account key failed: %w",
			errors.NewValueErrorf(
				publicKey.SignAlgo.String(),
				"signature algorithm type not found"))
	}

	hashAlgo := crypto.CryptoToRuntimeHashingAlgorithm(publicKey.HashAlgo)
	if hashAlgo == runtime.HashAlgorithmUnknown {
		return nil, fmt.Errorf(
			"revoking account key failed: %w",
			errors.NewValueErrorf(
				publicKey.HashAlgo.String(),
				"hashing algorithm type not found"))
	}

	return &runtime.AccountKey{
		KeyIndex: publicKey.Index,
		PublicKey: &runtime.PublicKey{
			PublicKey: publicKey.PublicKey.Encode(),
			SignAlgo:  signAlgo,
		},
		HashAlgo:  hashAlgo,
		Weight:    publicKey.Weight,
		IsRevoked: publicKey.Revoked,
	}, nil
}

// InternalAddEncodedAccountKey adds an encoded public key to an existing
// account.
//
// This function returns following error
// * NewAccountNotFoundError - if the specified account does not exist
// * ValueError - if the provided encodedPublicKey is not valid public key
func (updater *accountKeyUpdater) InternalAddEncodedAccountKey(
	address flow.Address,
	encodedPublicKey []byte,
) error {
	ok, err := updater.accounts.Exists(address)
	if err != nil {
		return fmt.Errorf("adding encoded account key failed: %w", err)
	}

	if !ok {
		return errors.NewAccountNotFoundError(address)
	}

	var publicKey flow.AccountPublicKey

	publicKey, err = flow.DecodeRuntimeAccountPublicKey(encodedPublicKey, 0)
	if err != nil {
		hexEncodedPublicKey := hex.EncodeToString(encodedPublicKey)
		return fmt.Errorf(
			"adding encoded account key failed: %w",
			errors.NewValueErrorf(
				hexEncodedPublicKey,
				"invalid encoded public key value: %w",
				err))
	}

	err = updater.accounts.AppendPublicKey(address, publicKey)
	if err != nil {
		return fmt.Errorf("adding encoded account key failed: %w", err)
	}

	return nil
}

// RemoveAccountKey revokes a public key by index from an existing account.
//
// This function returns an error if the specified account does not exist, the
// provided key is invalid, or if key revoking fails.
func (updater *accountKeyUpdater) removeAccountKey(
	address flow.Address,
	keyIndex int,
) (
	[]byte,
	error,
) {
	ok, err := updater.accounts.Exists(address)
	if err != nil {
		return nil, fmt.Errorf("remove account key failed: %w", err)
	}

	if !ok {
		issue := errors.NewAccountNotFoundError(address)
		return nil, fmt.Errorf("remove account key failed: %w", issue)
	}

	if keyIndex < 0 {
		err = errors.NewValueErrorf(
			fmt.Sprint(keyIndex),
			"key index must be positive")
		return nil, fmt.Errorf("remove account key failed: %w", err)
	}

	var publicKey flow.AccountPublicKey
	publicKey, err = updater.accounts.GetPublicKey(
		address,
		uint64(keyIndex))
	if err != nil {
		return nil, fmt.Errorf("remove account key failed: %w", err)
	}

	// mark this key as revoked
	publicKey.Revoked = true

	encodedPublicKey, err := updater.accounts.SetPublicKey(
		address,
		uint64(keyIndex),
		publicKey)
	if err != nil {
		return nil, fmt.Errorf("remove account key failed: %w", err)
	}

	return encodedPublicKey, nil
}

func (updater *accountKeyUpdater) AddEncodedAccountKey(
	runtimeAddress common.Address,
	publicKey []byte,
) error {
	defer updater.tracer.StartChildSpan(
		trace.FVMEnvAddEncodedAccountKey).End()

	err := updater.meter.MeterComputation(
		ComputationKindAddEncodedAccountKey,
		1)
	if err != nil {
		return fmt.Errorf("add encoded account key failed: %w", err)
	}

	address := flow.ConvertAddress(runtimeAddress)

	// TODO do a call to track the computation usage and memory usage
	//
	// don't enforce limit during adding a key
	updater.txnState.RunWithAllLimitsDisabled(func() {
		err = updater.InternalAddEncodedAccountKey(address, publicKey)
	})

	if err != nil {
		return fmt.Errorf("add encoded account key failed: %w", err)
	}
	return nil
}

func (updater *accountKeyUpdater) RevokeEncodedAccountKey(
	runtimeAddress common.Address,
	index int,
) (
	[]byte,
	error,
) {
	defer updater.tracer.StartChildSpan(trace.FVMEnvRevokeEncodedAccountKey).End()

	err := updater.meter.MeterComputation(
		ComputationKindRevokeEncodedAccountKey,
		1)
	if err != nil {
		return nil, fmt.Errorf("revoke encoded account key failed: %w", err)
	}

	address := flow.ConvertAddress(runtimeAddress)

	encodedKey, err := updater.removeAccountKey(address, index)
	if err != nil {
		return nil, fmt.Errorf("revoke encoded account key failed: %w", err)
	}

	return encodedKey, nil
}

func (updater *accountKeyUpdater) AddAccountKey(
	runtimeAddress common.Address,
	publicKey *runtime.PublicKey,
	hashAlgo runtime.HashAlgorithm,
	weight int,
) (
	*runtime.AccountKey,
	error,
) {
	defer updater.tracer.StartChildSpan(trace.FVMEnvAddAccountKey).End()

	err := updater.meter.MeterComputation(
		ComputationKindAddAccountKey,
		1)
	if err != nil {
		return nil, fmt.Errorf("add account key failed: %w", err)
	}

	accKey, err := updater.addAccountKey(
		flow.ConvertAddress(runtimeAddress),
		publicKey,
		hashAlgo,
		weight)
	if err != nil {
		return nil, fmt.Errorf("add account key failed: %w", err)
	}

	return accKey, nil
}

func (updater *accountKeyUpdater) RevokeAccountKey(
	runtimeAddress common.Address,
	keyIndex int,
) (
	*runtime.AccountKey,
	error,
) {
	defer updater.tracer.StartChildSpan(trace.FVMEnvRevokeAccountKey).End()

	err := updater.meter.MeterComputation(
		ComputationKindRevokeAccountKey,
		1)
	if err != nil {
		return nil, fmt.Errorf("revoke account key failed: %w", err)
	}

	return updater.revokeAccountKey(
		flow.ConvertAddress(runtimeAddress),
		keyIndex)
}
