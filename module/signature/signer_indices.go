package signature

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/ledger/common/bitutils"
	"github.com/onflow/flow-go/model/flow"
)

// EncodeSignerToIndicesAndSigType encodes the given stakingSigners and beaconSigners into bit vectors for
// signer indices and sig types.
// PREREQUISITES:
//   - The input `canonicalIdentifiers` must exhaustively list the set of authorized signers in their canonical order.
//   - The inputs `stakingSigners` and `beaconSigners` are treated as sets, i.e. they
//     should not contain any duplicates.
//   - A node can be listed in either `stakingSigners` or `beaconSigners`. A node appearing in both lists
//     constitutes an illegal input.
//   - `stakingSigners` must be a subset of `canonicalIdentifiers`
//   - `beaconSigners` must be a subset of `canonicalIdentifiers`
//
// RETURN VALUES:
//
//   - `signerIndices` is a bit vector. Let signerIndices[i] denote the ith bit of `signerIndices`.
//
//     .                            ┌ 1 if and only if canonicalIdentifiers[i] is in `stakingSigners` or `beaconSigners`
//     .         signerIndices[i] = └ 0 otherwise
//
//     Let `n` be the length of `canonicalIdentifiers`. `signerIndices` contains at least `n` bits, though, we
//     right-pad it with tailing zeros to full bytes.
//
//   - `sigTypes` is a bit vector. Let sigTypes[i] denote the ith bit of `sigTypes`
//     .                       ┌ 1 if and only if the ith signer is in `beaconSigners`
//     .         sigTypes[i] = └ 0 if and only if the ith signer is in `stakingSigners`
//     (Per prerequisite, we require that no signer is listed in both `beaconSigners` and `stakingSigners`)
//
// Example:
// As an example consider the case where we have a committee C of 10 nodes in canonical oder
//
//	C = [A,B,C,D,E,F,G,H,I,J]
//
// where nodes [B,F] are stakingSigners and beaconSigners are [C,E,G,I,J].
//  1. First return parameter: `signerIndices`
//     - We start with a bit vector v that has |C| number of bits
//     - If a node contributed either as staking signer or beacon signer,
//     we set the respective bit to 1:
//     .         [A,B,C,D,E,F,G,H,I,J]
//     .            ↓ ↓   ↓ ↓ ↓   ↓ ↓
//     .          0,1,1,0,1,1,1,0,1,1
//     - Lastly, right-pad the resulting bit vector with 0 to full bytes. We have 10 committee members,
//     so we pad to 2 bytes:
//     .          01101110 11000000
//  2. second return parameter: `sigTypes`
//     - Here, we restrict our focus on the signers, which we encoded in the previous step.
//     In our example, nodes [B,C,E,F,G,I,J] signed in canonical order. This is exactly the same order,
//     as we have represented the signer in the last step.
//     - For these 5 nodes in their canonical order, we encode each node's signature type as
//     .        bit-value 1: node was in beaconSigners
//     .        bit-value 0: node was in stakingSigners
//     This results in the bit vector
//     .            [B,C,E,F,G,I,J]
//     .             ↓ ↓ ↓ ↓ ↓ ↓ ↓
//     .             0,1,0,1,1,1,1
//     - Again, we right-pad with zeros to full bytes, As we only had 7 signers, the sigType slice is 1byte long
//     .            01011110
//
// the signer indices is prefixed with a checksum of the canonicalIdentifiers, which can be used by the decoder
// to verify if the decoder is using the same canonicalIdentifiers as the encoder to decode the signer indices.
//
// ERROR RETURNS
// During normal operations, no error returns are expected. This is because encoding signer sets is generally
// part of the node's internal work to generate messages. Hence, the inputs to this method come from other
// trusted components within the node. Therefore, any illegal input is treated as a symptom of an internal bug.
func EncodeSignerToIndicesAndSigType(
	canonicalIdentifiers flow.IdentifierList,
	stakingSigners flow.IdentifierList,
	beaconSigners flow.IdentifierList,
) (signerIndices []byte, sigTypes []byte, err error) {
	stakingSignersLookup := stakingSigners.Lookup()
	if len(stakingSignersLookup) != len(stakingSigners) {
		return nil, nil, fmt.Errorf("duplicated entries in staking signers %v", stakingSignersLookup)
	}
	beaconSignersLookup := beaconSigners.Lookup()
	if len(beaconSignersLookup) != len(beaconSigners) {
		return nil, nil, fmt.Errorf("duplicated entries in beacon signers %v", stakingSignersLookup)
	}

	// encode Identifiers to `signerIndices`; and for each signer, encode the signature type in `sigTypes`
	signerIndices = bitutils.MakeBitVector(len(canonicalIdentifiers))
	sigTypes = bitutils.MakeBitVector(len(stakingSigners) + len(beaconSigners))
	signerCounter := 0
	for canonicalIdx, member := range canonicalIdentifiers {
		if _, ok := stakingSignersLookup[member]; ok {
			bitutils.SetBit(signerIndices, canonicalIdx)
			// The default value for sigTypes is bit zero, which corresponds to a staking sig.
			// Hence, we don't have to change anything here.
			delete(stakingSignersLookup, member)
			signerCounter++
			continue
		}
		if _, ok := beaconSignersLookup[member]; ok {
			bitutils.SetBit(signerIndices, canonicalIdx)
			bitutils.SetBit(sigTypes, signerCounter)
			delete(beaconSignersLookup, member)
			signerCounter++
			continue
		}
	}

	if len(stakingSignersLookup) > 0 {
		return nil, nil, fmt.Errorf("unknown staking signers %v", stakingSignersLookup)
	}
	if len(beaconSignersLookup) > 0 {
		return nil, nil, fmt.Errorf("unknown or duplicated beacon signers %v", beaconSignersLookup)
	}

	prefixed := PrefixCheckSum(canonicalIdentifiers, signerIndices)

	return prefixed, sigTypes, nil
}

// DecodeSigTypeToStakingAndBeaconSigners decodes the bit-vector `sigType` to the set of
// staking signer identities (`stakingSigners`) and the set of beacon signer identities (`beaconSigners`).
// Prerequisite:
//   - The input `signers` must be the set of signers in their canonical order.
//
// Expected Error returns during normal operations:
//   - signature.IsInvalidSigTypesError if the given `sigType` does not encode a valid sequence of signature types
func DecodeSigTypeToStakingAndBeaconSigners(
	signers flow.IdentitySkeletonList,
	sigType []byte,
) (flow.IdentitySkeletonList, flow.IdentitySkeletonList, error) {
	numberSigners := len(signers)
	if err := validPadding(sigType, numberSigners); err != nil {
		if errors.Is(err, ErrIncompatibleBitVectorLength) || errors.Is(err, ErrIllegallyPaddedBitVector) {
			return nil, nil, NewInvalidSigTypesErrorf("invalid padding of sigTypes: %w", err)
		}
		return nil, nil, fmt.Errorf("unexpected exception while checking padding of sigTypes: %w", err)
	}

	// decode bits to IdentitySkeletonList
	stakingSigners := make(flow.IdentitySkeletonList, 0, numberSigners)
	beaconSigners := make(flow.IdentitySkeletonList, 0, numberSigners)
	for i, signer := range signers {
		if bitutils.ReadBit(sigType, i) == 0 {
			stakingSigners = append(stakingSigners, signer)
		} else {
			beaconSigners = append(beaconSigners, signer)
		}
	}
	return stakingSigners, beaconSigners, nil
}

// EncodeSignersToIndices encodes the given signerIDs into compacted bit vector.
// PREREQUISITES:
//   - The input `canonicalIdentifiers` must exhaustively list the set of authorized signers in their canonical order.
//   - The input `signerIDs` represents a set, i.e. it should not contain any duplicates.
//   - `signerIDs` must be a subset of `canonicalIdentifiers`
//   - `signerIDs` can be in arbitrary order (canonical order _not required_)
//
// RETURN VALUE:
//   - `signerIndices` is a bit vector. Let signerIndices[i] denote the ith bit of `signerIndices`.
//     .                             ┌ 1 if and only if canonicalIdentifiers[i] is in `signerIDs`
//     .          signerIndices[i] = └ 0 otherwise
//     Let `n` be the length of `canonicalIdentifiers`. `signerIndices` contains at least `n` bits, though, we
//     right-pad it with tailing zeros to full bytes.
//
// Example:
// As an example consider the case where we have a committee C of 10 nodes in canonical oder
//
//	C = [A,B,C,D,E,F,G,H,I,J]
//
// where nodes [B,F] are stakingSigners, and beaconSigners are [C,E,G,I,J].
//  1. First return parameter: QC.signerIndices
//     - We start with a bit vector v that has |C| number of bits
//     - If a node contributed either as staking signer or beacon signer,
//     we set the respective bit to 1:
//     .          [A,B,C,D,E,F,G,H,I,J]
//     .             ↓ ↓   ↓ ↓ ↓   ↓ ↓
//     .           0,1,1,0,1,1,1,0,1,1
//     - Lastly, right-pad the resulting bit vector with 0 to full bytes. We have 10 committee members,
//     so we pad to 2 bytes:
//     .           01101110 11000000
//
// ERROR RETURNS
// During normal operations, no error returns are expected. This is because encoding signer sets is generally
// part of the node's internal work to generate messages. Hence, the inputs to this method come from other
// trusted components within the node. Therefore, any illegal input is treated as a symptom of an internal bug.
// canonicalIdentifiers represents all identities who are eligible to sign the given resource. It excludes
// identities who are ineligible to sign the given resource. For example, canonicalIdentifiers in the context
// of a cluster consensus quorum certificate would include authorized members of the cluster and
// exclude ejected members of the cluster, or unejected collection nodes from a different cluster.
// the signer indices is prefixed with a checksum of the canonicalIdentifiers, which can be used by the decoder
// to verify if the decoder is using the same canonicalIdentifiers as the encoder to decode the signer indices.
func EncodeSignersToIndices(
	canonicalIdentifiers flow.IdentifierList,
	signerIDs flow.IdentifierList,
) (signerIndices []byte, err error) {
	signersLookup := signerIDs.Lookup()
	if len(signersLookup) != len(signerIDs) {
		return nil, fmt.Errorf("duplicated entries in signerIDs %v", signerIDs)
	}

	// encode Identifiers to bits
	signerIndices = bitutils.MakeBitVector(len(canonicalIdentifiers))
	for canonicalIdx, member := range canonicalIdentifiers {
		if _, ok := signersLookup[member]; ok {
			bitutils.SetBit(signerIndices, canonicalIdx)
			delete(signersLookup, member)
		}
	}
	if len(signersLookup) > 0 {
		return nil, fmt.Errorf("unknown signers IDs in the keys of %v", signersLookup)
	}

	prefixed := PrefixCheckSum(canonicalIdentifiers, signerIndices)

	return prefixed, nil
}

// DecodeSignerIndicesToIdentifiers decodes the given compacted bit vector into signerIDs
// Prerequisite:
//   - The input `canonicalIdentifiers` must exhaustively list the set of authorized signers in their canonical order.
//
// Expected Error returns during normal operations:
// * signature.InvalidSignerIndicesError if the given index vector `prefixed` does not encode a valid set of signers
func DecodeSignerIndicesToIdentifiers(
	canonicalIdentifiers flow.IdentifierList,
	prefixed []byte,
) (flow.IdentifierList, error) {
	indices, err := decodeSignerIndices(canonicalIdentifiers, prefixed)
	if err != nil {
		return nil, err
	}

	signerIDs := make(flow.IdentifierList, 0, len(indices))
	for _, index := range indices {
		signerIDs = append(signerIDs, canonicalIdentifiers[index])
	}
	return signerIDs, nil
}

func decodeSignerIndices(
	canonicalIdentifiers flow.IdentifierList,
	prefixed []byte,
) ([]int, error) {
	// the prefixed contains the checksum of the canonicalIdentifiers that the signerIndices
	// creator saw.
	// extract the checksum and compare with the canonicalIdentifiers to see if both
	// the signerIndices creator and validator see the same list.
	signerIndices, err := CompareAndExtract(canonicalIdentifiers, prefixed)
	if err != nil {
		if errors.Is(err, ErrInvalidChecksum) {
			return nil, NewInvalidSignerIndicesErrorf("signer indices' checkum is invalid: %w", err)
		}
		return nil, fmt.Errorf("unexpected exception while checking signer indices: %w", err)
	}

	numberCanonicalNodes := len(canonicalIdentifiers)
	err = validPadding(signerIndices, numberCanonicalNodes)
	if err != nil {
		if errors.Is(err, ErrIncompatibleBitVectorLength) || errors.Is(err, ErrIllegallyPaddedBitVector) {
			return nil, NewInvalidSignerIndicesErrorf("invalid padding of signerIndices: %w", err)
		}
		return nil, fmt.Errorf("unexpected exception while checking padding of signer indices: %w", err)
	}

	// decode bits to Identifiers
	indices := make([]int, 0, numberCanonicalNodes)
	for i := 0; i < numberCanonicalNodes; i++ {
		if bitutils.ReadBit(signerIndices, i) == 1 {
			indices = append(indices, i)
		}
	}
	return indices, nil
}

// DecodeSignerIndicesToIdentities decodes the given compacted bit vector into node Identities.
// Prerequisite:
//   - The input `canonicalIdentifiers` must exhaustively list the set of authorized signers in their canonical order.
//
// The returned list of decoded identities is in canonical order.
//
// Expected Error returns during normal operations:
// * signature.InvalidSignerIndicesError if the given index vector `prefixed` does not encode a valid set of signers
func DecodeSignerIndicesToIdentities(
	canonicalIdentities flow.IdentitySkeletonList,
	prefixed []byte,
) (flow.IdentitySkeletonList, error) {
	indices, err := decodeSignerIndices(canonicalIdentities.NodeIDs(), prefixed)
	if err != nil {
		return nil, err
	}

	signers := make(flow.IdentitySkeletonList, 0, len(indices))
	for _, index := range indices {
		signers = append(signers, canonicalIdentities[index])
	}
	return signers, nil
}

// validPadding verifies that `bitVector` satisfies the following criteria
//  1. The `bitVector`'s length [in bytes], must be the _minimal_ possible length such that it can hold
//     `numUsedBits` number of bits. Otherwise, we return an `ErrIncompatibleBitVectorLength`.
//  2. If `numUsedBits` is _not_ an integer-multiple of 8, `bitVector` is padded with tailing bits. Per
//     convention, these bits must be zero. Otherwise, we return an `ErrIllegallyPaddedBitVector`.
//
// Expected Error returns during normal operations:
//   - ErrIncompatibleBitVectorLength if the vector has the wrong length
//   - ErrIllegallyPaddedBitVector if the vector is padded with bits other than 0
func validPadding(bitVector []byte, numUsedBits int) error {
	// Verify condition 1:
	l := len(bitVector)
	if l != bitutils.MinimalByteSliceLength(numUsedBits) {
		return fmt.Errorf("the bit vector contains a payload of %d used bits, so it should have %d bytes but has %d bytes: %w",
			numUsedBits, bitutils.MinimalByteSliceLength(numUsedBits), l, ErrIncompatibleBitVectorLength)
	}
	// Condition 1 implies that the number of padded bits must be strictly smaller than 8. Otherwise, the vector
	// could have fewer bytes and still have enough room to store `numUsedBits`.

	// Verify condition 2, i.e. that all padded bits are all 0:
	// * As `bitVector` passed check 1, all padded bits are located in `bitVector`s _last byte_.
	// * Let `lastByte` be the last byte of `bitVector`. The leading bits, specifically `numUsedBits & 7`,
	//   belong to the used payload, which could have non-zero values. We remove these using left-bit-shifts.
	//   The result contains exactly all padded bits (plus some auxiliary 0-bits included by the bit-shift
	//   operator). Hence, condition 2 is satisfied if and only if the result is identical to zero.
	// Note that this implementation is much more efficient than individually checking the padded bits, as we check all
	// padded bits at once; furthermore, we only use multiplication, subtraction, shift, which are fast.
	if numUsedBits&7 == 0 { // if numUsedBits is multiple of 8, then there are no padding bits to check
		return nil
	}
	// the above check has ensured that lastByte does exist (l==0 is excluded)
	lastByte := bitVector[l-1]
	if (lastByte << (numUsedBits & 7)) != 0 { // shift by numUsedBits % 8
		return fmt.Errorf("some padded bits are not zero with %d used bits (bitVector: %x): %w", numUsedBits, bitVector, ErrIllegallyPaddedBitVector)
	}

	return nil
}
