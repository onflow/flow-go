// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
	"reflect"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"

	"github.com/onflow/flow-go/model/fingerprint"
	"github.com/onflow/flow-go/storage/merkle"
	_ "github.com/onflow/flow-go/utils/binstat"
)

const IdentifierLen = 32

// Identifier represents a 32-byte unique identifier for an entity.
type Identifier [IdentifierLen]byte

// IdentifierFilter is a filter on identifiers.
type IdentifierFilter func(Identifier) bool

var (
	// ZeroID is the lowest value in the 32-byte ID space.
	ZeroID = Identifier{}
)

// HexStringToIdentifier converts a hex string to an identifier. The input
// must be 64 characters long and contain only valid hex characters.
func HexStringToIdentifier(hexString string) (Identifier, error) {
	var identifier Identifier
	i, err := hex.Decode(identifier[:], []byte(hexString))
	if err != nil {
		return identifier, err
	}
	if i != 32 {
		return identifier, fmt.Errorf("malformed input, expected 32 bytes (64 characters), decoded %d", i)
	}
	return identifier, nil
}

func MustHexStringToIdentifier(hexString string) Identifier {
	id, err := HexStringToIdentifier(hexString)
	if err != nil {
		panic(err)
	}
	return id
}

// String returns the hex string representation of the identifier.
func (id Identifier) String() string {
	return hex.EncodeToString(id[:])
}

// Format handles formatting of id for different verbs. This is called when
// formatting an identifier with fmt.
func (id Identifier) Format(state fmt.State, verb rune) {
	switch verb {
	case 'x', 's', 'v':
		_, _ = state.Write([]byte(id.String()))
	default:
		_, _ = state.Write([]byte(fmt.Sprintf("%%!%c(%s=%s)", verb, reflect.TypeOf(id), id)))
	}
}

// IsSampled is a utility method to sample entities based on their ids
// the range is from [0, 64].
// 0 is 100% (all data will be collected)
// 32 is ~50%
// 64 is ~0% (no data will be collected)
func (id Identifier) IsSampled(sensitivity uint) bool {
	if sensitivity > 64 {
		return false
	}
	// take the first 8 bytes and check the first few bits based on sensitivity
	// higher sensitivity means more bits has to be zero, means less number of samples
	// sensitivity of zero, means everything is sampled
	return binary.BigEndian.Uint64(id[:8])>>uint64(64-sensitivity) == 0
}

func (id Identifier) MarshalText() ([]byte, error) {
	return []byte(id.String()), nil
}

func (id *Identifier) UnmarshalText(text []byte) error {
	var err error
	*id, err = HexStringToIdentifier(string(text))
	return err
}

func HashToID(hash []byte) Identifier {
	var id Identifier
	copy(id[:], hash)
	return id
}

// MakeID creates an ID from a hash of encoded data. MakeID uses `model.Fingerprint() []byte` to get the byte
// representation of the entity, which uses RLP to encode the data. If the input defines its own canonical encoding by
// implementing Fingerprinter, it uses that instead. That allows removal of non-unique fields from structs or
// overwriting of the used encoder. We are using Fingerprint instead of the default encoding for two reasons: a) JSON
// (the default encoding) does not specify an order for the elements of arrays and objects, which could lead to
// different hashes depending on the JSON implementation and b) the Fingerprinter interface allows to exclude fields not
// needed in the pre-image of the hash that comprises the Identifier, which could be different from the encoding for
// sending entities in messages or for storing them.
func MakeID(entity interface{}) Identifier {
	//bs1 := binstat.EnterTime(binstat.BinMakeID + ".??lock.Fingerprint")
	data := fingerprint.Fingerprint(entity)
	//binstat.LeaveVal(bs1, int64(len(data)))
	//bs2 := binstat.EnterTimeVal(binstat.BinMakeID+".??lock.Hash", int64(len(data)))
	var id Identifier
	hash.ComputeSHA3_256((*[32]byte)(&id), data)
	//binstat.Leave(bs2)
	return id
}

// PublicKeyToID creates an ID from a public key.
func PublicKeyToID(pk crypto.PublicKey) (Identifier, error) {
	var id Identifier
	pkBytes := pk.Encode()
	hash.ComputeSHA3_256((*[32]byte)(&id), pkBytes)
	return id, nil
}

// GetIDs gets the IDs for a slice of entities.
func GetIDs(value interface{}) []Identifier {
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Slice {
		panic(fmt.Sprintf("non-slice value (%T)", value))
	}
	slice := make([]Identifier, 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		entity, ok := v.Index(i).Interface().(Entity)
		if !ok {
			panic(fmt.Sprintf("slice contains non-entity (%T)", v.Index(i).Interface()))
		}
		slice = append(slice, entity.ID())
	}
	return slice
}

func MerkleRoot(ids ...Identifier) Identifier {
	var root Identifier
	tree, _ := merkle.NewTree(IdentifierLen) // we verify in a unit test that constructor does not error for this paramter
	for i, id := range ids {
		val := make([]byte, 8)
		binary.BigEndian.PutUint64(val, uint64(i))
		_, _ = tree.Put(id[:], val) // Tree copies keys and values internally
		// `Put` only errors for keys whose length does not conform to the pre-configured length. As
		// Identifiers are fixed-sized arrays, errors are impossible here, which we also verify in a unit test.
	}
	hash := tree.Hash()
	copy(root[:], hash)
	return root
}

func CheckMerkleRoot(root Identifier, ids ...Identifier) bool {
	computed := MerkleRoot(ids...)
	return root == computed
}

func ConcatSum(ids ...Identifier) Identifier {
	hasher := hash.NewSHA3_256()
	for _, id := range ids {
		_, _ = hasher.Write(id[:])
	}
	hash := hasher.SumHash()
	return HashToID(hash)
}

func CheckConcatSum(sum Identifier, fps ...Identifier) bool {
	computed := ConcatSum(fps...)
	return sum == computed
}

// Sample returns random sample of length 'size' of the ids
// [Fisher-Yates shuffle](https://en.wikipedia.org/wiki/Fisher-Yates_shuffle).
func Sample(size uint, ids ...Identifier) []Identifier {
	n := uint(len(ids))
	dup := make([]Identifier, 0, n)
	dup = append(dup, ids...)
	// if sample size is greater than total size, return all the elements
	if n <= size {
		return dup
	}
	for i := uint(0); i < size; i++ {
		j := uint(rand.Intn(int(n - i)))
		dup[i], dup[j+i] = dup[j+i], dup[i]
	}
	return dup[:size]
}

func CidToId(c cid.Cid) (Identifier, error) {
	decoded, err := mh.Decode(c.Hash())

	if err != nil {
		return ZeroID, fmt.Errorf("failed to decode CID: %w", err)
	}

	if decoded.Code != mh.SHA2_256 {
		return ZeroID, fmt.Errorf("unsupported CID hash function: %v", decoded.Name)
	}

	if decoded.Length != IdentifierLen {
		return ZeroID, fmt.Errorf("invalid CID length: %d", decoded.Length)
	}

	return HashToID(decoded.Digest), nil
}

func IdToCid(f Identifier) cid.Cid {
	hash, _ := mh.Encode(f[:], mh.SHA2_256)
	return cid.NewCidV0(hash)
}

func ByteSliceToId(b []byte) (Identifier, error) {
	var id Identifier
	if len(b) != IdentifierLen {
		return id, fmt.Errorf("illegal length for a flow identifier %x: got: %d, expected: %d", b, len(b), IdentifierLen)
	}

	copy(id[:], b[:])

	return id, nil
}

func ByteSlicesToIds(b [][]byte) (IdentifierList, error) {
	total := len(b)
	ids := make(IdentifierList, total)

	for i := 0; i < total; i++ {
		id, err := ByteSliceToId(b[i])
		if err != nil {
			return nil, err
		}

		ids[i] = id
	}

	return ids, nil
}

func IdsToBytes(identifiers []Identifier) [][]byte {
	var byteIds [][]byte
	for _, id := range identifiers {
		tempID := id // avoid capturing loop variable
		byteIds = append(byteIds, tempID[:])
	}

	return byteIds
}
