package common

import (
	"encoding/binary"
	"fmt"

	"github.com/dapperlabs/flow-go/ledger"
)

// EncodingDecodingVersion encoder/decoder code only supports
// decoding data with version smaller or equal to this value
// bumping this number prevents older versions of the code
// to deal with the newer version of data
// codes should be updated with backward compatibility if needed
// and act differently based on the encoding decoding version
const EncodingDecodingVersion = uint64(0)

// EncodingType capture the type of encoded entity
type EncodingType uint16

const (
	// EncodingTypeStateCommitment - encoding type for StateCommitments
	EncodingTypeStateCommitment = iota
	// EncodingTypeKeyPart - encoding type for KeyParts (a subset of key)
	EncodingTypeKeyPart
	// EncodingTypeKey - encoding type for Keys (unique identifier to reference a location in ledger)
	EncodingTypeKey
	// EncodingTypeValue - encoding type for Ledger Values
	EncodingTypeValue
	// EncodingTypePath - encoding type for Paths (trie storage location of a key value pair)
	EncodingTypePath
	// EncodingTypePayload - encoding type for Payloads (stored at trie nodes including key value pair )
	EncodingTypePayload
	// EncodingTypeProof encoding type for Proofs
	// (all data needed to verify a key value pair at specific stateCommitment)
	EncodingTypeProof
	// EncodingTypeBatchProof - encoding type for BatchProofs
	EncodingTypeBatchProof
	// EncodingTypeQuery - encoding type for ledger query
	EncodingTypeQuery
	// EncodingTypeUpdate - encoding type for ledger update
	EncodingTypeUpdate
	// EncodingTypeTrieUpdate - encoding type for trie update
	EncodingTypeTrieUpdate
	// encodingTypeUnknown - unknown encoding type - Warning this should always be the last item in the list
	encodingTypeUnknown
)

func (e EncodingType) String() string {
	return [...]string{"StateCommitment", "KeyPart", "Key", "Value", "Path", "Payload", "Proof", "BatchProof", "Unknown"}[e]
}

// checkEncDecVer extracts encoding bytes from a raw encoded message
// checks it against the supported versions and returns the rest of rawInput (excluding encDecVersion bytes)
func checkEncDecVer(rawInput []byte) (rest []byte, version uint64, err error) {
	version, rest, err = readUint64(rawInput)
	if err != nil {
		return rest, version, fmt.Errorf("error checking the encoding decoding version: %w", err)
	}
	// error on versions coming from future till a time-machine is invented
	if version > EncodingDecodingVersion {
		return rest, version, fmt.Errorf("incompatible encoding decoding version (%d > %d): %w", version, EncodingDecodingVersion, err)
	}
	// return the rest of bytes
	return rest, version, nil
}

// checkEncodingType extracts encoding byte from a raw encoded message
// checks it against the supported versions and returns the rest of rawInput (excluding encDecVersion bytes)
func checkEncodingType(rawInput []byte, expectedType uint16) (rest []byte, err error) {
	t, r, err := readUint16(rawInput)
	if err != nil {
		return r, fmt.Errorf("error checking type of the encoded entity: %w", err)
	}

	// error if type is known for this code
	if t >= encodingTypeUnknown {
		return r, fmt.Errorf("unknown entity type in the encoded data (%d > %d)", t, encodingTypeUnknown)
	}

	// error if type is known for this code
	if t != expectedType {
		return r, fmt.Errorf("unexpected entity type, got (%v) but (%v) was expected", EncodingType(t), EncodingType(expectedType))
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
	buffer := appendUint64([]byte{}, EncodingDecodingVersion)

	// encode key part entity type
	buffer = appendUint16(buffer, EncodingTypeKeyPart)

	// encode the key part content
	buffer = append(buffer, encodeKeyPart(kp)...)
	return buffer
}

func encodeKeyPart(kp *ledger.KeyPart) []byte {
	buffer := make([]byte, 0)

	// encode "Type" field of the key part
	buffer = appendUint16(buffer, kp.Type)

	// encode "Value" field of the key part
	buffer = append(buffer, kp.Value...)
	return buffer
}

// DecodeKeyPart constructs a key part from an encoded key part
func DecodeKeyPart(encodedKeyPart []byte) (*ledger.KeyPart, error) {
	// currently we ignore the version but in the future we
	// can do switch case based on the version if needed
	rest, _, err := checkEncDecVer(encodedKeyPart)
	if err != nil {
		return nil, fmt.Errorf("error decoding key part: %w", err)
	}

	// check the encoding type
	rest, err = checkEncodingType(rest, EncodingTypeKeyPart)
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
	kpt, kpv, err := readUint16(inp)
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
	buffer := appendUint64([]byte{}, EncodingDecodingVersion)
	// encode key entity type
	buffer = appendUint16(buffer, EncodingTypeKey)
	// encode key content
	buffer = append(buffer, encodeKey(k)...)

	return buffer
}

// encodeKey encodes a key into a byte slice
func encodeKey(k *ledger.Key) []byte {
	buffer := make([]byte, 0)
	// encode number of key parts
	buffer = appendUint16(buffer, uint16(len(k.KeyParts)))
	// iterate over key parts
	for _, kp := range k.KeyParts {
		// encode the key part
		encKP := encodeKeyPart(&kp)
		// encode the len of the encoded key part
		buffer = appendUint64(buffer, uint64(len(encKP)))
		// append the encoded key part
		buffer = append(buffer, encKP...)
	}
	return buffer
}

// DecodeKey constructs a key from an encoded key part
func DecodeKey(encodedKey []byte) (*ledger.Key, error) {
	// check the enc dec version
	rest, _, err := checkEncDecVer(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding key: %w", err)
	}
	// check the encoding type
	rest, err = checkEncodingType(rest, EncodingTypeKey)
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
	numOfParts, rest, err := readUint16(inp)
	if err != nil {
		return nil, fmt.Errorf("error decoding key (content): %w", err)
	}

	for i := 0; i < int(numOfParts); i++ {
		var kpEncSize uint64
		var kpEnc []byte
		// read encoded key part size
		kpEncSize, rest, err = readUint64(rest)
		if err != nil {
			return nil, fmt.Errorf("error decoding key (content): %w", err)
		}

		// read encoded key part
		kpEnc, rest, err = readSlice(rest, int(kpEncSize))
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
	buffer := appendUint64([]byte{}, EncodingDecodingVersion)

	// encode key entity type
	buffer = appendUint16(buffer, EncodingTypeValue)

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
	rest, _, err := checkEncDecVer(encodedValue)
	if err != nil {
		return nil, err
	}

	// check the encoding type
	rest, err = checkEncodingType(rest, EncodingTypeValue)
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
	buffer := appendUint64([]byte{}, EncodingDecodingVersion)

	// encode key entity type
	buffer = appendUint16(buffer, EncodingTypePath)

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
	rest, _, err := checkEncDecVer(encodedPath)
	if err != nil {
		return nil, err
	}

	// check the encoding type
	rest, err = checkEncodingType(rest, EncodingTypePath)
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
	buffer := appendUint64([]byte{}, EncodingDecodingVersion)

	// encode key entity type
	buffer = appendUint16(buffer, EncodingTypePayload)

	// append encoded payload content
	buffer = append(buffer, encodePayload(p)...)

	return buffer
}

func encodePayload(p *ledger.Payload) []byte {
	buffer := make([]byte, 0)

	// encode key
	encK := encodeKey(&p.Key)

	// encode encoded key size
	buffer = appendUint64(buffer, uint64(len(encK)))

	// append encoded key content
	buffer = append(buffer, encK...)

	// encode value
	encV := encodeValue(p.Value)

	// encode encoded value size
	buffer = appendUint64(buffer, uint64(len(encV)))

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
	rest, _, err := checkEncDecVer(encodedPayload)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}
	// check the encoding type
	rest, err = checkEncodingType(rest, EncodingTypePayload)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}
	return decodePayload(rest)
}

func decodePayload(inp []byte) (*ledger.Payload, error) {

	// read encoded key size
	encKeySize, rest, err := readUint64(inp)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}

	// read encoded key
	encKey, rest, err := readSlice(rest, int(encKeySize))
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}

	// decode the key
	key, err := decodeKey(encKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}

	// read encoded value size
	encValeSize, rest, err := readUint64(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}

	// read encoded value
	encValue, rest, err := readSlice(rest, int(encValeSize))
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
	buffer := appendUint64([]byte{}, EncodingDecodingVersion)

	// encode key entity type
	buffer = appendUint16(buffer, EncodingTypeTrieUpdate)

	// append encoded payload content
	buffer = append(buffer, encodeTrieUpdate(t)...)

	return buffer
}

func encodeTrieUpdate(t *ledger.TrieUpdate) []byte {
	buffer := make([]byte, 0)

	// encode state commitment (size and data)
	buffer = appendUint16(buffer, uint16(len(t.StateCommitment)))
	buffer = append(buffer, t.StateCommitment...)

	// encode number of paths
	buffer = appendUint32(buffer, uint32(t.Size()))

	if t.Size() == 0 {
		return buffer
	}

	// encode paths
	// encode path size (assuming all paths are the same size)
	buffer = appendUint16(buffer, uint16(t.Paths[0].Size()))
	for _, path := range t.Paths {
		buffer = append(buffer, encodePath(path)...)
	}

	// we assume same number of payloads
	// encode payloads
	for _, pl := range t.Payloads {
		encPl := encodePayload(pl)
		buffer = appendUint32(buffer, uint32(len(encPl)))
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
	rest, _, err := checkEncDecVer(encodedTrieUpdate)
	if err != nil {
		return nil, fmt.Errorf("error decoding trie update: %w", err)
	}
	// check the encoding type
	rest, err = checkEncodingType(rest, EncodingTypeTrieUpdate)
	if err != nil {
		return nil, fmt.Errorf("error decoding trie update: %w", err)
	}
	return decodeTrieUpdate(rest)
}

func decodeTrieUpdate(inp []byte) (*ledger.TrieUpdate, error) {

	paths := make([]ledger.Path, 0)
	payloads := make([]*ledger.Payload, 0)

	// decode state commitment
	scSize, rest, err := readUint16(inp)
	if err != nil {
		return nil, fmt.Errorf("error decoding trie update: %w", err)
	}

	sc, rest, err := readSlice(rest, int(scSize))
	if err != nil {
		return nil, fmt.Errorf("error decoding trie update: %w", err)
	}

	// decode number of paths
	numOfPaths, rest, err := readUint32(rest)

	// decode path size
	pathSize, rest, err := readUint16(rest)

	var path ledger.Path
	var encPath []byte
	for i := 0; i < int(numOfPaths); i++ {
		encPath, rest, err = readSlice(rest, int(pathSize))
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
		payloadSize, rest, err = readUint32(rest)
		if err != nil {
			return nil, fmt.Errorf("error decoding trie update: %w", err)
		}
		encPayload, rest, err = readSlice(rest, int(payloadSize))
		if err != nil {
			return nil, fmt.Errorf("error decoding trie update: %w", err)
		}
		payload, err = decodePayload(encPayload)
		if err != nil {
			return nil, fmt.Errorf("error decoding trie update: %w", err)
		}
		payloads = append(payloads, payload)
	}
	return &ledger.TrieUpdate{StateCommitment: sc, Paths: paths, Payloads: payloads}, nil
}

// EncodeProof encodes the content of a proof into a byte slice
func EncodeProof(p *ledger.Proof) []byte {
	if p == nil {
		return []byte{}
	}
	// encode EncodingDecodingType
	buffer := appendUint64([]byte{}, EncodingDecodingVersion)

	// encode key entity type
	buffer = appendUint16(buffer, EncodingTypeProof)

	// append encoded proof content
	buffer = append(buffer, encodeProof(p)...)

	return buffer
}

func encodeProof(p *ledger.Proof) []byte {
	// first byte is reserved for inclusion flag
	buffer := make([]byte, 1)
	if p.Inclusion {
		// set the first bit to 1 if it is an inclusion proof
		buffer[0] |= 1 << 7
	}

	// steps are encoded as a single byte
	buffer = appendUint8(buffer, p.Steps)

	// include flags size and content
	buffer = appendUint8(buffer, uint8(len(p.Flags)))
	buffer = append(buffer, p.Flags...)

	// include path size and content
	buffer = appendUint16(buffer, uint16(p.Path.Size()))
	buffer = append(buffer, p.Path...)

	// include encoded payload size and content
	encPayload := encodePayload(p.Payload)
	buffer = appendUint64(buffer, uint64(len(encPayload)))
	buffer = append(buffer, encPayload...)

	// and finally include all interims (hash values)
	// number of interims
	buffer = appendUint8(buffer, uint8(len(p.Interims)))
	for _, inter := range p.Interims {
		buffer = appendUint16(buffer, uint16(len(inter)))
		buffer = append(buffer, inter...)
	}

	return buffer
}

// DecodeProof construct a proof from an encoded byte slice
func DecodeProof(encodedProof []byte) (*ledger.Proof, error) {
	// check the enc dec version
	rest, _, err := checkEncDecVer(encodedProof)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	// check the encoding type
	rest, err = checkEncodingType(rest, EncodingTypeProof)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	return decodeProof(rest)
}

func decodeProof(inp []byte) (*ledger.Proof, error) {
	pInst := ledger.NewProof()

	// Inclusion flag
	byteInclusion, rest, err := readSlice(inp, 1)
	pInst.Inclusion, _ = IsBitSet(byteInclusion, 0)

	// read steps
	steps, rest, err := readUint8(rest)
	pInst.Steps = steps

	// read flags
	flagsSize, rest, err := readUint8(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	flags, rest, err := readSlice(rest, int(flagsSize))
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	pInst.Flags = flags

	// read path
	pathSize, rest, err := readUint16(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	path, rest, err := readSlice(rest, int(pathSize))
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	pInst.Path = path

	// read payload
	encPayloadSize, rest, err := readUint64(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	encPayload, rest, err := readSlice(rest, int(encPayloadSize))
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	payload, err := decodePayload(encPayload)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	pInst.Payload = payload

	// read interims
	interimsLen, rest, err := readUint8(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	interims := make([][]byte, 0)

	var interimSize uint16
	var interim []byte

	for i := 0; i < int(interimsLen); i++ {
		interimSize, rest, err = readUint16(rest)
		if err != nil {
			return nil, fmt.Errorf("error decoding proof: %w", err)
		}

		interim, rest, err = readSlice(rest, int(interimSize))
		if err != nil {
			return nil, fmt.Errorf("error decoding proof: %w", err)
		}
		interims = append(interims, interim)
	}
	pInst.Interims = interims

	return pInst, nil
}

// EncodeBatchProof encodes a batch proof into a byte slice
func EncodeBatchProof(bp *ledger.BatchProof) []byte {
	if bp == nil {
		return []byte{}
	}
	// encode EncodingDecodingType
	buffer := appendUint64([]byte{}, EncodingDecodingVersion)
	// encode key entity type
	buffer = appendUint16(buffer, EncodingTypeBatchProof)
	// encode batch proof content
	buffer = append(buffer, encodeBatchProof(bp)...)

	return buffer
}

// encodeBatchProof encodes a batch proof into a byte slice
func encodeBatchProof(bp *ledger.BatchProof) []byte {
	buffer := make([]byte, 0)
	// encode number of proofs
	buffer = appendUint32(buffer, uint32(len(bp.Proofs)))
	// iterate over proofs
	for _, p := range bp.Proofs {
		// encode the proof
		encP := encodeProof(p)
		// encode the len of the encoded proof
		buffer = appendUint64(buffer, uint64(len(encP)))
		// append the encoded proof
		buffer = append(buffer, encP...)
	}
	return buffer
}

// DecodeBatchProof constructs a batch proof from an encoded byte slice
func DecodeBatchProof(encodedBatchProof []byte) (*ledger.BatchProof, error) {
	// check the enc dec version
	rest, _, err := checkEncDecVer(encodedBatchProof)
	if err != nil {
		return nil, fmt.Errorf("error decoding batch proof: %w", err)
	}
	// check the encoding type
	rest, err = checkEncodingType(rest, EncodingTypeBatchProof)
	if err != nil {
		return nil, fmt.Errorf("error decoding batch proof: %w", err)
	}

	// decode the batch proof content
	bp, err := decodeBatchProof(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding batch proof: %w", err)
	}
	return bp, nil
}

func decodeBatchProof(inp []byte) (*ledger.BatchProof, error) {
	bp := ledger.NewBatchProof()
	// number of proofs
	numOfProofs, rest, err := readUint32(inp)
	if err != nil {
		return nil, fmt.Errorf("error decoding batch proof (content): %w", err)
	}

	for i := 0; i < int(numOfProofs); i++ {
		var encProofSize uint64
		var encProof []byte
		// read encoded proof size
		encProofSize, rest, err = readUint64(rest)
		if err != nil {
			return nil, fmt.Errorf("error decoding batch proof (content): %w", err)
		}

		// read encoded proof
		encProof, rest, err = readSlice(rest, int(encProofSize))
		if err != nil {
			return nil, fmt.Errorf("error decoding batch proof (content): %w", err)
		}

		// decode encoded proof
		proof, err := decodeProof(encProof)
		if err != nil {
			return nil, fmt.Errorf("error decoding batch proof (content): %w", err)
		}
		bp.Proofs = append(bp.Proofs, proof)
	}
	return bp, nil
}

func readSlice(input []byte, size int) (value []byte, rest []byte, err error) {
	if len(input) < size {
		return nil, input, fmt.Errorf("input size is too small to be splited %d < %d ", len(input), size)
	}
	return input[:size], input[size:], nil
}

func readUint8(input []byte) (value uint8, rest []byte, err error) {
	if len(input) < 1 {
		return 0, input, fmt.Errorf("input size (%d) is too small to read a uint8", len(input))
	}
	return uint8(input[0]), input[1:], nil
}

func readUint16(input []byte) (value uint16, rest []byte, err error) {
	if len(input) < 2 {
		return 0, input, fmt.Errorf("input size (%d) is too small to read a uint16", len(input))
	}
	return binary.BigEndian.Uint16(input[:2]), input[2:], nil
}

func readUint32(input []byte) (value uint32, rest []byte, err error) {
	if len(input) < 4 {
		return 0, input, fmt.Errorf("input size (%d) is too small to read a uint32", len(input))
	}
	return binary.BigEndian.Uint32(input[:4]), input[4:], nil
}

func readUint64(input []byte) (value uint64, rest []byte, err error) {
	if len(input) < 8 {
		return 0, input, fmt.Errorf("input size (%d) is too small to read a uint64", len(input))
	}
	return binary.BigEndian.Uint64(input[:8]), input[8:], nil
}

func writeUint8(buffer []byte, loc int, value uint8) int {
	buffer[loc] = byte(value)
	return loc + 1
}

func writeUint16(buffer []byte, loc int, value uint16) int {
	binary.BigEndian.PutUint16(buffer[loc:], value)
	return loc + 2
}

func writeUint32(buffer []byte, loc int, value uint32) int {
	binary.BigEndian.PutUint32(buffer[loc:], value)
	return loc + 4
}

func writeUint64(buffer []byte, loc int, value uint64) int {
	binary.BigEndian.PutUint64(buffer[loc:], value)
	return loc + 8
}

func appendUint8(input []byte, value uint8) []byte {
	return append(input, byte(value))
}

func appendUint16(input []byte, value uint16) []byte {
	buffer := make([]byte, 2)
	binary.BigEndian.PutUint16(buffer, value)
	return append(input, buffer...)
}

func appendUint32(input []byte, value uint32) []byte {
	buffer := make([]byte, 4)
	binary.BigEndian.PutUint32(buffer, value)
	return append(input, buffer...)
}

func appendUint64(input []byte, value uint64) []byte {
	buffer := make([]byte, 8)
	binary.BigEndian.PutUint64(buffer, value)
	return append(input, buffer...)
}
