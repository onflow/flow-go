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

// ConsensusSigPackerImpl implements the hotstuff.Packer interface.
// The encoding method is RLP.
type ConsensusSigPackerImpl struct {
	committees hotstuff.Committee
	encoder    *rlp.Encoder // rlp encoder is used in order to ensure deterministic encoding
}

var _ hotstuff.Packer = &ConsensusSigPackerImpl{}

func NewConsensusSigPackerImpl(committees hotstuff.Committee) *ConsensusSigPackerImpl {
	return &ConsensusSigPackerImpl{
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
func (p *ConsensusSigPackerImpl) Pack(blockID flow.Identifier, sig *hotstuff.BlockSignatureData) ([]flow.Identifier, []byte, error) {
	consensus, err := p.committees.Identities(blockID, filter.HasRole(flow.RoleConsensus))
	if err != nil {
		return nil, nil, fmt.Errorf("could not find consensus committees by block id(%v): %w", blockID, err)
	}

	// lookup is a map from node identifier to node identity
	lookup := consensus.Lookup()
	count := len(sig.StakingSigners) + len(sig.RandomBeaconSigners)

	signerIDs := make([]flow.Identifier, 0, count)
	// the order of sig types matches the signer IDs so that it indicates
	// which type of signature each signer produces.
	sigTypes := make([]hotstuff.SigType, 0, count)

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

	seralized, err := seralizeToBytes(sigTypes)
	if err != nil {
		return nil, nil, fmt.Errorf("could not seralize sig types to bytes at block: %v, %w", blockID, err)
	}

	data := signatureData{
		SigType:                   seralized,
		AggregatedStakingSig:      sig.AggregatedStakingSig,
		AggregatedRandomBeaconSig: sig.AggregatedRandomBeaconSig,
		RandomBeacon:              sig.ReconstructedRandomBeaconSig,
	}

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
func (p *ConsensusSigPackerImpl) Unpack(blockID flow.Identifier, signerIDs []flow.Identifier, sigData []byte) (*hotstuff.BlockSignatureData, error) {
	var data signatureData
	err := p.encoder.Decode(sigData, &data)
	if err != nil {
		return nil, fmt.Errorf("could not decode sig data %s: %w", err, signature.ErrInvalidFormat)
	}

	stakingSigners := make([]flow.Identifier, 0)
	randomBeaconSigners := make([]flow.Identifier, 0)

	sigTypes, err := deseralizeFromBytes(data.SigType, len(signerIDs))
	if err != nil {
		return nil, fmt.Errorf("failed to deseralize sig types from bytes: %w", err)
	}

	for i, sigType := range sigTypes {
		if sigType == hotstuff.SigTypeStaking {
			stakingSigners = append(stakingSigners, signerIDs[i])
		} else if sigType == hotstuff.SigTypeRandomBeacon {
			randomBeaconSigners = append(randomBeaconSigners, signerIDs[i])
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

// given that a SigType can either be a Staking Sig or a Random Beacon Sig
// a SigType can be converted into a bool.
// it returns:
// - (false, nil) for SigTypeStakingSig
// - (true, nil) for SigTypeRandomBeaconSig
// - (false, signature.ErrInvalidFormat) for invalid sig type
func sigTypeToBool(t hotstuff.SigType) (bool, error) {
	if t == hotstuff.SigTypeStaking {
		return false, nil
	}
	if t == hotstuff.SigTypeRandomBeacon {
		return true, nil
	}
	return false, signature.ErrInvalidFormat
}

// seralize the sig types into a compact format
func seralizeToBytes(sigTypes []hotstuff.SigType) ([]byte, error) {
	bytes := make([]byte, 0)
	for byt := 0; byt < len(sigTypes); byt += 8 {
		b := byte(0)
		offset := 7
		for pos := byt; pos < 8 && pos < len(sigTypes); pos++ {
			sigType := sigTypes[pos]

			if !sigType.Valid() {
				return nil, fmt.Errorf("invalid sig type: %v at pos %v", sigType, pos)
			}

			b = b ^ (byte(sigType) << offset)
		}
		bytes = append(bytes, b)
		offset--
	}
	return bytes, nil
}

// deseralize the sig types from bytes
func deseralizeFromBytes(seralized []byte, count int) ([]hotstuff.SigType, error) {
	types := make([]hotstuff.SigType, 0, count)

	// validate the length of seralized
	// it must have enough bytes to fit the `count` number of bits
	totalBytes := count / 8
	if count%8 > 0 {
		totalBytes++
	}

	if len(seralized) != totalBytes {
		return nil, fmt.Errorf("mismatching bytes for seralized sig types, total signers: %v"+
			", expect bytes: %v, actual bytes: %v, %w", count, totalBytes, len(seralized), signature.ErrInvalidFormat)
	}

	// parse each bit in the bit-vector, bit 0 is SigTypeStaking, bit 1 is SigTypeRandomBeacon
	for i := 0; i < count; i++ {
		byt := seralized[i/8]
		posMask := byte((1 << 7) >> (i % 8))
		if byt&posMask == 0 {
			types = append(types, hotstuff.SigTypeStaking)
		} else {
			types = append(types, hotstuff.SigTypeRandomBeacon)
		}
	}

	// if there are remaining bits, they must be all `0`s
	if count%8 > 0 {
		// since we've validated the length of seralized, then the last byte
		// must contains the remaining bits
		last := seralized[len(seralized)-1]
		remainings := last << (count % 8) // shift away the last used bits
		if remainings != byte(0) {
			return nil, fmt.Errorf("the remaining bits are expect to be all 0s, but actually are: %v: %w",
				remainings, signature.ErrInvalidFormat)
		}
	}

	return types, nil
}
