package environment

import (
	"fmt"

	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// AccountKeyReader provide read access to account keys.
type AccountKeyReader interface {
	// GetAccountKey retrieves a public key by index from an existing account.
	//
	// This function returns a nil key with no errors, if a key doesn't exist at
	// the given index. An error is returned if the specified account does not
	// exist, the provided index is not valid, or if the key retrieval fails.
	GetAccountKey(
		address runtime.Address,
		keyIndex int,
	) (
		*runtime.AccountKey,
		error,
	)
}

type ParseRestrictedAccountKeyReader struct {
	txnState *state.TransactionState
	impl     AccountKeyReader
}

func NewParseRestrictedAccountKeyReader(
	txnState *state.TransactionState,
	impl AccountKeyReader,
) AccountKeyReader {
	return ParseRestrictedAccountKeyReader{
		txnState: txnState,
		impl:     impl,
	}
}

func (reader ParseRestrictedAccountKeyReader) GetAccountKey(
	address runtime.Address,
	keyIndex int,
) (
	*runtime.AccountKey,
	error,
) {
	return parseRestrict2Arg1Ret(
		reader.txnState,
		"GetAccountKey",
		reader.impl.GetAccountKey,
		address,
		keyIndex)
}

type accountKeyReader struct {
	tracer *Tracer
	meter  Meter

	accounts Accounts
}

func NewAccountKeyReader(
	tracer *Tracer,
	meter Meter,
	accounts Accounts,
) AccountKeyReader {
	return &accountKeyReader{
		tracer:   tracer,
		meter:    meter,
		accounts: accounts,
	}
}

func (reader *accountKeyReader) GetAccountKey(
	address runtime.Address,
	keyIndex int,
) (
	*runtime.AccountKey,
	error,
) {
	defer reader.tracer.StartSpanFromRoot(trace.FVMEnvGetAccountKey).End()

	err := reader.meter.MeterComputation(ComputationKindGetAccountKey, 1)
	if err != nil {
		return nil, fmt.Errorf("get account key failed: %w", err)
	}

	accountAddress := flow.Address(address)

	ok, err := reader.accounts.Exists(accountAddress)
	if err != nil {
		return nil, fmt.Errorf("getting account key failed: %w", err)
	}

	if !ok {
		issue := errors.NewAccountNotFoundError(accountAddress)
		return nil, fmt.Errorf("getting account key failed: %w", issue)
	}

	// Don't return an error for invalid key indices
	if keyIndex < 0 {
		return nil, nil
	}

	var publicKey flow.AccountPublicKey
	publicKey, err = reader.accounts.GetPublicKey(
		accountAddress,
		uint64(keyIndex))
	if err != nil {
		// If a key is not found at a given index, then return a nil key with
		// no errors.  This is to be inline with the Cadence runtime. Otherwise,
		// Cadence runtime cannot distinguish between a 'key not found error'
		// vs other internal errors.
		if errors.IsAccountAccountPublicKeyNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("getting account key failed: %w", err)
	}

	// Prepare the account key to return

	signAlgo := crypto.CryptoToRuntimeSigningAlgorithm(publicKey.SignAlgo)
	if signAlgo == runtime.SignatureAlgorithmUnknown {
		err = errors.NewValueErrorf(
			publicKey.SignAlgo.String(),
			"signature algorithm type not found")
		return nil, fmt.Errorf("getting account key failed: %w", err)
	}

	hashAlgo := crypto.CryptoToRuntimeHashingAlgorithm(publicKey.HashAlgo)
	if hashAlgo == runtime.HashAlgorithmUnknown {
		err = errors.NewValueErrorf(
			publicKey.HashAlgo.String(),
			"hashing algorithm type not found")
		return nil, fmt.Errorf("getting account key failed: %w", err)
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
