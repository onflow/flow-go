package types

import (
	"fmt"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
	cadenceRLP "github.com/onflow/cadence/runtime/stdlib/rlp"

	"github.com/onflow/flow-go/model/flow"
)

type FlowAddress flow.Address

var FlowAddressCadenceType = cadence.TheAddressType
var FlowAddressSemaType = sema.TheAddressType

func (addr FlowAddress) ToCadenceValue() cadence.Address {
	return cadence.Address(addr)
}

type PublicPath string

var PublicPathCadenceType = cadence.ThePathType
var PublicPathSemaType = sema.PathType

func (p PublicPath) ToCadenceValue() cadence.Path {
	return cadence.Path{
		Domain:     common.PathDomainPublic,
		Identifier: string(p),
	}
}

type SignedData []byte

var SignedDataCadenceType = cadence.NewVariableSizedArrayType(cadence.TheUInt8Type)
var SignedDataSemaType = sema.ByteArrayType

func (sd SignedData) ToCadenceValue() cadence.Array {
	values := make([]cadence.Value, len(sd))
	for i, v := range sd {
		values[i] = cadence.NewUInt8(v)
	}
	return cadence.NewArray(values).WithType(SignedDataCadenceType)
}

type KeyIndices []uint64

var KeyIndicesCadenceType = cadence.NewVariableSizedArrayType(cadence.TheUInt64Type)
var KeyIndicesSemaType = &sema.VariableSizedType{Type: sema.UInt64Type}

func (ki KeyIndices) ToCadenceValue() cadence.Array {
	values := make([]cadence.Value, len(ki))
	for i, v := range ki {
		values[i] = cadence.NewUInt64(v)
	}
	return cadence.NewArray(values).WithType(KeyIndicesCadenceType)
}

func (ki KeyIndices) Count() int {
	return len(ki)
}

type Signature []byte

var SignatureCadenceType = cadence.NewVariableSizedArrayType(cadence.TheUInt8Type)

func (s Signature) ToCadenceValue() cadence.Array {
	values := make([]cadence.Value, len(s))
	for i, v := range s {
		values[i] = cadence.NewUInt8(v)
	}
	return cadence.NewArray(values).WithType(SignatureCadenceType)
}

type Signatures []Signature

var SignaturesCadenceType = cadence.NewVariableSizedArrayType(SignatureCadenceType)
var SignaturesSemaType = sema.ByteArrayArrayType

func (ss Signatures) ToCadenceValue() cadence.Array {
	values := make([]cadence.Value, len(ss))
	for i, s := range ss {
		values[i] = s.ToCadenceValue()
	}
	return cadence.NewArray(values).WithType(SignaturesCadenceType)
}

func (ss Signatures) Count() int {
	return len(ss)
}

// COAOwnershipProofInContext contains all the data
// needed to verify a COAOwnership proof.
// The proof is verified by checking the signatures over the
// input signed data (SignedData), then loading the resource
// capability from the provided path in the proof, and
// at last checking if the EVMAddress of the resource matches
// the provided one.
type COAOwnershipProofInContext struct {
	COAOwnershipProof
	SignedData SignedData
	EVMAddress Address
}

func NewCOAOwnershipProofInContext(sd []byte, addr Address, encodedProof []byte) (*COAOwnershipProofInContext, error) {
	proof, err := COAOwnershipProofFromEncoded(encodedProof)
	if err != nil {
		return nil, err
	}
	return &COAOwnershipProofInContext{
		COAOwnershipProof: *proof,
		SignedData:        sd,
		EVMAddress:        addr,
	}, nil
}

func (proof *COAOwnershipProofInContext) ToCadenceValues() []cadence.Value {
	return []cadence.Value{
		proof.Address.ToCadenceValue(),
		proof.CapabilityPath.ToCadenceValue(),
		proof.SignedData.ToCadenceValue(),
		proof.KeyIndices.ToCadenceValue(),
		proof.Signatures.ToCadenceValue(),
		proof.EVMAddress.ToCadenceValue(),
	}
}

// COAOwnershipProof is a proof that a flow account
// controls a COA resource. To do so, the flow
// account (Address is address of this account)
// provides signatures (with proper total weights) over an arbitary data input
// set by proof requester. KeyIndicies captures,
// which account keys has been used for signatures.
// Beside signatures, it provides the CapabilityPath
// where the resource EVMAddress capability is stored.
type COAOwnershipProof struct {
	KeyIndices     KeyIndices
	Address        FlowAddress
	CapabilityPath PublicPath
	Signatures     Signatures
}

func (p *COAOwnershipProof) Encode() ([]byte, error) {
	return rlp.EncodeToBytes(p)
}

func COAOwnershipProofSignatureCountFromEncoded(data []byte) (int, error) {
	// first break into proof encoded items
	encodedItems, _, err := cadenceRLP.DecodeList(data, 0)
	if err != nil {
		return 0, err
	}
	// first encoded item is KeyIndicies
	// so reading number of elements in the key indicies
	// should return the count without the need to fully decode
	KeyIndices, _, err := cadenceRLP.DecodeList(encodedItems[0], 0)
	return len(KeyIndices), err
}

func COAOwnershipProofFromEncoded(data []byte) (*COAOwnershipProof, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty proof")
	}
	p := &COAOwnershipProof{}
	return p, rlp.DecodeBytes(data, p)
}
