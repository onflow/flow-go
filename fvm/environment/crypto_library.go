package environment

import (
	"fmt"

	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/state"
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
	txnState *state.TransactionState
	impl     CryptoLibrary
}

func NewParseRestrictedCryptoLibrary(
	txnState *state.TransactionState,
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
		"Hash",
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
		"VerifySignature",
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
		"ValidatePublicKey",
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
		"BLSVerifyPOP",
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
		"BLSAggregateSignatures",
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
		"BLSAggregatePublicKeys",
		lib.impl.BLSAggregatePublicKeys,
		keys)
}

// TODO(rbtz): add spans to the public functions
type cryptoLibrary struct {
	tracer *Tracer
	meter  Meter
}

func NewCryptoLibrary(tracer *Tracer, meter Meter) CryptoLibrary {
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
	defer lib.tracer.StartSpanFromRoot(trace.FVMEnvHash).End()

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
	defer lib.tracer.StartSpanFromRoot(trace.FVMEnvVerifySignature).End()

	err := lib.meter.MeterComputation(
		ComputationKindVerifySignature,
		1)
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
	err := lib.meter.MeterComputation(
		ComputationKindValidatePublicKey,
		1)
	if err != nil {
		return fmt.Errorf("validate public key failed: %w", err)
	}

	return crypto.ValidatePublicKey(pk.SignAlgo, pk.PublicKey)
}

func (cryptoLibrary) BLSVerifyPOP(
	pk *runtime.PublicKey,
	sig []byte,
) (
	bool,
	error,
) {
	return crypto.VerifyPOP(pk, sig)
}

func (cryptoLibrary) BLSAggregateSignatures(sigs [][]byte) ([]byte, error) {
	return crypto.AggregateSignatures(sigs)
}

func (cryptoLibrary) BLSAggregatePublicKeys(
	keys []*runtime.PublicKey,
) (
	*runtime.PublicKey,
	error,
) {

	return crypto.AggregatePublicKeys(keys)
}
