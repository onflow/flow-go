package environment

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
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
		runtimeAddress common.Address,
		keyIndex int,
	) (
		*runtime.AccountKey,
		error,
	)
	AccountKeysCount(runtimeAddress common.Address) (uint64, error)
}

type ParseRestrictedAccountKeyReader struct {
	txnState state.NestedTransactionPreparer
	impl     AccountKeyReader
}

func NewParseRestrictedAccountKeyReader(
	txnState state.NestedTransactionPreparer,
	impl AccountKeyReader,
) AccountKeyReader {
	return ParseRestrictedAccountKeyReader{
		txnState: txnState,
		impl:     impl,
	}
}

func (reader ParseRestrictedAccountKeyReader) GetAccountKey(
	runtimeAddress common.Address,
	keyIndex int,
) (
	*runtime.AccountKey,
	error,
) {
	return parseRestrict2Arg1Ret(
		reader.txnState,
		trace.FVMEnvGetAccountKey,
		reader.impl.GetAccountKey,
		runtimeAddress,
		keyIndex)
}

func (reader ParseRestrictedAccountKeyReader) AccountKeysCount(
	runtimeAddress common.Address,
) (
	uint64,
	error,
) {
	return parseRestrict1Arg1Ret(
		reader.txnState,
		"AccountKeysCount",
		reader.impl.AccountKeysCount,
		runtimeAddress,
	)
}

type accountKeyReader struct {
	tracer tracing.TracerSpan
	meter  Meter

	accounts Accounts
}

func NewAccountKeyReader(
	tracer tracing.TracerSpan,
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
	runtimeAddress common.Address,
	keyIndex int,
) (
	*runtime.AccountKey,
	error,
) {
	defer reader.tracer.StartChildSpan(trace.FVMEnvGetAccountKey).End()

	formatErr := func(err error) (*runtime.AccountKey, error) {
		return nil, fmt.Errorf("getting account key failed: %w", err)
	}

	err := reader.meter.MeterComputation(ComputationKindGetAccountKey, 1)
	if err != nil {
		return formatErr(err)
	}

	// Don't return an error for invalid key indices
	if keyIndex < 0 {
		return nil, nil
	}

	address := flow.ConvertAddress(runtimeAddress)

	// address verification is also done in this step
	accountPublicKey, err := reader.accounts.GetPublicKey(
		address,
		uint64(keyIndex))
	if err != nil {
		// If a key is not found at a given index, then return a nil key with
		// no errors.  This is to be inline with the Cadence runtime. Otherwise,
		// Cadence runtime cannot distinguish between a 'key not found error'
		// vs other internal errors.
		if errors.IsAccountPublicKeyNotFoundError(err) {
			return nil, nil
		}

		return formatErr(err)
	}

	// Prepare the account key to return
	runtimeAccountKey, err := FlowToRuntimeAccountKey(accountPublicKey)
	if err != nil {
		return formatErr(err)
	}

	return runtimeAccountKey, nil
}

func (reader *accountKeyReader) AccountKeysCount(
	runtimeAddress common.Address,
) (
	uint64,
	error,
) {
	defer reader.tracer.StartChildSpan(trace.FVMEnvAccountKeysCount).End()

	formatErr := func(err error) (uint64, error) {
		return 0, fmt.Errorf("fetching account key count failed: %w", err)
	}

	err := reader.meter.MeterComputation(ComputationKindAccountKeysCount, 1)
	if err != nil {
		return formatErr(err)
	}

	// address verification is also done in this step
	return reader.accounts.GetPublicKeyCount(
		flow.ConvertAddress(runtimeAddress))
}

func FlowToRuntimeAccountKey(
	flowKey flow.AccountPublicKey,
) (
	*runtime.AccountKey,
	error,
) {
	signAlgo := crypto.CryptoToRuntimeSigningAlgorithm(flowKey.SignAlgo)
	if signAlgo == runtime.SignatureAlgorithmUnknown {
		return nil, errors.NewValueErrorf(
			flowKey.SignAlgo.String(),
			"signature algorithm type not found",
		)
	}

	hashAlgo := crypto.CryptoToRuntimeHashingAlgorithm(flowKey.HashAlgo)
	if hashAlgo == runtime.HashAlgorithmUnknown {
		return nil, errors.NewValueErrorf(
			flowKey.HashAlgo.String(),
			"hashing algorithm type not found",
		)
	}

	publicKey := &runtime.PublicKey{
		PublicKey: flowKey.PublicKey.Encode(),
		SignAlgo:  signAlgo,
	}

	return &runtime.AccountKey{
		KeyIndex:  flowKey.Index,
		PublicKey: publicKey,
		HashAlgo:  hashAlgo,
		Weight:    flowKey.Weight,
		IsRevoked: flowKey.Revoked,
	}, nil
}
