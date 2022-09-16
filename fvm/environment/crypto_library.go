package environment

import (
	"fmt"

	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/module/trace"
)

// TODO(rbtz): add spans to the public functions
type CryptoLibrary struct {
	tracer *Tracer
	meter  Meter
}

func NewCryptoLibrary(tracer *Tracer, meter Meter) *CryptoLibrary {
	return &CryptoLibrary{
		tracer: tracer,
		meter:  meter,
	}
}

func (lib *CryptoLibrary) Hash(
	data []byte,
	tag string,
	hashAlgorithm runtime.HashAlgorithm,
) ([]byte, error) {
	defer lib.tracer.StartSpanFromRoot(trace.FVMEnvHash).End()

	err := lib.meter.MeterComputation(ComputationKindHash, 1)
	if err != nil {
		return nil, fmt.Errorf("hash failed: %w", err)
	}

	hashAlgo := crypto.RuntimeToCryptoHashingAlgorithm(hashAlgorithm)
	return crypto.HashWithTag(hashAlgo, tag, data)
}

func (lib *CryptoLibrary) VerifySignature(
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

func (lib *CryptoLibrary) ValidatePublicKey(pk *runtime.PublicKey) error {
	err := lib.meter.MeterComputation(
		ComputationKindValidatePublicKey,
		1)
	if err != nil {
		return fmt.Errorf("validate public key failed: %w", err)
	}

	return crypto.ValidatePublicKey(pk.SignAlgo, pk.PublicKey)
}

func (CryptoLibrary) BLSVerifyPOP(
	pk *runtime.PublicKey,
	sig []byte,
) (
	bool,
	error) {
	return crypto.VerifyPOP(pk, sig)
}

func (CryptoLibrary) BLSAggregateSignatures(sigs [][]byte) ([]byte, error) {
	return crypto.AggregateSignatures(sigs)
}

func (CryptoLibrary) BLSAggregatePublicKeys(
	keys []*runtime.PublicKey,
) (*runtime.PublicKey, error) {

	return crypto.AggregatePublicKeys(keys)
}
