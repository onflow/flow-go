// Package encoding provides byte serialization and deserialization of trie and ledger structs.
package encoding

import (
	"fmt"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
)

// Version captures the maximum version of encoding that this code supports
// in other words this code encodes the data with the latest version and
// can only decode data with version smaller or equal to this value
// bumping this number prevents older versions of the code to deal with the newer version of data
// codes should be updated with backward compatibility if needed
const Version = uint16(0)

// Type capture the type of encoded entity (e.g. State, Key, Value, Path)
type Type uint8

const (
	// TypeUnknown - unknown type
	TypeUnknown = iota
	// TypeState - type for State
	TypeState
	// TypeKeyPart - type for KeyParts (a subset of key)
	TypeKeyPart
	// TypeKey - type for Keys (unique identifier to reference a location in ledger)
	TypeKey
	// TypeValue - type for Ledger Values
	TypeValue
	// TypePath - type for Paths (trie storage location of a key value pair)
	TypePath
	// TypePayload - type for Payloads (stored at trie nodes including key value pair )
	TypePayload
	// TypeProof type for Proofs
	// (all data needed to verify a key value pair at specific state)
	TypeProof
	// TypeBatchProof - type for BatchProofs
	TypeBatchProof
	// TypeQuery - type for ledger query
	TypeQuery
	// TypeUpdate - type for ledger update
	TypeUpdate
	// TypeTrieUpdate - type for trie update
	TypeTrieUpdate
	// this is used to flag types from the future
	typeUnsuported
)

func (e Type) String() string {
	return [...]string{"Unknown", "State", "KeyPart", "Key", "Value", "Path", "Payload", "Proof", "BatchProof", "Query", "Update", "Trie Update"}[e]
}

// CheckVersion extracts encoding bytes from a raw encoded message
// checks it against the supported versions and returns the rest of rawInput (excluding encDecVersion bytes)
func CheckVersion(rawInput []byte) (rest []byte, version uint16, err error) {
	version, rest, err = utils.ReadUint16(rawInput)
	if err != nil {
		return rest, version, fmt.Errorf("error checking the encoding decoding version: %w", err)
	}
	// error on versions coming from future till a time-machine is invented
	if version > Version {
		return rest, version, fmt.Errorf("incompatible encoding decoding version (%d > %d): %w", version, Version, err)
	}
	// return the rest of bytes
	return rest, version, nil
}

// CheckType extracts encoding byte from a raw encoded message
// checks it against the supported versions and returns the rest of rawInput (excluding encDecVersion bytes)
func CheckType(rawInput []byte, expectedType uint8) (rest []byte, err error) {
	t, r, err := utils.ReadUint8(rawInput)
	if err != nil {
		return r, fmt.Errorf("error checking type of the encoded entity: %w", err)
	}

	// error if type is known for this code
	if t >= typeUnsuported {
		return r, fmt.Errorf("unknown entity type in the encoded data (%d > %d)", t, typeUnsuported)
	}

	// error if type is known for this code
	if t != expectedType {
		return r, fmt.Errorf("unexpected entity type, got (%v) but (%v) was expected", Type(t), Type(expectedType))
	}

	// return the rest of bytes
	return r, nil
}

// EncodeKeyPart encodes a key part into a byte slice
func EncodeKeyPart(kp *ledger.KeyPart) []byte {
	if kp == nil {
		return []byte{}
	}
	// EncodingDecodingType
	buffer := utils.AppendUint16([]byte{}, Version)

	// encode key part entity type
	buffer = utils.AppendUint8(buffer, TypeKeyPart)

	// encode the key part content
	buffer = append(buffer, encodeKeyPart(kp)...)
	return buffer
}

func encodeKeyPart(kp *ledger.KeyPart) []byte {
	buffer := make([]byte, 0)

	// encode "Type" field of the key part
	buffer = utils.AppendUint16(buffer, kp.Type)

	// encode "Value" field of the key part
	buffer = append(buffer, kp.Value...)
	return buffer
}

// DecodeKeyPart constructs a key part from an encoded key part
func DecodeKeyPart(encodedKeyPart []byte) (*ledger.KeyPart, error) {
	// currently we ignore the version but in the future we
	// can do switch case based on the version if needed
	rest, _, err := CheckVersion(encodedKeyPart)
	if err != nil {
		return nil, fmt.Errorf("error decoding key part: %w", err)
	}

	// check the type
	rest, err = CheckType(rest, TypeKeyPart)
	if err != nil {
		return nil, fmt.Errorf("error decoding key part: %w", err)
	}

	// decode the key part content
	key, err := decodeKeyPart(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding key part: %w", err)
	}

	return key, nil
}

func decodeKeyPart(inp []byte) (*ledger.KeyPart, error) {
	// read key part type and the rest is the key item part
	kpt, kpv, err := utils.ReadUint16(inp)
	if err != nil {
		return nil, fmt.Errorf("error decoding key part (content): %w", err)
	}
	return &ledger.KeyPart{Type: kpt, Value: kpv}, nil
}

// EncodeKey encodes a key into a byte slice
func EncodeKey(k *ledger.Key) []byte {
	if k == nil {
		return []byte{}
	}
	// encode EncodingDecodingType
	buffer := utils.AppendUint16([]byte{}, Version)
	// encode key entity type
	buffer = utils.AppendUint8(buffer, TypeKey)
	// encode key content
	buffer = append(buffer, encodeKey(k)...)

	return buffer
}

// encodeKey encodes a key into a byte slice
func encodeKey(k *ledger.Key) []byte {
	buffer := make([]byte, 0)
	// encode number of key parts
	buffer = utils.AppendUint16(buffer, uint16(len(k.KeyParts)))
	// iterate over key parts
	for _, kp := range k.KeyParts {
		// encode the key part
		encKP := encodeKeyPart(&kp)
		// encode the len of the encoded key part
		buffer = utils.AppendUint32(buffer, uint32(len(encKP)))
		// append the encoded key part
		buffer = append(buffer, encKP...)
	}
	return buffer
}

// DecodeKey constructs a key from an encoded key part
func DecodeKey(encodedKey []byte) (*ledger.Key, error) {
	// check the enc dec version
	rest, _, err := CheckVersion(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding key: %w", err)
	}
	// check the encoding type
	rest, err = CheckType(rest, TypeKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding key: %w", err)
	}

	// decode the key content
	key, err := decodeKey(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding key: %w", err)
	}
	return key, nil
}

func decodeKey(inp []byte) (*ledger.Key, error) {
	key := &ledger.Key{}
	numOfParts, rest, err := utils.ReadUint16(inp)
	if err != nil {
		return nil, fmt.Errorf("error decoding key (content): %w", err)
	}

	for i := 0; i < int(numOfParts); i++ {
		var kpEncSize uint32
		var kpEnc []byte
		// read encoded key part size
		kpEncSize, rest, err = utils.ReadUint32(rest)
		if err != nil {
			return nil, fmt.Errorf("error decoding key (content): %w", err)
		}

		// read encoded key part
		kpEnc, rest, err = utils.ReadSlice(rest, int(kpEncSize))
		if err != nil {
			return nil, fmt.Errorf("error decoding key (content): %w", err)
		}

		// decode encoded key part
		kp, err := decodeKeyPart(kpEnc)
		if err != nil {
			return nil, fmt.Errorf("error decoding key (content): %w", err)
		}
		key.KeyParts = append(key.KeyParts, *kp)
	}
	return key, nil
}

// EncodeValue encodes a value into a byte slice
func EncodeValue(v ledger.Value) []byte {
	// encode EncodingDecodingType
	buffer := utils.AppendUint16([]byte{}, Version)

	// encode key entity type
	buffer = utils.AppendUint8(buffer, TypeValue)

	// encode value
	buffer = append(buffer, encodeValue(v)...)

	return buffer
}

func encodeValue(v ledger.Value) []byte {
	return v
}

// DecodeValue constructs a ledger value using an encoded byte slice
func DecodeValue(encodedValue []byte) (ledger.Value, error) {
	// check enc dec version
	rest, _, err := CheckVersion(encodedValue)
	if err != nil {
		return nil, err
	}

	// check the encoding type
	rest, err = CheckType(rest, TypeValue)
	if err != nil {
		return nil, err
	}

	return decodeValue(rest)
}

func decodeValue(inp []byte) (ledger.Value, error) {
	return ledger.Value(inp), nil
}

// EncodePath encodes a path into a byte slice
func EncodePath(p ledger.Path) []byte {
	// encode EncodingDecodingType
	buffer := utils.AppendUint16([]byte{}, Version)

	// encode key entity type
	buffer = utils.AppendUint8(buffer, TypePath)

	// encode path
	buffer = append(buffer, encodePath(p)...)

	return buffer
}

func encodePath(p ledger.Path) []byte {
	return p
}

// DecodePath constructs a path value using an encoded byte slice
func DecodePath(encodedPath []byte) (ledger.Path, error) {
	// check enc dec version
	rest, _, err := CheckVersion(encodedPath)
	if err != nil {
		return nil, err
	}

	// check the encoding type
	rest, err = CheckType(rest, TypePath)
	if err != nil {
		return nil, err
	}

	return decodePath(rest)
}

func decodePath(inp []byte) (ledger.Path, error) {
	return ledger.Path(inp), nil
}

// EncodePayload encodes a ledger payload
func EncodePayload(p *ledger.Payload) []byte {
	if p == nil {
		return []byte{}
	}
	// encode EncodingDecodingType
	buffer := utils.AppendUint16([]byte{}, Version)

	// encode key entity type
	buffer = utils.AppendUint8(buffer, TypePayload)

	// append encoded payload content
	buffer = append(buffer, encodePayload(p)...)

	return buffer
}

func encodePayload(p *ledger.Payload) []byte {
	buffer := make([]byte, 0)

	// encode key
	encK := encodeKey(&p.Key)

	// encode encoded key size
	buffer = utils.AppendUint32(buffer, uint32(len(encK)))

	// append encoded key content
	buffer = append(buffer, encK...)

	// encode value
	encV := encodeValue(p.Value)

	// encode encoded value size
	buffer = utils.AppendUint64(buffer, uint64(len(encV)))

	// append encoded key content
	buffer = append(buffer, encV...)

	return buffer
}

// DecodePayload construct a payload from an encoded byte slice
func DecodePayload(encodedPayload []byte) (*ledger.Payload, error) {
	// if empty don't decode
	if len(encodedPayload) == 0 {
		return nil, nil
	}
	// check the enc dec version
	rest, _, err := CheckVersion(encodedPayload)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}
	// check the encoding type
	rest, err = CheckType(rest, TypePayload)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}
	return decodePayload(rest)
}

func decodePayload(inp []byte) (*ledger.Payload, error) {

	// read encoded key size
	encKeySize, rest, err := utils.ReadUint32(inp)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}

	// read encoded key
	encKey, rest, err := utils.ReadSlice(rest, int(encKeySize))
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}

	// decode the key
	key, err := decodeKey(encKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}

	// read encoded value size
	encValeSize, rest, err := utils.ReadUint64(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}

	// read encoded value
	encValue, _, err := utils.ReadSlice(rest, int(encValeSize))
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}

	// decode value
	value, err := decodeValue(encValue)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}

	return &ledger.Payload{Key: *key, Value: value}, nil
}

// EncodeTrieUpdate encodes a trie update struct
func EncodeTrieUpdate(t *ledger.TrieUpdate) []byte {
	if t == nil {
		return []byte{}
	}
	// encode EncodingDecodingType
	buffer := utils.AppendUint16([]byte{}, Version)

	// encode key entity type
	buffer = utils.AppendUint8(buffer, TypeTrieUpdate)

	// append encoded payload content
	buffer = append(buffer, encodeTrieUpdate(t)...)

	return buffer
}

func encodeTrieUpdate(t *ledger.TrieUpdate) []byte {
	buffer := make([]byte, 0)

	// encode root hash (size and data)
	buffer = utils.AppendUint16(buffer, uint16(len(t.RootHash)))
	buffer = append(buffer, t.RootHash...)

	// encode number of paths
	buffer = utils.AppendUint32(buffer, uint32(t.Size()))

	if t.Size() == 0 {
		return buffer
	}

	// encode paths
	// encode path size (assuming all paths are the same size)
	buffer = utils.AppendUint16(buffer, uint16(t.Paths[0].Size()))
	for _, path := range t.Paths {
		buffer = append(buffer, encodePath(path)...)
	}

	// we assume same number of payloads
	// encode payloads
	for _, pl := range t.Payloads {
		encPl := encodePayload(pl)
		buffer = utils.AppendUint32(buffer, uint32(len(encPl)))
		buffer = append(buffer, encPl...)
	}

	return buffer
}

// DecodeTrieUpdate construct a trie update from an encoded byte slice
func DecodeTrieUpdate(encodedTrieUpdate []byte) (*ledger.TrieUpdate, error) {
	// if empty don't decode
	if len(encodedTrieUpdate) == 0 {
		return nil, nil
	}
	// check the enc dec version
	rest, _, err := CheckVersion(encodedTrieUpdate)
	if err != nil {
		return nil, fmt.Errorf("error decoding trie update: %w", err)
	}
	// check the encoding type
	rest, err = CheckType(rest, TypeTrieUpdate)
	if err != nil {
		return nil, fmt.Errorf("error decoding trie update: %w", err)
	}
	return decodeTrieUpdate(rest)
}

func decodeTrieUpdate(inp []byte) (*ledger.TrieUpdate, error) {

	paths := make([]ledger.Path, 0)
	payloads := make([]*ledger.Payload, 0)

	// decode root hash
	rhSize, rest, err := utils.ReadUint16(inp)
	if err != nil {
		return nil, fmt.Errorf("error decoding trie update: %w", err)
	}

	rh, rest, err := utils.ReadSlice(rest, int(rhSize))
	if err != nil {
		return nil, fmt.Errorf("error decoding trie update: %w", err)
	}

	// decode number of paths
	numOfPaths, rest, err := utils.ReadUint32(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding trie update: %w", err)
	}

	// decode path size
	pathSize, rest, err := utils.ReadUint16(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding trie update: %w", err)
	}

	var path ledger.Path
	var encPath []byte
	for i := 0; i < int(numOfPaths); i++ {
		encPath, rest, err = utils.ReadSlice(rest, int(pathSize))
		if err != nil {
			return nil, fmt.Errorf("error decoding trie update: %w", err)
		}
		path, err = decodePath(encPath)
		if err != nil {
			return nil, fmt.Errorf("error decoding trie update: %w", err)
		}
		paths = append(paths, path)
	}

	var payloadSize uint32
	var encPayload []byte
	var payload *ledger.Payload

	for i := 0; i < int(numOfPaths); i++ {
		payloadSize, rest, err = utils.ReadUint32(rest)
		if err != nil {
			return nil, fmt.Errorf("error decoding trie update: %w", err)
		}
		encPayload, rest, err = utils.ReadSlice(rest, int(payloadSize))
		if err != nil {
			return nil, fmt.Errorf("error decoding trie update: %w", err)
		}
		payload, err = decodePayload(encPayload)
		if err != nil {
			return nil, fmt.Errorf("error decoding trie update: %w", err)
		}
		payloads = append(payloads, payload)
	}
	return &ledger.TrieUpdate{RootHash: rh, Paths: paths, Payloads: payloads}, nil
}

// EncodeTrieProof encodes the content of a proof into a byte slice
func EncodeTrieProof(p *ledger.TrieProof) []byte {
	if p == nil {
		return []byte{}
	}
	// encode version
	buffer := utils.AppendUint16([]byte{}, Version)

	// encode proof entity type
	buffer = utils.AppendUint8(buffer, TypeProof)

	// append encoded proof content
	buffer = append(buffer, encodeTrieProof(p)...)

	return buffer
}

func encodeTrieProof(p *ledger.TrieProof) []byte {
	// first byte is reserved for inclusion flag
	buffer := make([]byte, 1)
	if p.Inclusion {
		// set the first bit to 1 if it is an inclusion proof
		buffer[0] |= 1 << 7
	}

	// steps are encoded as a single byte
	buffer = utils.AppendUint8(buffer, p.Steps)

	// include flags size and content
	buffer = utils.AppendUint8(buffer, uint8(len(p.Flags)))
	buffer = append(buffer, p.Flags...)

	// include path size and content
	buffer = utils.AppendUint16(buffer, uint16(p.Path.Size()))
	buffer = append(buffer, p.Path...)

	// include encoded payload size and content
	encPayload := encodePayload(p.Payload)
	buffer = utils.AppendUint64(buffer, uint64(len(encPayload)))
	buffer = append(buffer, encPayload...)

	// and finally include all interims (hash values)
	// number of interims
	buffer = utils.AppendUint8(buffer, uint8(len(p.Interims)))
	for _, inter := range p.Interims {
		buffer = utils.AppendUint16(buffer, uint16(len(inter)))
		buffer = append(buffer, inter...)
	}

	return buffer
}

// DecodeTrieProof construct a proof from an encoded byte slice
func DecodeTrieProof(encodedProof []byte) (*ledger.TrieProof, error) {
	// check the enc dec version
	rest, _, err := CheckVersion(encodedProof)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	// check the encoding type
	rest, err = CheckType(rest, TypeProof)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	return decodeTrieProof(rest)
}

func decodeTrieProof(inp []byte) (*ledger.TrieProof, error) {
	pInst := ledger.NewTrieProof()

	// Inclusion flag
	byteInclusion, rest, err := utils.ReadSlice(inp, 1)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	pInst.Inclusion = utils.Bit(byteInclusion, 0) == 1

	// read steps
	steps, rest, err := utils.ReadUint8(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	pInst.Steps = steps

	// read flags
	flagsSize, rest, err := utils.ReadUint8(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	flags, rest, err := utils.ReadSlice(rest, int(flagsSize))
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	pInst.Flags = flags

	// read path
	pathSize, rest, err := utils.ReadUint16(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	path, rest, err := utils.ReadSlice(rest, int(pathSize))
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	pInst.Path = path

	// read payload
	encPayloadSize, rest, err := utils.ReadUint64(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	encPayload, rest, err := utils.ReadSlice(rest, int(encPayloadSize))
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	payload, err := decodePayload(encPayload)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	pInst.Payload = payload

	// read interims
	interimsLen, rest, err := utils.ReadUint8(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	interims := make([][]byte, 0)

	var interimSize uint16
	var interim []byte

	for i := 0; i < int(interimsLen); i++ {
		interimSize, rest, err = utils.ReadUint16(rest)
		if err != nil {
			return nil, fmt.Errorf("error decoding proof: %w", err)
		}

		interim, rest, err = utils.ReadSlice(rest, int(interimSize))
		if err != nil {
			return nil, fmt.Errorf("error decoding proof: %w", err)
		}
		interims = append(interims, interim)
	}
	pInst.Interims = interims

	return pInst, nil
}

// EncodeTrieBatchProof encodes a batch proof into a byte slice
func EncodeTrieBatchProof(bp *ledger.TrieBatchProof) []byte {
	if bp == nil {
		return []byte{}
	}
	// encode version
	buffer := utils.AppendUint16([]byte{}, Version)

	// encode batch proof entity type
	buffer = utils.AppendUint8(buffer, TypeBatchProof)
	// encode batch proof content
	buffer = append(buffer, encodeTrieBatchProof(bp)...)

	return buffer
}

// encodeBatchProof encodes a batch proof into a byte slice
func encodeTrieBatchProof(bp *ledger.TrieBatchProof) []byte {
	buffer := make([]byte, 0)
	// encode number of proofs
	buffer = utils.AppendUint32(buffer, uint32(len(bp.Proofs)))
	// iterate over proofs
	for _, p := range bp.Proofs {
		// encode the proof
		encP := encodeTrieProof(p)
		// encode the len of the encoded proof
		buffer = utils.AppendUint64(buffer, uint64(len(encP)))
		// append the encoded proof
		buffer = append(buffer, encP...)
	}
	return buffer
}

// DecodeTrieBatchProof constructs a batch proof from an encoded byte slice
func DecodeTrieBatchProof(encodedBatchProof []byte) (*ledger.TrieBatchProof, error) {
	// check the enc dec version
	rest, _, err := CheckVersion(encodedBatchProof)
	if err != nil {
		return nil, fmt.Errorf("error decoding batch proof: %w", err)
	}
	// check the encoding type
	rest, err = CheckType(rest, TypeBatchProof)
	if err != nil {
		return nil, fmt.Errorf("error decoding batch proof: %w", err)
	}

	// decode the batch proof content
	bp, err := decodeTrieBatchProof(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding batch proof: %w", err)
	}
	return bp, nil
}

func decodeTrieBatchProof(inp []byte) (*ledger.TrieBatchProof, error) {
	bp := ledger.NewTrieBatchProof()
	// number of proofs
	numOfProofs, rest, err := utils.ReadUint32(inp)
	if err != nil {
		return nil, fmt.Errorf("error decoding batch proof (content): %w", err)
	}

	for i := 0; i < int(numOfProofs); i++ {
		var encProofSize uint64
		var encProof []byte
		// read encoded proof size
		encProofSize, rest, err = utils.ReadUint64(rest)
		if err != nil {
			return nil, fmt.Errorf("error decoding batch proof (content): %w", err)
		}

		// read encoded proof
		encProof, rest, err = utils.ReadSlice(rest, int(encProofSize))
		if err != nil {
			return nil, fmt.Errorf("error decoding batch proof (content): %w", err)
		}

		// decode encoded proof
		proof, err := decodeTrieProof(encProof)
		if err != nil {
			return nil, fmt.Errorf("error decoding batch proof (content): %w", err)
		}
		bp.Proofs = append(bp.Proofs, proof)
	}
	return bp, nil
}
