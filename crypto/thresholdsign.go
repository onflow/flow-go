// +build relic

package crypto

// #cgo CFLAGS: -g -Wall -std=c99
// #include "thresholdsign_include.h"
import "C"

import (
	"errors"
	"fmt"

	"github.com/dapperlabs/flow-go/crypto/hash"
)

// BLS-based threshold signature on BLS 12-381 curve
// The BLS settings are the same as in the signature
// scheme defined in the package.

// the threshold value (t) if fixed for now to t = floor((n-1)/2)
// where (t+1) signatures are required to reconstruct the
// threshold signature.
// FeldmanVSS, FeldmanVSSQual and JointFeldman are
// key generators for this threshold signature scheme

// The package offers two api:
// - stateful api where a structure holds all information
//  of the threshold signature protocols and is recommended
//  to be used for safety and to reduce protocol inconsistencies.
// - stateless api where only a to reconstruct the signature
// is provided. Verifying and storing the signature shares
// has to be done outside of the library.

// thresholdSigner is part of the stateful api
// It holds the data needed for threshold signaures
type thresholdSigner struct {
	// size of the group
	size int
	// the thresold t of the scheme where (t+1) shares are
	// required to reconstruct a signature
	threshold int
	// the index of the current node
	currentIndex int
	// the current node private key (a DKG output)
	currentPrivateKey PrivateKey
	// the group public key (a DKG output)
	groupPublicKey PublicKey
	// the group public key shares (a DKG output)
	publicKeyShares []PublicKey
	// the hasher to be used for all signatures
	hashAlgo hash.Hasher
	// the message to be signed. Siganture shares and the threshold signature
	// are verified using this message
	messageToSign []byte
	// the valid signature shares received from other nodes
	shares []byte // simulates an array of Signatures
	// (or a matrix of by bytes) to accommodate a cgo constraint
	// the list of signers corresponding to the list of shares
	signers []index
	// the threshold signature. It is equal to nil if less than (t+1) shares are
	// received
	thresholdSignature Signature
	// tempShare stores a temporary share that is staged but not committed yet
	// to the list of shares. tempOrig is the index related to tempShare
	tempShare Signature
	tempOrig  int
}

// NewThresholdSigner creates a new instance of Threshold signer using BLS
// hash is the hashing algorithm to be used
// size is the number of participants, threshold is the threshold value
func NewThresholdSigner(size int, threshold int, currentIndex int, hashAlgo hash.Hasher) (*thresholdSigner, error) {
	if size < ThresholdMinSize || size > ThresholdMaxSize {
		return nil, fmt.Errorf("size should be between %d and %d", ThresholdMinSize, ThresholdMaxSize)
	}
	if currentIndex >= size || currentIndex < 0 {
		return nil, fmt.Errorf("The current index must be between 0 and %d", size-1)
	}
	if threshold >= size || threshold < 0 {
		return nil, fmt.Errorf("The threshold must be between 0 and %d", size-1)
	}

	// initialize BLS settings
	_ = newBLSBLS12381()

	// internal list of valid signature shares
	shares := make([]byte, 0, (threshold+1)*SignatureLenBLSBLS12381)
	signers := make([]index, 0, threshold+1)

	return &thresholdSigner{
		size:               size,
		threshold:          threshold,
		currentIndex:       currentIndex,
		hashAlgo:           hashAlgo,
		shares:             shares,
		signers:            signers,
		thresholdSignature: nil,
		tempShare:          nil,
		tempOrig:           -1,
	}, nil
}

// SetKeys sets the private and public keys needed by the threshold signature
// - groupPublicKey is the group public key corresponding to the group secret key
// - sharePublicKeys are the public key shares corresponding to the nodes private
// key shares.
// - currentPrivateKey is the current node's own private key share
// Output keys of FeldmanVSS, FeldmanVSSQual and JointFeldman protocols could
// be used as the input keys to this function.
func (s *thresholdSigner) SetKeys(currentPrivateKey PrivateKey,
	groupPublicKey PublicKey,
	sharePublicKeys []PublicKey) {

	s.currentPrivateKey = currentPrivateKey
	s.groupPublicKey = groupPublicKey
	s.publicKeyShares = sharePublicKeys
}

// SetMessageToSign sets the next message to be signed
// All signatures shares of a different message are ignored
func (s *thresholdSigner) SetMessageToSign(message []byte) {
	s.ClearShares()
	s.messageToSign = message
}

// SignShare generates a signature share using the current private key share
func (s *thresholdSigner) SignShare() (Signature, error) {
	if s.currentPrivateKey == nil {
		return nil, errors.New("The private key of the current node is not set")
	}
	// sign
	share, err := s.currentPrivateKey.Sign(s.messageToSign, s.hashAlgo)
	if err != nil {
		return nil, fmt.Errorf("share signature failed: %w", err)
	}
	// add the node own signature
	valid, err := s.AddSignatureShare(s.currentIndex, share)
	if err != nil {
		return nil, fmt.Errorf("share signature failed: %w", err)
	}
	if !valid {
		return nil, errors.New("The current node private and public keys do not match")
	}
	return share, nil
}

// VerifyShare verifies a signature share using the signer's public key
func (s *thresholdSigner) verifyShare(share Signature, signerIndex index) (bool, error) {
	if len(s.publicKeyShares)-1 < int(signerIndex) {
		return false, errors.New("The node public keys are not set")
	}

	return s.publicKeyShares[signerIndex].Verify(share, s.messageToSign, s.hashAlgo)
}

// VerifyThresholdSignature verifies a threshold signature using the group public key
func (s *thresholdSigner) VerifyThresholdSignature(thresholdSignature Signature) (bool, error) {
	if s.groupPublicKey == nil {
		return false, errors.New("The group public key is not set")
	}
	return s.groupPublicKey.Verify(thresholdSignature, s.messageToSign, s.hashAlgo)
}

// ClearShares clears the shares and signers lists
func (s *thresholdSigner) ClearShares() {
	s.thresholdSignature = nil
	s.tempShare = nil
	s.tempOrig = -1
	s.signers = s.signers[:0]
	s.shares = s.shares[:0]
}

// EnoughShares checks whether there are enough shares to reconstruct a signature
func (s *thresholdSigner) EnoughShares() bool {
	// note: len(s.signers) is always <= s.threshold + 1
	return len(s.signers) == (s.threshold + 1)
}

func (s *thresholdSigner) emptyTempShare() {
	s.tempShare = nil
	s.tempOrig = -1
}

// AddSignatureShare adds a signature share to the local list of shares.
//
// If the share is valid, not perviously added and the threshold is not reached yet,
// it is appended to the local list of valid shares. This is equivelent to staging
// and commiting a share at the same step.
// The function returns true if the share is valid and new, false otherwise.
func (s *thresholdSigner) AddSignatureShare(orig int, share Signature) (bool, error) {
	// stage the share
	verif, err := s.VerifyStageShare(orig, share)
	if err != nil {
		return verif, fmt.Errorf("add signature failed: %w", err)
	}
	if verif {
		// commit the share
		err = s.CommitShare()
		if err != nil {
			return true, fmt.Errorf("add signature failed: %w", err)
		}
	}
	return verif, nil
}

// VerifyStageShare verifies a signature share without adding it to the local list of shares.
//
// If the share is valid and new, it is stored temporarily till is it committed to the shares
// list using CommitShare.
// Any call to VerifyStageShare, AddSignatureShare or CommitShare empties the temporary share.
// If the threshold is already reached, the VerifyStageShare still verifies the share.
// The function returns true if the share is valid and new, false otherwise.
func (s *thresholdSigner) VerifyStageShare(orig int, share Signature) (bool, error) {
	// empty the temporary share
	s.emptyTempShare()

	if orig >= s.size || orig < 0 {
		return false, errors.New("orig input is invalid")
	}

	verif, err := s.verifyShare(share, index(orig))
	if err != nil {
		return false, fmt.Errorf("signature share is invalid: %w", err)
	}

	// check if the share is new
	isSeen := false
	for _, e := range s.signers {
		if e == index(orig) {
			isSeen = true
			break
		}
	}

	// assign the temp share
	if verif && !isSeen {
		// store the share temporarily
		s.tempShare = share
		s.tempOrig = orig
		return true, nil
	}
	return false, nil
}

// CommitShare commits the temporary signature to the local list of shares if the threshold
// is not reached.
//
// The temprory share is stored by the latest call to VerifyStageShare. Calling CommitShare
// empties the temporary share.
func (s *thresholdSigner) CommitShare() error {
	if s.tempShare == nil || s.tempOrig == -1 {
		return fmt.Errorf("there is no signature to commit")
	}
	// append the temporary share
	if !s.EnoughShares() {
		s.shares = append(s.shares, s.tempShare...)
		s.signers = append(s.signers, index(s.tempOrig))
	}

	// empty the temporary share
	s.emptyTempShare()
	return nil
}

// ThresholdSignature returns the threshold signature if the threshold was reached, nil otherwise
func (s *thresholdSigner) ThresholdSignature() (Signature, error) {
	// thresholdSignature is only computed once
	if s.thresholdSignature != nil {
		return s.thresholdSignature, nil
	}
	// reconstruct the threshold signature
	if s.EnoughShares() {
		thresholdSignature, err := s.reconstructThresholdSignature()
		if err != nil {
			return nil, err
		}
		s.thresholdSignature = thresholdSignature
		return thresholdSignature, nil
	}
	return nil, errors.New("The number of signatures shares does not reach the threshold")
}

// ReconstructThresholdSignature reconstructs the threshold signature from at least (t+1) shares.
func (s *thresholdSigner) reconstructThresholdSignature() (Signature, error) {
	// sanity check
	if len(s.shares) != len(s.signers)*signatureLengthBLSBLS12381 {
		s.ClearShares()
		return nil, errors.New("The number of signature shares is not matching the number of signers")
	}
	thresholdSignature := make([]byte, signatureLengthBLSBLS12381)
	// Lagrange Interpolate at point 0
	if C.G1_lagrangeInterpolateAtZero(
		(*C.uchar)(&thresholdSignature[0]),
		(*C.uchar)(&s.shares[0]),
		(*C.uint8_t)(&s.signers[0]), (C.int)(len(s.signers)),
	) != valid {
		return nil, errors.New("reading signatures has failed")
	}

	// Verify the computed signature
	verif, err := s.VerifyThresholdSignature(thresholdSignature)
	if err != nil {
		return nil, fmt.Errorf("verify threshold signature failed: %w", err)
	}
	if !verif {
		return nil, errors.New(
			"The constructed threshold signature in incorrect. There might be an issue with the set keys")
	}

	return thresholdSignature, nil
}

// ReconstructThresholdSignature is a stateless api that takes a list of
// signatures and their signers's indices and returns the threshold signature.
// size is the size of the threshold signature group
// The function does not check the validity of the shares, and does not check
// the validity of the resulting signature. It also does not check the signatures signers
// are distinct.
// The function assumes the threshold value is equal to floor((n-1)/2)
// ReconstructThresholdSignature returns:
// - error if the inputs are not in the correct range or if the threshold is not reached
// - Signature: the threshold signature if there is no returned error, nil otherwise
func ReconstructThresholdSignature(size int, threshold int,
	shares []Signature, signers []int) (Signature, error) {
	// initialize BLS settings
	_ = newBLSBLS12381()
	if size < ThresholdMinSize || size > ThresholdMaxSize {
		return nil, fmt.Errorf("size should be between %d and %d",
			ThresholdMinSize, ThresholdMaxSize)
	}
	if threshold >= size || threshold < 0 {
		return nil, fmt.Errorf("The threshold must be between 0 and %d", size-1)
	}

	if len(shares) != len(signers) {
		return nil, errors.New("The number of signature shares is not matching the number of signers")
	}

	if len(shares) < threshold+1 {
		return nil, errors.New("The number of signatures does not reach the threshold")
	}

	// flatten the shares (required by the C layer)
	flatShares := make([]byte, 0, signatureLengthBLSBLS12381*(threshold+1))
	indexSigners := make([]index, 0, threshold+1)
	for i, share := range shares {
		flatShares = append(flatShares, share...)
		if signers[i] >= size || signers[i] < 0 {
			return nil, fmt.Errorf("signer index #%d is invalid", i)
		}
		indexSigners = append(indexSigners, index(signers[i]))
	}

	thresholdSignature := make([]byte, signatureLengthBLSBLS12381)
	// Lagrange Interpolate at point 0
	if C.G1_lagrangeInterpolateAtZero(
		(*C.uchar)(&thresholdSignature[0]),
		(*C.uchar)(&flatShares[0]),
		(*C.uint8_t)(&indexSigners[0]), (C.int)(threshold+1),
	) != valid {
		return nil, errors.New("reading signatures has failed")
	}
	return thresholdSignature, nil
}

// EnoughShares is a stateless function that takes the value of the threshold
// and a shares number and returns true if the shares number is enough
// to reconstruct a threshold signature.
func EnoughShares(threshold int, sharesNumber int) bool {
	return sharesNumber > threshold
}

// ThresholdSignKeyGen is a centralized key generation for a BLS-based
// threeshold signature scheme.
func ThresholdSignKeyGen(size int, threshold int, seed []byte) ([]PrivateKey,
	[]PublicKey, PublicKey, error) {
	if size < ThresholdMinSize || size > ThresholdMaxSize {
		return nil, nil, nil, fmt.Errorf("size should be between %d and %d", ThresholdMinSize, ThresholdMaxSize)
	}
	if threshold >= size || threshold < 0 {
		return nil, nil, nil, fmt.Errorf("The threshold must be between 0 and %d", size-1)
	}
	// initialize BLS settings
	_ = newBLSBLS12381()
	// the scalars x and G2 points y
	x := make([]scalar, size)
	y := make([]pointG2, size)
	var X0 pointG2
	// seed relic
	if err := seedRelic(seed); err != nil {
		return nil, nil, nil, err
	}
	// Generate a polynomial P in Zr[X] of degree t
	a := make([]scalar, threshold+1)
	randZrStar(&a[0])
	for i := 1; i < threshold+1; i++ {
		randZr(&a[i])
	}
	// compute the shares
	for i := index(1); int(i) <= size; i++ {
		C.Zr_polynomialImage(
			(*C.bn_st)(&x[i-1]),
			(*C.ep2_st)(&y[i-1]),
			(*C.bn_st)(&a[0]), (C.int)(len(a)),
			(C.uint8_t)(i),
		)
	}
	// group public key
	genScalarMultG2(&X0, &a[0])
	// export the keys
	skShares := make([]PrivateKey, size)
	pkShares := make([]PublicKey, size)
	var pkGroup PublicKey
	for i := 0; i < size; i++ {
		skShares[i] = &PrKeyBLSBLS12381{
			scalar: x[i],
		}
		pkShares[i] = &PubKeyBLSBLS12381{
			point: y[i],
		}
	}
	pkGroup = &PubKeyBLSBLS12381{
		point: X0,
	}
	return skShares, pkShares, pkGroup, nil
}
