package model

import (
	"bytes"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encoding/rlp"
)

// SigDataPacker implements logic for encoding/decoding SignatureData using RLP encoding.
type SigDataPacker struct {
	codec rlp.Codec // rlp encoder is used in order to ensure deterministic encoding
}

// SignatureData is a compact data type for encoding the block signature data
type SignatureData struct {
	// bit-vector indicating type of signature for each signer.
	// the order of each sig type matches the order of corresponding signer IDs
	SigType []byte

	AggregatedStakingSig         []byte
	AggregatedRandomBeaconSig    []byte
	ReconstructedRandomBeaconSig crypto.Signature
}

// Encode performs encoding of SignatureData
func (p *SigDataPacker) Encode(sigData *SignatureData) ([]byte, error) {
	var buf bytes.Buffer
	encoder := p.codec.NewEncoder(&buf)
	err := encoder.Encode(sigData)
	return buf.Bytes(), err
}

// Decode performs decoding of SignatureData
// This function is side-effect free. It only ever returns
// a model.InvalidFormatError, which indicates an invalid encoding.
func (p *SigDataPacker) Decode(data []byte) (*SignatureData, error) {
	bs := bytes.NewReader(data)
	decoder := p.codec.NewDecoder(bs)
	var sigData SignatureData
	err := decoder.Decode(&sigData)
	if err != nil {
		return nil, NewInvalidFormatErrorf("given data is not a valid encoding of SignatureData: %w", err)
	}
	return &sigData, nil
}

// UnpackRandomBeaconSig takes sigData previously packed by packer,
// decodes it and extracts random beacon signature.
// This function is side-effect free. It only ever returns a
// model.InvalidFormatError, which indicates an invalid encoding.
func UnpackRandomBeaconSig(sigData []byte) (crypto.Signature, error) {
	// decode into typed data
	packer := SigDataPacker{}
	sig, err := packer.Decode(sigData)
	return sig.ReconstructedRandomBeaconSig, err
}
