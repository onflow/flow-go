package signature

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encoding/rlp"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/signature"
)

// ConsensusSigDataPacker implements the hotstuff.Packer interface.
// The encoding method is RLP.
type ConsensusSigDataPacker struct {
	committees hotstuff.Committee
	encoder    *rlp.Encoder // rlp encoder is used in order to ensure deterministic encoding
}

var _ hotstuff.Packer = &ConsensusSigDataPacker{}

func NewConsensusSigDataPacker(committees hotstuff.Committee) *ConsensusSigDataPacker {
	return &ConsensusSigDataPacker{
		committees: committees,
		encoder:    rlp.NewEncoder(),
	}
}

// signatureData is a compact data type for encoding the block signature data
type signatureData struct {
	// bit-vector indicating type of signature for each signer.
	// the order of each sig type matches the order of corresponding signer IDs
	SigType []byte

	AggregatedStakingSig      []byte
	AggregatedRandomBeaconSig []byte
	RandomBeacon              crypto.Signature
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

	// read all the possible signer IDs at the given block
	consensus, err := p.committees.Identities(blockID, filter.HasRole(flow.RoleConsensus))
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
			return nil, nil, fmt.Errorf("staking signer ID (%v) not found in the committees at block: %v", stakingSigner, blockID)
		}
	}

	for _, beaconSigner := range sig.RandomBeaconSigners {
		_, ok := lookup[beaconSigner]
		if ok {
			signerIDs = append(signerIDs, beaconSigner)
			sigTypes = append(sigTypes, hotstuff.SigTypeRandomBeacon)
		} else {
			return nil, nil, fmt.Errorf("random beacon signer ID (%v) not found in the committees at block: %v", beaconSigner, blockID)
		}
	}

	// serialize the sig type for compaction
	serialized, err := serializeToBytes(sigTypes)
	if err != nil {
		return nil, nil, fmt.Errorf("could not serialize sig types to bytes at block: %v, %w", blockID, err)
	}

	data := signatureData{
		SigType:                   serialized,
		AggregatedStakingSig:      sig.AggregatedStakingSig,
		AggregatedRandomBeaconSig: sig.AggregatedRandomBeaconSig,
		RandomBeacon:              sig.ReconstructedRandomBeaconSig,
	}

	// encode the structured data into raw bytes
	encoded, err := p.encoder.Encode(data)
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
//  - (nil, signature.ErrInvalidFormat) if failed to unpack the signature data
func (p *ConsensusSigDataPacker) Unpack(blockID flow.Identifier, signerIDs []flow.Identifier, sigData []byte) (*hotstuff.BlockSignatureData, error) {
	// decode into typed data
	var data signatureData
	err := p.encoder.Decode(sigData, &data)
	if err != nil {
		return nil, fmt.Errorf("could not decode sig data %s: %w", err, signature.ErrInvalidFormat)
	}

	// deserialize the compact sig types
	sigTypes, err := deserializeFromBytes(data.SigType, len(signerIDs))
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize sig types from bytes: %w", err)
	}

	// read all the possible signer IDs at the given block
	consensus, err := p.committees.Identities(blockID, filter.HasRole(flow.RoleConsensus))
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
				signerID, blockID, signature.ErrInvalidFormat)
		}

		if sigType == hotstuff.SigTypeStaking {
			stakingSigners = append(stakingSigners, signerID)
		} else if sigType == hotstuff.SigTypeRandomBeacon {
			randomBeaconSigners = append(randomBeaconSigners, signerID)
		} else {
			return nil, fmt.Errorf("unknown sigType %v, %w", sigType, signature.ErrInvalidFormat)
		}
	}

	return &hotstuff.BlockSignatureData{
		StakingSigners:               stakingSigners,
		RandomBeaconSigners:          randomBeaconSigners,
		AggregatedStakingSig:         data.AggregatedStakingSig,
		AggregatedRandomBeaconSig:    data.AggregatedRandomBeaconSig,
		ReconstructedRandomBeaconSig: data.RandomBeacon,
	}, nil
}

// the total number of bytes required to fit the `count` number of bits
func bytesCount(count int) int {
	totalBytes := count / 8
	if count%8 > 0 {
		totalBytes++
	}
	return totalBytes
}

// serializeToBytes encodes the given sigTypes into a bit vector.
// We append tailing `0`s to the vector to represent it as bytes.
func serializeToBytes(sigTypes []hotstuff.SigType) ([]byte, error) {
	totalBytes := bytesCount(len(sigTypes))
	bytes := make([]byte, 0, totalBytes)
	// a sig type can be converted into one bit.
	// so every 8 sig types will fit into one byte.
	// iterate through every 8 sig types, for each sig type in each
	// group, convert into each bit, and fill the bit into the byte.
	// the remaining unfilled bits in the last byte will be 0
	for byt := 0; byt < len(sigTypes); byt += 8 {
		b := byte(0)
		offset := 7
		for pos := byt; pos < byt+8 && pos < len(sigTypes); pos++ {
			sigType := sigTypes[pos]

			if !sigType.Valid() {
				return nil, fmt.Errorf("invalid sig type: %v at pos %v", sigType, pos)
			}

			b ^= (byte(sigType) << offset)
			offset--
		}
		bytes = append(bytes, b)
	}
	return bytes, nil
}

// deserializeFromBytes decodes the sig types from the given bit vector
// - serialized: bit-vector, one bit for each signer (tailing `0`s appended to make full bytes)
// - count: the total number of sig types to be deserialized from the given bytes
// It returns:
// - (sigTypes, nil) if successfully deserialized sig types
// - (nil, signature.ErrInvalidFormat) if the number of serialized bytes doesn't match the given number of sig types
// - (nil, signature.ErrInvalidFormat) if the remaining bits in the last byte are not all 0s
func deserializeFromBytes(serialized []byte, count int) ([]hotstuff.SigType, error) {
	types := make([]hotstuff.SigType, 0, count)

	// validate the length of serialized
	// it must have enough bytes to fit the `count` number of bits
	totalBytes := bytesCount(count)

	if len(serialized) != totalBytes {
		return nil, fmt.Errorf("encoding sig types of %d signers requires %d bytes but got %d bytes: %w",
			count, totalBytes, len(serialized), signature.ErrInvalidFormat)
	}

	// parse each bit in the bit-vector, bit 0 is SigTypeStaking, bit 1 is SigTypeRandomBeacon
	for i := 0; i < count; i++ {
		byt := serialized[i/8]
		offset := 7 - (i % 8)
		posMask := byte(1 << offset)
		if byt&posMask == 0 {
			types = append(types, hotstuff.SigTypeStaking)
		} else {
			types = append(types, hotstuff.SigTypeRandomBeacon)
		}
	}

	// if there are remaining bits, they must be all `0`s
	if count%8 > 0 {
		// since we've validated the length of serialized, then the last byte
		// must contain the remaining bits
		last := serialized[len(serialized)-1]
		remainings := last << (count % 8) // shift away the last used bits
		if remainings != byte(0) {
			return nil, fmt.Errorf("the remaining bits are expect to be all 0s, but actually are %v: %w",
				remainings, signature.ErrInvalidFormat)
		}
	}

	return types, nil
}
