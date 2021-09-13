// +build relic

package signature

import (
	"fmt"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	//"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module"
)

// SignatureAggregatorSameMessage aggregates BLS signatures of the same message from different signers.
// The public keys and message are aggreed upon upfront.
//
// Currently, the module does not support signatures with multiplicity higher than 1. Each signer is allowed
// to sign at most once.
//
// Aggregation uses BLS scheme. Mitigation against rogue attacks is done using Proof Of Possession (PoP)
// This module does not verify PoPs of input public keys, it assumes verification was done outside this module.
//
// Implementation of SignatureAggregator is not thread-safe, the caller should
// make sure the calls are concurrent safe.
type SignatureAggregatorSameMessage struct {
	// TODO: initial incomplete fields that will evolve
	message       []byte
	hasher        hash.Hasher
	n             int                      // number of participants indexed from 0 to n-1
	publicKeys    []crypto.PublicKey       // keys indexed from 0 to n-1, signer i is assigned to public key i
	idToSignature map[int]crypto.Signature // signatures indexed by the signer index
}

// NewSignatureAggregatorSameMessage returns a new SignatureAggregatorSameMessage structure.
//
// A new SignatureAggregatorSameMessage is needed for each set of public keys. If the key set changes,
// a new structure needs to be instantiated. Participants are defined by their public keys, and are
// indexed from 0 to n-1 where n is the length of the public key slice.
// The function errors with engine.InvalidInputError if any input is invalid.
func NewSignatureAggregatorSameMessage(
	message []byte, // message to be aggregate signatures for
	dsTag string, // domain separation tag used for signatures
	publicKeys []crypto.PublicKey, // public keys of participants agreed upon upfront
) (*SignatureAggregatorSameMessage, error) {
	panic("implement me")
	return &SignatureAggregatorSameMessage{}, nil
}

// Verify verifies the input signature under the stored message and stored
// key at the input index.
//
// This function does not update the internal state.
// The function errors:
//  - engine.InvalidInputErrorf if the index input is invalid
//  - random error if the execution failed
// The function does not return an error for any invalid signature.
// The function is not thread-safe.
func (s *SignatureAggregatorSameMessage) Verify(signer int, sig crypto.Signature) (bool, error) {
	panic("implement me")
}

// VerifyAndAdd verifies the input signature under the stored message and stored
// key at the input index. If the verification passes, the signature is added to the internal
// signature state.
// The function errors:
//  - engine.InvalidInputErrorf if the index input is invalid
//  - ErrDuplicatedSigner if the signer has been already added
//  - random error if the execution failed
// The function does not return an error for any invalid signature.
// If no error is returned, the bool represents the validity of the signature.
// The function is not thread-safe.
func (s *SignatureAggregatorSameMessage) VerifyAndAdd(signer int, sig crypto.Signature) (bool, error) {
	panic("implement me")
}

// TrustedAdd adds a signature to the internal state without verifying it.
//
// The Aggregate function makes a sanity check on the aggregated signature and only
// outputs valid signatures. This would detect if TrustedAdd has added any invalid
// signature.
// The function errors:
//  - engine.InvalidInputErrorf if the index input is invalid
//  - ErrDuplicatedSigner if the signer has been already added
//  - random error if the execution failed
// The function is not thread-safe.
func (s *SignatureAggregatorSameMessage) TrustedAdd(signer int, sig crypto.Signature) error {
	panic("implement me")
}

// HasSignature checks if a signer has already provided a valid signature.
//
// The function errors:
//  - engine.InvalidInputError if the index input is invalid
//  - random error if the execution failed
// The function is not thread-safe.
func (s *SignatureAggregatorSameMessage) HasSignature(signer int) (bool, error) {
	panic("implement me")
}

// Aggregate aggregates the stored BLS signatures and returns the aggregated signature.
//
// Aggregate attempts to aggregate the internal signatures and returns the resulting signature.
// The function performs a final verification and errors if any signature fails the desrialization
// or if the aggregated signature is not valid.
// required for the function safety since "TrustedAdd" allows adding invalid signatures.
// The function is not thread-safe.
//
// TODO : When compacting the list of signers, update the return from []int
// to a compact bit vector.
func (s *SignatureAggregatorSameMessage) Aggregate() ([]int, crypto.Signature, error) {
	panic("implement me")
}

// VerifyAggregate verifies an aggregated signature against the stored message and the stored
// keys corresponding to the input signers.
// Aggregating the keys of the signers internally is optimized to only look at the keys delta
// compared to the latest execution of the function. The function is therefore not thread-safe.
// The function errors:
//  - engine.InvalidInputErrorf if the indices are invalid
//  - random error if the execution failed
func (s *SignatureAggregatorSameMessage) VerifyAggregate(sig crypto.Signature, signers []int) (bool, error) {
	panic("implement me")
}

// TODO : to delete in V2
// AggregationVerifier is an aggregating verifier that can verify signatures and
// verify aggregated signatures of the same message.
// *Important: the aggregation verifier can only verify signatures in the context
// of the provided KMAC tag.
type AggregationVerifier struct {
	hasher hash.Hasher
}

// TODO : to delete in V2
// NewAggregationVerifier creates a new aggregation verifier, which can only
// verify signatures. *Important*: the aggregation verifier can only verify
// signatures in the context of the provided KMAC tag.
func NewAggregationVerifier(tag string) *AggregationVerifier {
	av := &AggregationVerifier{
		hasher: crypto.NewBLSKMAC(tag),
	}
	return av
}

// TODO : to delete in V2
// Verify will verify the given signature against the given message and public key.
func (av *AggregationVerifier) Verify(msg []byte, sig crypto.Signature, key crypto.PublicKey) (bool, error) {
	return key.Verify(sig, msg, av.hasher)
}

// TODO : to delete in V2
// VerifyMany will verify the given aggregated signature against the given message and the
// provided public keys.
func (av *AggregationVerifier) VerifyMany(msg []byte, sig crypto.Signature, keys []crypto.PublicKey) (bool, error) {

	// bls multi-signature verification with a single message
	valid, err := crypto.VerifyBLSSignatureOneMessage(keys, sig, msg, av.hasher)
	if err != nil {
		return false, fmt.Errorf("could not verify aggregated signature: %w", err)
	}

	return valid, nil
}

// TODO : to delete in V2
// AggregationProvider is an aggregating signer and verifier that can create/verify
// signatures, as well as aggregating & verifying aggregated signatures.
// *Important*: the aggregation verifier can only verify signatures in the context
// of the provided KMAC tag.
type AggregationProvider struct {
	*AggregationVerifier
	local module.Local
}

// TODO : to delete in V2
// NewAggregationProvider creates a new aggregation provider using the given private
// key to generate signatures. *Important*: the aggregation provider can only
// create and verify signatures in the context of the provided HMAC tag.
func NewAggregationProvider(tag string, local module.Local) *AggregationProvider {
	ap := &AggregationProvider{
		AggregationVerifier: NewAggregationVerifier(tag),
		local:               local,
	}
	return ap
}

// TODO : to delete in V2
// Sign will sign the given message bytes with the internal private key and
// return the signature on success.
func (ap *AggregationProvider) Sign(msg []byte) (crypto.Signature, error) {
	return ap.local.Sign(msg, ap.hasher)
}

// TODO : to delete in V2
// Aggregate will aggregate the given signatures into one aggregated signature.
func (ap *AggregationProvider) Aggregate(sigs []crypto.Signature) (crypto.Signature, error) {

	// BLS aggregation
	sig, err := crypto.AggregateBLSSignatures(sigs)
	if err != nil {
		return nil, fmt.Errorf("could not aggregate BLS signatures: %w", err)
	}
	return sig, nil
}
