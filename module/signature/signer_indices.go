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

	signerIndices = bitutils.MakeBitVector(len(canonicalIdentifiers))
	sigTypes = bitutils.MakeBitVector(len(stakingSigners) + len(beaconSigners))
	signerCounter := 0
	for cannonicalIdx, member := range canonicalIdentifiers {
		if _, ok := stakingSignersLookup[member]; ok {
			bitutils.SetBit(signerIndices, cannonicalIdx)
			// The default value for sigTypes is bit zero, which corresponds to a staking sig.
			// Hence, we don't have to change anything here.
			delete(stakingSignersLookup, member)
			signerCounter++
			continue
		}
		if _, ok := beaconSignersLookup[member]; ok {
			bitutils.SetBit(signerIndices, cannonicalIdx)
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
// staking signer identifiers (`stakingSigners`) and the set of beacon signer identifiers (`beaconSigners`).
// Prerequisite:
//  * The input `signers` must be the set of signers in their canonical order.
//
// Expected Error returns during normal operations:
//  * IncompatibleIndexSliceError indicates that `signerIndices` has the wrong length
//  * IllegallyPaddedIndexSliceError is the vector is padded with bits other than 0
func DecodeSigTypeToStakingAndBeaconSigners(
	signers flow.IdentifierList,
	sigType []byte,
) (stakingSigners flow.IdentifierList, beaconSigners flow.IdentifierList, err error) {
	numberSigners := len(signers)
	if len(sigType) != bitutils.PaddedByteSliceLength(numberSigners) {
		return nil, nil, fmt.Errorf("there are %d signers, so the sigType vector should have %d bytes but has length %d: %w",
			numberSigners, bitutils.PaddedByteSliceLength(numberSigners), len(sigType), IncompatibleBitVectorLengthError)
	}

	// bits for padding are all required to be 0:
	for i := numberSigners; i < len(sigType); i++ {
		if bitutils.ReadBit(sigType, i) == 1 {
			return nil, nil, fmt.Errorf("all padded bits must be 0, but bit at index %d is 1: %w", i, IllegallyPaddedBitVectorError)
		}
	}

	// convert
	stakingSigners = make(flow.IdentifierList, 0, numberSigners)
	beaconSigners = make(flow.IdentifierList, 0, numberSigners)
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

	signerIndices = bitutils.MakeBitVector(len(canonicalIdentifiers))
	for cannonicalIdx, member := range canonicalIdentifiers {
		if _, ok := signersLookup[member]; ok {
			bitutils.SetBit(signerIndices, cannonicalIdx)
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
	numberCannonicalNodes := len(canonicalIdentifiers)
	if len(signerIndices) != bitutils.PaddedByteSliceLength(numberCannonicalNodes) {
		return nil, fmt.Errorf("received a cannonical set of %d nodes, so the index vector should have %d bytes but has length %d: %w",
			numberCannonicalNodes, bitutils.PaddedByteSliceLength(numberCannonicalNodes), len(signerIndices), IncompatibleBitVectorLengthError)
	}

	// bits for padding are all required to be 0:
	for i := numberCannonicalNodes; i < len(signerIndices); i++ {
		if bitutils.ReadBit(signerIndices, i) == 1 {
			return nil, fmt.Errorf("all padded bits must be 0, but bit at index %d is 1: %w", i, IllegallyPaddedBitVectorError)
		}
	}

	// convert
	signerIDs := make(flow.IdentifierList, 0, numberCannonicalNodes)
	for i := 0; i < numberCannonicalNodes; i++ {
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
	numberCannonicalNodes := len(canonicalIdentities)
	if len(signerIndices) != bitutils.PaddedByteSliceLength(numberCannonicalNodes) {
		return nil, fmt.Errorf("received a cannonical set of %d nodes, so the index vector should have %d bytes but has length %d: %w",
			numberCannonicalNodes, bitutils.PaddedByteSliceLength(numberCannonicalNodes), len(signerIndices), IncompatibleBitVectorLengthError)
	}

	// bits for padding are all required to be 0:
	for i := numberCannonicalNodes; i < len(signerIndices); i++ {
		if bitutils.ReadBit(signerIndices, i) == 1 {
			return nil, fmt.Errorf("all padded bits must be 0, but bit at index %d is 1: %w", i, IllegallyPaddedBitVectorError)
		}
	}

	// convert
	signerIdentities := make(flow.IdentityList, 0, numberCannonicalNodes)
	for i := 0; i < numberCannonicalNodes; i++ {
		if bitutils.ReadBit(signerIndices, i) == 1 {
			signerIdentities = append(signerIdentities, canonicalIdentities[i])
		}
	}
	return signerIdentities, nil
}
