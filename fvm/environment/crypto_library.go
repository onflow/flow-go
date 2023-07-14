package environment

import (
	"fmt"

	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/module/trace"
)

type CryptoLibrary interface {
	Hash(
		data []byte,
		tag string,
		hashAlgorithm runtime.HashAlgorithm,
	) (
		[]byte,
		error,
	)

	VerifySignature(
		signature []byte,
		tag string,
		signedData []byte,
		publicKey []byte,
		signatureAlgorithm runtime.SignatureAlgorithm,
		hashAlgorithm runtime.HashAlgorithm,
	) (
		bool,
		error,
	)

	ValidatePublicKey(pk *runtime.PublicKey) error

	BLSVerifyPOP(
		pk *runtime.PublicKey,
		sig []byte,
	) (
		bool,
		error,
	)

	BLSAggregateSignatures(sigs [][]byte) ([]byte, error)

	BLSAggregatePublicKeys(
		keys []*runtime.PublicKey,
	) (
		*runtime.PublicKey,
		error,
	)
}

type ParseRestrictedCryptoLibrary struct {
	txnState state.NestedTransactionPreparer
	impl     CryptoLibrary
}

func NewParseRestrictedCryptoLibrary(
	txnState state.NestedTransactionPreparer,
	impl CryptoLibrary,
) CryptoLibrary {
	return ParseRestrictedCryptoLibrary{
		txnState: txnState,
		impl:     impl,
	}
}

func (lib ParseRestrictedCryptoLibrary) Hash(
	data []byte,
	tag string,
	hashAlgorithm runtime.HashAlgorithm,
) (
	[]byte,
	error,
) {
	return parseRestrict3Arg1Ret(
		lib.txnState,
		trace.FVMEnvHash,
		lib.impl.Hash,
		data,
		tag,
		hashAlgorithm)
}

func (lib ParseRestrictedCryptoLibrary) VerifySignature(
	signature []byte,
	tag string,
	signedData []byte,
	publicKey []byte,
	signatureAlgorithm runtime.SignatureAlgorithm,
	hashAlgorithm runtime.HashAlgorithm,
) (
	bool,
	error,
) {
	return parseRestrict6Arg1Ret(
		lib.txnState,
		trace.FVMEnvVerifySignature,
		lib.impl.VerifySignature,
		signature,
		tag,
		signedData,
		publicKey,
		signatureAlgorithm,
		hashAlgorithm)
}

func (lib ParseRestrictedCryptoLibrary) ValidatePublicKey(
	pk *runtime.PublicKey,
) error {
	return parseRestrict1Arg(
		lib.txnState,
		trace.FVMEnvValidatePublicKey,
		lib.impl.ValidatePublicKey,
		pk)
}

func (lib ParseRestrictedCryptoLibrary) BLSVerifyPOP(
	pk *runtime.PublicKey,
	sig []byte,
) (
	bool,
	error,
) {
	return parseRestrict2Arg1Ret(
		lib.txnState,
		trace.FVMEnvBLSVerifyPOP,
		lib.impl.BLSVerifyPOP,
		pk,
		sig)
}

func (lib ParseRestrictedCryptoLibrary) BLSAggregateSignatures(
	sigs [][]byte,
) (
	[]byte,
	error,
) {
	return parseRestrict1Arg1Ret(
		lib.txnState,
		trace.FVMEnvBLSAggregateSignatures,
		lib.impl.BLSAggregateSignatures,
		sigs)
}

func (lib ParseRestrictedCryptoLibrary) BLSAggregatePublicKeys(
	keys []*runtime.PublicKey,
) (
	*runtime.PublicKey,
	error,
) {
	return parseRestrict1Arg1Ret(
		lib.txnState,
		trace.FVMEnvBLSAggregatePublicKeys,
		lib.impl.BLSAggregatePublicKeys,
		keys)
}

type cryptoLibrary struct {
	tracer tracing.TracerSpan
	meter  Meter
}

func NewCryptoLibrary(tracer tracing.TracerSpan, meter Meter) CryptoLibrary {
	return &cryptoLibrary{
		tracer: tracer,
		meter:  meter,
	}
}

func (lib *cryptoLibrary) Hash(
	data []byte,
	tag string,
	hashAlgorithm runtime.HashAlgorithm,
) (
	[]byte,
	error,
) {
	defer lib.tracer.StartChildSpan(trace.FVMEnvHash).End()

	err := lib.meter.MeterComputation(ComputationKindHash, 1)
	if err != nil {
		return nil, fmt.Errorf("hash failed: %w", err)
	}

	hashAlgo := crypto.RuntimeToCryptoHashingAlgorithm(hashAlgorithm)
	return crypto.HashWithTag(hashAlgo, tag, data)
}

func (lib *cryptoLibrary) VerifySignature(
	signature []byte,
	tag string,
	signedData []byte,
	publicKey []byte,
	signatureAlgorithm runtime.SignatureAlgorithm,
	hashAlgorithm runtime.HashAlgorithm,
) (
	bool,
	error,
) {
	defer lib.tracer.StartChildSpan(trace.FVMEnvVerifySignature).End()

	err := lib.meter.MeterComputation(ComputationKindVerifySignature, 1)
	if err != nil {
		return false, fmt.Errorf("verify signature failed: %w", err)
	}

	valid, err := crypto.VerifySignatureFromRuntime(
		signature,
		tag,
		signedData,
		publicKey,
		signatureAlgorithm,
		hashAlgorithm,
	)

	if err != nil {
		return false, fmt.Errorf("verify signature failed: %w", err)
	}

	return valid, nil
}

func (lib *cryptoLibrary) ValidatePublicKey(pk *runtime.PublicKey) error {
	defer lib.tracer.StartChildSpan(trace.FVMEnvValidatePublicKey).End()

	err := lib.meter.MeterComputation(ComputationKindValidatePublicKey, 1)
	if err != nil {
		return fmt.Errorf("validate public key failed: %w", err)
	}

	return crypto.ValidatePublicKey(pk.SignAlgo, pk.PublicKey)
}

func (lib *cryptoLibrary) BLSVerifyPOP(
	pk *runtime.PublicKey,
	sig []byte,
) (
	bool,
	error,
) {
	defer lib.tracer.StartChildSpan(trace.FVMEnvBLSVerifyPOP).End()

	err := lib.meter.MeterComputation(ComputationKindBLSVerifyPOP, 1)
	if err != nil {
		return false, fmt.Errorf("BLSVerifyPOP failed: %w", err)
	}

	return crypto.VerifyPOP(pk, sig)
}

func (lib *cryptoLibrary) BLSAggregateSignatures(
	sigs [][]byte,
) (
	[]byte,
	error,
) {
	defer lib.tracer.StartChildSpan(trace.FVMEnvBLSAggregateSignatures).End()

	err := lib.meter.MeterComputation(ComputationKindBLSAggregateSignatures, 1)
	if err != nil {
		return nil, fmt.Errorf("BLSAggregateSignatures failed: %w", err)
	}

	return crypto.AggregateSignatures(sigs)
}

func (lib *cryptoLibrary) BLSAggregatePublicKeys(
	keys []*runtime.PublicKey,
) (
	*runtime.PublicKey,
	error,
) {
	defer lib.tracer.StartChildSpan(trace.FVMEnvBLSAggregatePublicKeys).End()

	err := lib.meter.MeterComputation(ComputationKindBLSAggregatePublicKeys, 1)
	if err != nil {
		return nil, fmt.Errorf("BLSAggregatePublicKeys failed: %w", err)
	}

	return crypto.AggregatePublicKeys(keys)
}
