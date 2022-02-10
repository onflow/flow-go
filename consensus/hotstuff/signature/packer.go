package signature

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/packer"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

// ConsensusSigDataPacker implements the hotstuff.Packer interface.
// The encoding method is RLP.
type ConsensusSigDataPacker struct {
	packer.SigDataPacker
	committees hotstuff.Committee
}

var _ hotstuff.Packer = &ConsensusSigDataPacker{}

func NewConsensusSigDataPacker(committees hotstuff.Committee) *ConsensusSigDataPacker {
	return &ConsensusSigDataPacker{
		committees: committees,
	}
}

// Pack serializes the block signature data into raw bytes, suitable to creat a QC.
// To pack the block signature data, we first build a compact data type, and then encode it into bytes.
// Expected error returns during normal operations:
//  * none; all errors are symptoms of inconsistent input data or corrupted internal state.
func (p *ConsensusSigDataPacker) Pack(blockID flow.Identifier, sig *hotstuff.BlockSignatureData) ([]flow.Identifier, []byte, error) {
	// breaking staking and random beacon signers into signerIDs and sig type for compaction
	// each signer must have its signerID and sig type stored at the same index in the two slices
	count := len(sig.StakingSigners) + len(sig.RandomBeaconSigners)
	signerIDs := make([]flow.Identifier, 0, count)
	sigTypes := make([]hotstuff.SigType, 0, count)

	// retrieve all authorized consensus participants at the given block
	consensus, err := p.committees.Identities(blockID, filter.Any)
	if err != nil {
		return nil, nil, fmt.Errorf("could not find consensus committees by block id(%v): %w", blockID, err)
	}

	// lookup is a map from node identifier to node identity
	// it is used to check the given signers are all valid signers at the given block
	lookup := consensus.Lookup()

	for _, stakingSigner := range sig.StakingSigners {
		_, ok := lookup[stakingSigner]
		if ok {
			signerIDs = append(signerIDs, stakingSigner)
			sigTypes = append(sigTypes, hotstuff.SigTypeStaking)
		} else {
			return nil, nil, fmt.Errorf("staking signer %v not found in the committee at block: %v", stakingSigner, blockID)
		}
	}

	for _, beaconSigner := range sig.RandomBeaconSigners {
		_, ok := lookup[beaconSigner]
		if ok {
			signerIDs = append(signerIDs, beaconSigner)
			sigTypes = append(sigTypes, hotstuff.SigTypeRandomBeacon)
		} else {
			return nil, nil, fmt.Errorf("random beacon signer %v not found in the committee at block: %v", beaconSigner, blockID)
		}
	}

	// serialize the sig type for compaction
	serialized, err := serializeToBitVector(sigTypes)
	if err != nil {
		return nil, nil, fmt.Errorf("could not serialize sig types to bytes at block: %v, %w", blockID, err)
	}

	data := packer.SignatureData{
		SigType:                      serialized,
		AggregatedStakingSig:         sig.AggregatedStakingSig,
		AggregatedRandomBeaconSig:    sig.AggregatedRandomBeaconSig,
		ReconstructedRandomBeaconSig: sig.ReconstructedRandomBeaconSig,
	}

	// encode the structured data into raw bytes
	encoded, err := p.Encode(&data)
	if err != nil {
		return nil, nil, fmt.Errorf("could not encode data %v, %w", data, err)
	}

	return signerIDs, encoded, nil
}

// Unpack de-serializes the provided signature data.
// blockID is the block that the aggregated sig is signed for
// sig is the aggregated signature data
// It returns:
//  - (sigData, nil) if successfully unpacked the signature data
//  - (nil, model.ErrInvalidFormat) if failed to unpack the signature data
func (p *ConsensusSigDataPacker) Unpack(blockID flow.Identifier, signerIDs []flow.Identifier, sigData []byte) (*hotstuff.BlockSignatureData, error) {
	// decode into typed data
	data, err := p.Decode(sigData)
	if err != nil {
		return nil, fmt.Errorf("could not decode sig data %s: %w", err, model.ErrInvalidFormat)
	}

	// deserialize the compact sig types
	sigTypes, err := deserializeFromBitVector(data.SigType, len(signerIDs))
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize sig types from bytes: %w", err)
	}

	// read all the possible signer IDs at the given block
	consensus, err := p.committees.Identities(blockID, filter.Any)
	if err != nil {
		return nil, fmt.Errorf("could not find consensus committees by block id(%v): %w", blockID, err)
	}

	// lookup is a map from node identifier to node identity
	// it is used to check the given signerIDs are all valid signers at the given block
	lookup := consensus.Lookup()

	// read each signer's signerID and sig type from two different slices
	// group signers by its sig type
	stakingSigners := make([]flow.Identifier, 0, len(signerIDs))
	randomBeaconSigners := make([]flow.Identifier, 0, len(signerIDs))

	for i, sigType := range sigTypes {
		signerID := signerIDs[i]
		_, ok := lookup[signerID]
		if !ok {
			return nil, fmt.Errorf("unknown signer ID (%v) at the given block (%v): %w",
				signerID, blockID, model.ErrInvalidFormat)
		}

		if sigType == hotstuff.SigTypeStaking {
			stakingSigners = append(stakingSigners, signerID)
		} else if sigType == hotstuff.SigTypeRandomBeacon {
			randomBeaconSigners = append(randomBeaconSigners, signerID)
		} else {
			return nil, fmt.Errorf("unknown sigType %v, %w", sigType, model.ErrInvalidFormat)
		}
	}

	return &hotstuff.BlockSignatureData{
		StakingSigners:               stakingSigners,
		RandomBeaconSigners:          randomBeaconSigners,
		AggregatedStakingSig:         data.AggregatedStakingSig,
		AggregatedRandomBeaconSig:    data.AggregatedRandomBeaconSig,
		ReconstructedRandomBeaconSig: data.ReconstructedRandomBeaconSig,
	}, nil
}

// the total number of bytes required to fit the `count` number of bits
func bytesCount(count int) int {
	return (count + 7) >> 3
}

// serializeToBitVector encodes the given sigTypes into a bit vector.
// We append tailing `0`s to the vector to represent it as bytes.
func serializeToBitVector(sigTypes []hotstuff.SigType) ([]byte, error) {
	totalBytes := bytesCount(len(sigTypes))
	bytes := make([]byte, 0, totalBytes)
	// a sig type can be converted into one bit, the type at index 0 being converted into the least significant bit:
	// the random beacon type is mapped to 1, while the staking type is mapped to 0.
	// the remaining unfilled bits in the last byte will be 0
	const initialMask = byte(1 << 7)

	b := byte(0)
	mask := initialMask
	// iterate through every 8 sig types, and convert it into a byte
	for pos, sigType := range sigTypes {
		if sigType == hotstuff.SigTypeRandomBeacon {
			b ^= mask // set a bit to one
		} else if sigType != hotstuff.SigTypeStaking {
			return nil, fmt.Errorf("invalid sig type: %v at pos %v", sigType, pos)
		}

		mask >>= 1     // move to the next bit
		if mask == 0 { // this happens every 8 loop iterations
			bytes = append(bytes, b)
			b = byte(0)
			mask = initialMask
		}
	}
	// add the last byte when the length is not multiple of 8
	if mask != initialMask {
		bytes = append(bytes, b)
	}
	return bytes, nil
}

// deserializeFromBitVector decodes the sig types from the given bit vector
// - serialized: bit-vector, one bit for each signer (tailing `0`s appended to make full bytes)
// - count: the total number of sig types to be deserialized from the given bytes
// It returns:
// - (sigTypes, nil) if successfully deserialized sig types
// - (nil, model.ErrInvalidFormat) if the number of serialized bytes doesn't match the given number of sig types
// - (nil, model.ErrInvalidFormat) if the remaining bits in the last byte are not all 0s
func deserializeFromBitVector(serialized []byte, count int) ([]hotstuff.SigType, error) {
	types := make([]hotstuff.SigType, 0, count)

	// validate the length of serialized vector
	// it must be equal to the bytes required to fit exactly `count` number of bits
	totalBytes := bytesCount(count)
	if len(serialized) != totalBytes {
		return nil, fmt.Errorf("encoding sig types of %d signers requires %d bytes but got %d bytes: %w",
			count, totalBytes, len(serialized), model.ErrInvalidFormat)
	}

	// parse each bit in the bit-vector, bit 0 is SigTypeStaking, bit 1 is SigTypeRandomBeacon
	var byt byte
	var offset int
	for i := 0; i < count; i++ {
		byt = serialized[i>>3]
		offset = 7 - (i & 7)
		mask := byte(1 << offset)
		if byt&mask == 0 {
			types = append(types, hotstuff.SigTypeStaking)
		} else {
			types = append(types, hotstuff.SigTypeRandomBeacon)
		}
	}

	// remaining bits (if any), they must be all `0`s
	remainings := byt << (8 - offset)
	if remainings != byte(0) {
		return nil, fmt.Errorf("the remaining bits are expected to be all 0s, but are %v: %w",
			remainings, model.ErrInvalidFormat)
	}

	return types, nil
}
