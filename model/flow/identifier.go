// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"reflect"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/fingerprint"
	"github.com/onflow/flow-go/storage/merkle"
	_ "github.com/onflow/flow-go/utils/binstat"
)

// Identifier represents a 32-byte unique identifier for an entity.
type Identifier [32]byte

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
	hasher := hash.NewSHA3_256()
	hash := hasher.ComputeHash(data)
	id := HashToID(hash)
	//binstat.Leave(bs2)
	return id
}

// PublicKeyToID creates an ID from a public key.
func PublicKeyToID(pub crypto.PublicKey) (Identifier, error) {
	b := pub.Encode()
	hasher := hash.NewSHA3_256()
	hash := hasher.ComputeHash(b)
	return HashToID(hash), nil
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
	tree := merkle.NewTree()
	for _, id := range ids {
		tree.Put(id[:], nil)
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
	var sum Identifier
	hasher := hash.NewSHA3_256()
	for _, id := range ids {
		_, _ = hasher.Write(id[:])
	}
	hash := hasher.SumHash()
	copy(sum[:], hash)
	return sum
}

func CheckConcatSum(sum Identifier, fps ...Identifier) bool {
	computed := ConcatSum(fps...)
	return sum == computed
}

// Sample returns random sample of length 'size' of the ids
// [Fisher-Yates shuffle](https://en.wikipedia.org/wiki/Fisher–Yates_shuffle).
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
