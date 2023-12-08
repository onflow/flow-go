package model

import (
	"bytes"
	"fmt"

	"github.com/onflow/crypto"
	"github.com/onflow/flow-go/model/encoding/rlp"
	"github.com/onflow/flow-go/model/flow"
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

// BeaconSignature extracts the source of randomness from the QC sigData.
//
// The sigData is an RLP encoded structure that is part of QuorumCertificate.
// The function only ever returns a model.InvalidFormatError, which indicates an
// invalid encoding.
func BeaconSignature(qc *flow.QuorumCertificate) ([]byte, error) {
	// unpack sig data to extract random beacon signature
	packer := SigDataPacker{}
	sigData, err := packer.Decode(qc.SigData)
	if err != nil {
		return nil, fmt.Errorf("could not unpack block signature: %w", err)
	}
	return sigData.ReconstructedRandomBeaconSig, nil
}
