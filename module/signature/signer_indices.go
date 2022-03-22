package signature

import (
	"fmt"

	"github.com/onflow/flow-go/ledger/common/bitutils"
	"github.com/onflow/flow-go/model/flow"
)

// EncodeSignerToIndicesAndSigType encodes the given stakingSigners and beaconSigners into bit vectors for
// signer indices and sig types.
// PREREQUISITES:
//  * The input `canonicalIdentifiers` must exhaustively list the set of authorized signers in their canonical order.
//  * The inputs `stakingSigners` and `beaconSigners` are treated as sets, i.e. they
//    should not contain any duplicates.
//  * A node can be listed in either `stakingSigners` or `beaconSigners`. A node appearing in both lists
//    constitutes an illegal input.
//  * `stakingSigners` must be a subset of `canonicalIdentifiers`
//  * `beaconSigners` must be a subset of `canonicalIdentifiers`
//
// RETURN VALUES:
//  *  `signerIndices` is a bit vector. Let signerIndices[i] denote the ith bit of `signerIndices`.
//                             ┌ 1 if and only if canonicalIdentifiers[i] is in `stakingSigners` or  `beaconSigners`
//          signerIndices[i] = └ 0 otherwise
//      Let `n` be the length of `canonicalIdentifiers`. `signerIndices` contains at least `n` bits, though, we
//      right-pad it with tailing zeros to full bytes.
//  * `sigTypes` is a bit vector. Let sigTypes[i] denote the ith bit of `sigTypes`
//                        ┌ 1 if and only if the ith signer is in `beaconSigners`
//          sigTypes[i] = └ 0 if and only if the ith signer is in `stakingSigners`
//     (Per prerequisite, we require that no signer is listed in both `beaconSigners` and `stakingSigners`)
//
// Example:
// As an example consider the case where we have a committee C of 10 nodes in canonical oder
//            C = [A,B,C,D,E,F,G,H,I,J]
// where nodes [B,F] are stakingSigners and beaconSigners are [C,E,G,I,J].
//  * First return parameter: `signerIndices`
//    - We start with a bit vector v that has |C| number of bits
//    - If a node contributed either as staking signer or beacon signer,
//      we set the respective bit to 1:
//          [A,B,C,D,E,F,G,H,I,J]
//             ↓     ↓ ↓     ↓ ↓
//           0,1,1,0,1,1,1,0,1,1
//    - Lastly, right-pad the resulting bit vector with 0 to full bytes. We have 10 committee members,
//      so we pad to 2 bytes:
//           01101110 11000000
//  * second return parameter: `sigTypes`
//    - Here, we restrict our focus on the signers, which we encoded in the previous step.
//      In out example, nodes [B,C,E,F,G,I,J] signed in canonical order. This is exactly the same order,
//      as we have represented the signer in the last step.
//    - For these 5 nodes in their canonical order, we encode each node's signature type as
// 			bit-value 1: node was in beaconSigners
// 			bit-value 0: node was in stakingSigners
//      This results in the bit vector
//            [B,C,E,F,G,I,J]
//             ↓ ↓ ↓ ↓ ↓ ↓ ↓
//             0,1,0,1,1,1,1
//    - Again, we right-pad with zeros to full bytes, As we only had 7 signers, the sigType slice is 1byte long
//             01011110
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

	return signerIndices, sigTypes, nil
}

// DecodeSigTypeToStakingAndBeaconSigners decodes the bit-vector `sigType` to the set of
// staking signer identities (`stakingSigners`) and the set of beacon signer identities (`beaconSigners`).
// Prerequisite:
//  * The input `signers` must be the set of signers in their canonical order.
//
// Expected Error returns during normal operations:
//  * IncompatibleIndexSliceError indicates that `signerIndices` has the wrong length
//  * IllegallyPaddedIndexSliceError is the vector is padded with bits other than 0
func DecodeSigTypeToStakingAndBeaconSigners(
	signers flow.IdentityList,
	sigType []byte,
) (stakingSigners flow.IdentityList, beaconSigners flow.IdentityList, err error) {
	numberSigners := len(signers)
	if e := validPadding(sigType, numberSigners); e != nil {
		return nil, nil, fmt.Errorf("sigType is invalid: %w", e)
	}

	// decode bits to Identities
	stakingSigners = make(flow.IdentityList, 0, numberSigners)
	beaconSigners = make(flow.IdentityList, 0, numberSigners)
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
//  * The input `canonicalIdentifiers` must exhaustively list the set of authorized signers in their canonical order.
//  * The input `signerIDs` represents a set, i.e. it should not contain any duplicates.
//  * `signerIDs` must be a subset of `canonicalIdentifiers`
//
// RETURN VALUE:
//  *  `signerIndices` is a bit vector. Let signerIndices[i] denote the ith bit of `signerIndices`.
//                             ┌ 1 if and only if canonicalIdentifiers[i] is in `stakingSigners` or  `beaconSigners`
//          signerIndices[i] = └ 0 otherwise
//      Let `n` be the length of `canonicalIdentifiers`. `signerIndices` contains at least `n` bits, though, we
//      right-pad it with tailing zeros to full bytes.
//
// Example:
// As an example consider the case where we have a committee C of 10 nodes in canonical oder
//            C = [A,B,C,D,E,F,G,H,I,J]
// where nodes [B,F] are stakingSigners and beaconSigners are [C,E,G,I,J].
//  * First return parameter: QC.signerIndices
//    - We start with a bit vector v that has |C| number of bits
//    - If a node contributed either as staking signer or beacon signer,
//      we set the respective bit to 1:
//          [A,B,C,D,E,F,G,H,I,J]
//             ↓     ↓ ↓     ↓ ↓
//           0,1,1,0,1,1,1,0,1,1
//    - Lastly, right-pad the resulting bit vector with 0 to full bytes. We have 10 committee members,
//      so we pad to 2 bytes:
//           01101110 11000000
//
// ERROR RETURNS
// During normal operations, no error returns are expected. This is because encoding signer sets is generally
// part of the node's internal work to generate messages. Hence, the inputs to this method come from other
// trusted components within the node. Therefore, any illegal input is treated as a symptom of an internal bug.
// fullIdentities represents all identities who are eligible to sign the given resource. It excludes
// identities who are ineligible to sign the given resource. For example, fullIdentities in the context
// of a cluster consensus quorum certificate would include authorized members of the cluster and
// exclude ejected members of the cluster, or unejected collection nodes from a different cluster.
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
		return nil, fmt.Errorf("unknown signers %v", signersLookup)
	}

	return signerIndices, nil
}

// DecodeSignerIndicesToIdentifiers decodes the given compacted bit vector into signerIDs
// Prerequisite:
//  * The input `canonicalIdentifiers` must exhaustively list the set of authorized signers in their canonical order.
//
// Expected Error returns during normal operations:
//  * IncompatibleIndexSliceError indicates that `signerIndices` has the wrong length
//  * IllegallyPaddedIndexSliceError is the vector is padded with bits other than 0
func DecodeSignerIndicesToIdentifiers(
	canonicalIdentifiers flow.IdentifierList,
	signerIndices []byte,
) (flow.IdentifierList, error) {
	numberCanonicalNodes := len(canonicalIdentifiers)
	if e := validPadding(signerIndices, numberCanonicalNodes); e != nil {
		return nil, fmt.Errorf("signerIndices are invalid: %w", e)
	}

	// decode bits to Identifiers
	signerIDs := make(flow.IdentifierList, 0, numberCanonicalNodes)
	for i := 0; i < numberCanonicalNodes; i++ {
		if bitutils.ReadBit(signerIndices, i) == 1 {
			signerIDs = append(signerIDs, canonicalIdentifiers[i])
		}
	}
	return signerIDs, nil
}

// DecodeSignerIndicesToIdentities decodes the given compacted bit vector into node Identities.
// Prerequisite:
//  * The input `canonicalIdentifiers` must exhaustively list the set of authorized signers in their canonical order.
//
// Expected Error returns during normal operations:
//  * IncompatibleIndexSliceError indicates that `signerIndices` has the wrong length
//  * IllegallyPaddedIndexSliceError is the vector is padded with bits other than 0
func DecodeSignerIndicesToIdentities(
	canonicalIdentities flow.IdentityList,
	signerIndices []byte,
) (flow.IdentityList, error) {
	numberCanonicalNodes := len(canonicalIdentities)
	if e := validPadding(signerIndices, numberCanonicalNodes); e != nil {
		return nil, fmt.Errorf("signerIndices padding are invalid: %w", e)
	}

	// decode bits to Identities
	signerIdentities := make(flow.IdentityList, 0, numberCanonicalNodes)
	for i := 0; i < numberCanonicalNodes; i++ {
		if bitutils.ReadBit(signerIndices, i) == 1 {
			signerIdentities = append(signerIdentities, canonicalIdentities[i])
		}
	}
	return signerIdentities, nil
}

// validPadding verifies that `bitVector` satisfies the following criteria
//  1. The `bitVector`'s length [in bytes], must be the _minimal_ possible length such that it can hold
//    `numUsedBits` number of bits. Otherwise, we return an `IncompatibleBitVectorLengthError`.
//  2. If `numUsedBits` is _not_ an integer-multiple of 8, `bitVector` is padded with tailing bits. Per
//     convention, these bits must be zero. Otherwise, we return an `IllegallyPaddedBitVectorError`.
// All errors represent expected failure cases for byzantine inputs. There are _no unexpected_ error returns.
func validPadding(bitVector []byte, numUsedBits int) error {
	// Verify condition 1:
	l := len(bitVector)
	if l != bitutils.MinimalByteSliceLength(numUsedBits) {
		return fmt.Errorf("the bit vector contains a payload of %d used bits, so it should have %d bytes but has length %d: %w",
			numUsedBits, bitutils.MinimalByteSliceLength(numUsedBits), l, IncompatibleBitVectorLengthError)
	}
	// Condition 1 implies that the number of padded bits must be strictly smaller than 8. Otherwise, the vector
	// could have fewer bytes and still have enough room to store `numUsedBits`.

	// Verify condition 2, i.e. that all padded bits are all 0:
	// * The number of padded bits are: `numPaddedBits := 8*len(bitVector) - len(canonicalIdentifiers)`
	// * We know that numPaddedBits < 8; as `bitVector` passed check 1. Therefore, all padded bits are
	//   located in `bitVector`s _last byte_.
	// * Let `lastByte` be the last byte of `bitVector`. The leading bits, specifically `(8 - numPaddedBits)`
	//   belong to the used payload, which could have non-zero values. We remove these using left-bit-shifts.
	//   The result contains exactly all padded bits (plus some auxiliary 0-bits included by the bit-shift
	//   operator). Hence, condition 2 is satisfied if and only if the result is identical to zero.
	// Note that this implementation is much more efficient than individually checking the padded bits, as we check all
	// padded bits at once; furthermore, we only use multiplication, subtraction, shift, which are fast.
	if l == 0 {
		return nil
	}
	// the above check has ensured that lastByte does exist
	lastByte := bitVector[l-1]
	numPaddedBits := 8*l - numUsedBits
	if (lastByte << (8 - numPaddedBits)) != 0 {
		return fmt.Errorf("some padded bits are not zero: %w", IllegallyPaddedBitVectorError)
	}

	return nil
}
