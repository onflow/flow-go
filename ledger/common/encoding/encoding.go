// Package encoding provides byte serialization and deserialization of trie and ledger structs.
package encoding

import (
	"fmt"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/bitutils"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/utils"
)

// Versions capture the maximum version of encoding this code supports.
// I.e. this code encodes data with the latest version and only decodes
// data with version smaller or equal to these versions.
// Bumping a version number prevents older versions of code from handling
// the newer version of data. New code handling new data version
// should be updated to also support backward compatibility if needed.
const (
	PayloadVersion        = uint16(1)
	TrieUpdateVersion     = uint16(0) // Use payload version 0 encoding
	TrieProofVersion      = uint16(0) // Use payload version 0 encoding
	TrieBatchProofVersion = uint16(0) // Use payload version 0 encoding
)

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
func CheckVersion(rawInput []byte, maxVersion uint16) (rest []byte, version uint16, err error) {
	version, rest, err = utils.ReadUint16(rawInput)
	if err != nil {
		return rest, version, fmt.Errorf("error checking the encoding decoding version: %w", err)
	}
	// error on versions coming from future till a time-machine is invented
	if version > maxVersion {
		return rest, version, fmt.Errorf("incompatible encoding decoding version (%d > %d): %w", version, maxVersion, err)
	}
	// return the rest of bytes
	return rest, version, nil
}

// CheckType extracts encoding byte from a raw encoded message
// checks it against expected type and returns the rest of rawInput (excluding type byte)
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
	// encode version
	buffer := utils.AppendUint16([]byte{}, PayloadVersion)

	// encode key part entity type
	buffer = utils.AppendUint8(buffer, TypeKeyPart)

	// encode the key part content
	buffer = append(buffer, encodeKeyPart(kp, PayloadVersion)...)
	return buffer
}

func encodeKeyPart(kp *ledger.KeyPart, version uint16) []byte {
	buffer := make([]byte, 0, encodedKeyPartLength(kp, version))
	return encodeAndAppendKeyPart(buffer, kp, version)
}

func encodeAndAppendKeyPart(buffer []byte, kp *ledger.KeyPart, _ uint16) []byte {
	// encode "Type" field of the key part
	buffer = utils.AppendUint16(buffer, kp.Type)

	// encode "Value" field of the key part
	buffer = append(buffer, kp.Value...)

	return buffer
}

func encodedKeyPartLength(kp *ledger.KeyPart, _ uint16) int {
	// Key part is encoded as: type (2 bytes) + value
	return 2 + len(kp.Value)
}

// DecodeKeyPart constructs a key part from an encoded key part
func DecodeKeyPart(encodedKeyPart []byte) (*ledger.KeyPart, error) {
	rest, version, err := CheckVersion(encodedKeyPart, PayloadVersion)
	if err != nil {
		return nil, fmt.Errorf("error decoding key part: %w", err)
	}

	// check the type
	rest, err = CheckType(rest, TypeKeyPart)
	if err != nil {
		return nil, fmt.Errorf("error decoding key part: %w", err)
	}

	// decode the key part content (zerocopy)
	key, err := decodeKeyPart(rest, true, version)
	if err != nil {
		return nil, fmt.Errorf("error decoding key part: %w", err)
	}

	return key, nil
}

// decodeKeyPart decodes inp into KeyPart. If zeroCopy is true, KeyPart
// references data in inp.  Otherwise, it is copied.
func decodeKeyPart(inp []byte, zeroCopy bool, _ uint16) (*ledger.KeyPart, error) {
	// read key part type and the rest is the key item part
	kpt, kpv, err := utils.ReadUint16(inp)
	if err != nil {
		return nil, fmt.Errorf("error decoding key part (content): %w", err)
	}
	if zeroCopy {
		return &ledger.KeyPart{Type: kpt, Value: kpv}, nil
	}
	v := make([]byte, len(kpv))
	copy(v, kpv)
	return &ledger.KeyPart{Type: kpt, Value: v}, nil
}

// EncodeKey encodes a key into a byte slice
func EncodeKey(k *ledger.Key) []byte {
	if k == nil {
		return []byte{}
	}
	// encode EncodingDecodingType
	buffer := utils.AppendUint16([]byte{}, PayloadVersion)
	// encode key entity type
	buffer = utils.AppendUint8(buffer, TypeKey)
	// encode key content
	buffer = append(buffer, encodeKey(k, PayloadVersion)...)

	return buffer
}

// encodeKey encodes a key into a byte slice
func encodeKey(k *ledger.Key, version uint16) []byte {
	buffer := make([]byte, 0, encodedKeyLength(k, version))
	return encodeAndAppendKey(buffer, k, version)
}

func encodeAndAppendKey(buffer []byte, k *ledger.Key, version uint16) []byte {
	// encode number of key parts
	buffer = utils.AppendUint16(buffer, uint16(len(k.KeyParts)))

	// iterate over key parts
	for _, kp := range k.KeyParts {
		// encode the len of the encoded key part
		buffer = utils.AppendUint32(buffer, uint32(encodedKeyPartLength(&kp, version)))

		// encode the key part
		buffer = encodeAndAppendKeyPart(buffer, &kp, version)
	}

	return buffer
}

func encodedKeyLength(k *ledger.Key, version uint16) int {
	// Key is encoded as: number of key parts (2 bytes) and for each key part,
	// the key part size (4 bytes) + encoded key part (n bytes).
	size := 2 + 4*len(k.KeyParts)
	for _, kp := range k.KeyParts {
		size += encodedKeyPartLength(&kp, version)
	}
	return size
}

// DecodeKey constructs a key from an encoded key part
func DecodeKey(encodedKey []byte) (*ledger.Key, error) {
	// check the enc dec version
	rest, version, err := CheckVersion(encodedKey, PayloadVersion)
	if err != nil {
		return nil, fmt.Errorf("error decoding key: %w", err)
	}
	// check the encoding type
	rest, err = CheckType(rest, TypeKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding key: %w", err)
	}

	// decode the key content (zerocopy)
	key, err := decodeKey(rest, true, version)
	if err != nil {
		return nil, fmt.Errorf("error decoding key: %w", err)
	}
	return key, nil
}

// decodeKey decodes inp into Key. If zeroCopy is true, returned key
// references data in inp.  Otherwise, it is copied.
func decodeKey(inp []byte, zeroCopy bool, version uint16) (*ledger.Key, error) {
	key := &ledger.Key{}

	numOfParts, rest, err := utils.ReadUint16(inp)
	if err != nil {
		return nil, fmt.Errorf("error decoding key (content): %w", err)
	}

	if numOfParts == 0 {
		return key, nil
	}

	key.KeyParts = make([]ledger.KeyPart, numOfParts)

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
		kp, err := decodeKeyPart(kpEnc, zeroCopy, version)
		if err != nil {
			return nil, fmt.Errorf("error decoding key (content): %w", err)
		}

		key.KeyParts[i] = *kp
	}
	return key, nil
}

// EncodeValue encodes a value into a byte slice
func EncodeValue(v ledger.Value) []byte {
	// encode EncodingDecodingType
	buffer := utils.AppendUint16([]byte{}, PayloadVersion)

	// encode key entity type
	buffer = utils.AppendUint8(buffer, TypeValue)

	// encode value
	buffer = append(buffer, encodeValue(v, PayloadVersion)...)

	return buffer
}

func encodeValue(v ledger.Value, _ uint16) []byte {
	return v
}

func encodeAndAppendValue(buffer []byte, v ledger.Value, _ uint16) []byte {
	return append(buffer, v...)
}

func encodedValueLength(v ledger.Value, _ uint16) int {
	return len(v)
}

// DecodeValue constructs a ledger value using an encoded byte slice
func DecodeValue(encodedValue []byte) (ledger.Value, error) {
	// check enc dec version
	rest, _, err := CheckVersion(encodedValue, PayloadVersion)
	if err != nil {
		return nil, err
	}

	// check the encoding type
	rest, err = CheckType(rest, TypeValue)
	if err != nil {
		return nil, err
	}

	return rest, nil
}

// EncodePayload encodes a ledger payload
func EncodePayload(p *ledger.Payload) []byte {
	if p == nil {
		return []byte{}
	}
	// encode EncodingDecodingType
	buffer := utils.AppendUint16([]byte{}, PayloadVersion)

	// encode key entity type
	buffer = utils.AppendUint8(buffer, TypePayload)

	// append encoded payload content
	buffer = append(buffer, encodePayload(p, PayloadVersion)...)

	return buffer
}

// EncodeAndAppendPayloadWithoutPrefix encodes a ledger payload
// without prefix (version and type) and appends to buffer.
// If payload is nil, unmodified buffer is returned.
func EncodeAndAppendPayloadWithoutPrefix(buffer []byte, p *ledger.Payload, version uint16) []byte {
	if p == nil {
		return buffer
	}
	return encodeAndAppendPayload(buffer, p, version)
}

func EncodedPayloadLengthWithoutPrefix(p *ledger.Payload, version uint16) int {
	return encodedPayloadLength(p, version)
}

func encodePayload(p *ledger.Payload, version uint16) []byte {
	buffer := make([]byte, 0, encodedPayloadLength(p, version))
	return encodeAndAppendPayload(buffer, p, version)
}

func encodeAndAppendPayload(buffer []byte, p *ledger.Payload, version uint16) []byte {

	// encode encoded key size
	buffer = utils.AppendUint32(buffer, uint32(encodedKeyLength(&p.Key, version)))

	// encode key
	buffer = encodeAndAppendKey(buffer, &p.Key, version)

	// encode encoded value size
	encodedValueLen := encodedValueLength(p.Value, version)
	switch version {
	case 0:
		// In version 0, encoded value length is encoded as 8 bytes.
		buffer = utils.AppendUint64(buffer, uint64(encodedValueLen))
	default:
		// In version 1 and later, encoded value length is encoded as 4 bytes.
		buffer = utils.AppendUint32(buffer, uint32(encodedValueLen))
	}

	// encode value
	buffer = encodeAndAppendValue(buffer, p.Value, version)

	return buffer
}

func encodedPayloadLength(p *ledger.Payload, version uint16) int {
	if p == nil {
		return 0
	}
	switch version {
	case 0:
		// In version 0, payload is encoded as:
		//   encode key length (4 bytes) + encoded key +
		//   encoded value length (8 bytes) + encode value
		return 4 + encodedKeyLength(&p.Key, version) + 8 + encodedValueLength(p.Value, version)
	default:
		// In version 1 and later, payload is encoded as:
		//   encode key length (4 bytes) + encoded key +
		//   encoded value length (4 bytes) + encode value
		return 4 + encodedKeyLength(&p.Key, version) + 4 + encodedValueLength(p.Value, version)
	}
}

// DecodePayload construct a payload from an encoded byte slice
func DecodePayload(encodedPayload []byte) (*ledger.Payload, error) {
	// if empty don't decode
	if len(encodedPayload) == 0 {
		return nil, nil
	}
	// check the enc dec version
	rest, version, err := CheckVersion(encodedPayload, PayloadVersion)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}
	// check the encoding type
	rest, err = CheckType(rest, TypePayload)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}
	// decode payload (zerocopy)
	return decodePayload(rest, true, version)
}

// DecodePayloadWithoutPrefix constructs a payload from encoded byte slice
// without prefix (version and type). If zeroCopy is true, returned payload
// references data in encodedPayload. Otherwise, it is copied.
func DecodePayloadWithoutPrefix(encodedPayload []byte, zeroCopy bool, version uint16) (*ledger.Payload, error) {
	// if empty don't decode
	if len(encodedPayload) == 0 {
		return nil, nil
	}
	return decodePayload(encodedPayload, zeroCopy, version)
}

// decodePayload decodes inp into payload.  If zeroCopy is true,
// returned payload references data in inp.  Otherwise, it is copied.
func decodePayload(inp []byte, zeroCopy bool, version uint16) (*ledger.Payload, error) {

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
	key, err := decodeKey(encKey, zeroCopy, version)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}

	// read encoded value size
	var encValueSize int
	switch version {
	case 0:
		var size uint64
		size, rest, err = utils.ReadUint64(rest)
		encValueSize = int(size)
	default:
		var size uint32
		size, rest, err = utils.ReadUint32(rest)
		encValueSize = int(size)
	}

	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}

	// read encoded value
	encValue, _, err := utils.ReadSlice(rest, encValueSize)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}

	if zeroCopy {
		return &ledger.Payload{Key: *key, Value: encValue}, nil
	}

	v := make([]byte, len(encValue))
	copy(v, encValue)
	return &ledger.Payload{Key: *key, Value: v}, nil
}

// EncodeTrieUpdate encodes a trie update struct
func EncodeTrieUpdate(t *ledger.TrieUpdate) []byte {
	if t == nil {
		return []byte{}
	}
	// encode EncodingDecodingType
	buffer := utils.AppendUint16([]byte{}, TrieUpdateVersion)

	// encode key entity type
	buffer = utils.AppendUint8(buffer, TypeTrieUpdate)

	// append encoded payload content
	buffer = append(buffer, encodeTrieUpdate(t, TrieUpdateVersion)...)

	return buffer
}

func encodeTrieUpdate(t *ledger.TrieUpdate, version uint16) []byte {
	buffer := make([]byte, 0)

	// encode root hash (size and data)
	buffer = utils.AppendUint16(buffer, uint16(len(t.RootHash)))
	buffer = append(buffer, t.RootHash[:]...)

	// encode number of paths
	buffer = utils.AppendUint32(buffer, uint32(t.Size()))

	if t.Size() == 0 {
		return buffer
	}

	// encode paths
	// encode path size (assuming all paths are the same size)
	buffer = utils.AppendUint16(buffer, uint16(ledger.PathLen))
	for _, path := range t.Paths {
		buffer = append(buffer, path[:]...)
	}

	// we assume same number of payloads
	// encode payloads
	for _, pl := range t.Payloads {
		encPl := encodePayload(pl, version)
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
	rest, version, err := CheckVersion(encodedTrieUpdate, TrieUpdateVersion)
	if err != nil {
		return nil, fmt.Errorf("error decoding trie update: %w", err)
	}
	// check the encoding type
	rest, err = CheckType(rest, TypeTrieUpdate)
	if err != nil {
		return nil, fmt.Errorf("error decoding trie update: %w", err)
	}
	return decodeTrieUpdate(rest, version)
}

func decodeTrieUpdate(inp []byte, version uint16) (*ledger.TrieUpdate, error) {

	// decode root hash
	rhSize, rest, err := utils.ReadUint16(inp)
	if err != nil {
		return nil, fmt.Errorf("error decoding trie update: %w", err)
	}

	rhBytes, rest, err := utils.ReadSlice(rest, int(rhSize))
	if err != nil {
		return nil, fmt.Errorf("error decoding trie update: %w", err)
	}
	rh, err := ledger.ToRootHash(rhBytes)
	if err != nil {
		return nil, fmt.Errorf("decode trie update failed: %w", err)
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

	paths := make([]ledger.Path, numOfPaths)
	payloads := make([]*ledger.Payload, numOfPaths)

	var path ledger.Path
	var encPath []byte
	for i := 0; i < int(numOfPaths); i++ {
		encPath, rest, err = utils.ReadSlice(rest, int(pathSize))
		if err != nil {
			return nil, fmt.Errorf("error decoding trie update: %w", err)
		}
		path, err = ledger.ToPath(encPath)
		if err != nil {
			return nil, fmt.Errorf("error decoding trie update: %w", err)
		}
		paths[i] = path
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
		// Decode payload (zerocopy)
		payload, err = decodePayload(encPayload, true, version)
		if err != nil {
			return nil, fmt.Errorf("error decoding trie update: %w", err)
		}
		payloads[i] = payload
	}
	return &ledger.TrieUpdate{RootHash: rh, Paths: paths, Payloads: payloads}, nil
}

// EncodeTrieProof encodes the content of a proof into a byte slice
func EncodeTrieProof(p *ledger.TrieProof) []byte {
	if p == nil {
		return []byte{}
	}
	// encode version
	buffer := utils.AppendUint16([]byte{}, TrieProofVersion)

	// encode proof entity type
	buffer = utils.AppendUint8(buffer, TypeProof)

	// append encoded proof content
	proof := encodeTrieProof(p, TrieProofVersion)
	buffer = append(buffer, proof...)

	return buffer
}

func encodeTrieProof(p *ledger.TrieProof, version uint16) []byte {
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
	buffer = utils.AppendUint16(buffer, uint16(ledger.PathLen))
	buffer = append(buffer, p.Path[:]...)

	// include encoded payload size and content
	encPayload := encodePayload(p.Payload, version)
	buffer = utils.AppendUint64(buffer, uint64(len(encPayload)))
	buffer = append(buffer, encPayload...)

	// and finally include all interims (hash values)
	// number of interims
	buffer = utils.AppendUint8(buffer, uint8(len(p.Interims)))
	for _, inter := range p.Interims {
		buffer = utils.AppendUint16(buffer, uint16(len(inter)))
		buffer = append(buffer, inter[:]...)
	}

	return buffer
}

// DecodeTrieProof construct a proof from an encoded byte slice
func DecodeTrieProof(encodedProof []byte) (*ledger.TrieProof, error) {
	// check the enc dec version
	rest, version, err := CheckVersion(encodedProof, TrieProofVersion)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	// check the encoding type
	rest, err = CheckType(rest, TypeProof)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	return decodeTrieProof(rest, version)
}

func decodeTrieProof(inp []byte, version uint16) (*ledger.TrieProof, error) {
	pInst := ledger.NewTrieProof()

	// Inclusion flag
	byteInclusion, rest, err := utils.ReadSlice(inp, 1)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	pInst.Inclusion = bitutils.ReadBit(byteInclusion, 0) == 1

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
	pInst.Path, err = ledger.ToPath(path)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}

	// read payload
	encPayloadSize, rest, err := utils.ReadUint64(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	encPayload, rest, err := utils.ReadSlice(rest, int(encPayloadSize))
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	// Decode payload (zerocopy)
	payload, err := decodePayload(encPayload, true, version)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	pInst.Payload = payload

	// read interims
	interimsLen, rest, err := utils.ReadUint8(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}

	interims := make([]hash.Hash, interimsLen)

	var interimSize uint16
	var interim hash.Hash
	var interimBytes []byte

	for i := 0; i < int(interimsLen); i++ {
		interimSize, rest, err = utils.ReadUint16(rest)
		if err != nil {
			return nil, fmt.Errorf("error decoding proof: %w", err)
		}

		interimBytes, rest, err = utils.ReadSlice(rest, int(interimSize))
		if err != nil {
			return nil, fmt.Errorf("error decoding proof: %w", err)
		}
		interim, err = hash.ToHash(interimBytes)
		if err != nil {
			return nil, fmt.Errorf("error decoding proof: %w", err)
		}

		interims[i] = interim
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
	buffer := utils.AppendUint16([]byte{}, TrieBatchProofVersion)

	// encode batch proof entity type
	buffer = utils.AppendUint8(buffer, TypeBatchProof)
	// encode batch proof content
	buffer = append(buffer, encodeTrieBatchProof(bp, TrieBatchProofVersion)...)

	return buffer
}

// encodeBatchProof encodes a batch proof into a byte slice
func encodeTrieBatchProof(bp *ledger.TrieBatchProof, version uint16) []byte {
	buffer := make([]byte, 0)
	// encode number of proofs
	buffer = utils.AppendUint32(buffer, uint32(len(bp.Proofs)))
	// iterate over proofs
	for _, p := range bp.Proofs {
		// encode the proof
		encP := encodeTrieProof(p, version)
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
	rest, version, err := CheckVersion(encodedBatchProof, TrieBatchProofVersion)
	if err != nil {
		return nil, fmt.Errorf("error decoding batch proof: %w", err)
	}
	// check the encoding type
	rest, err = CheckType(rest, TypeBatchProof)
	if err != nil {
		return nil, fmt.Errorf("error decoding batch proof: %w", err)
	}

	// decode the batch proof content
	bp, err := decodeTrieBatchProof(rest, version)
	if err != nil {
		return nil, fmt.Errorf("error decoding batch proof: %w", err)
	}
	return bp, nil
}

func decodeTrieBatchProof(inp []byte, version uint16) (*ledger.TrieBatchProof, error) {
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
		proof, err := decodeTrieProof(encProof, version)
		if err != nil {
			return nil, fmt.Errorf("error decoding batch proof (content): %w", err)
		}
		bp.Proofs = append(bp.Proofs, proof)
	}
	return bp, nil
}
