// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"encoding/hex"
	"fmt"
	"reflect"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/storage/merkle"
)

// Identifier represents a 32-byte unique identifier for an entity.
type Identifier [32]byte

var (
	// ZeroID is the lowest value in the 32-byte ID space.
	ZeroID = Identifier{}
)

var GenesisParentID = ZeroID
var GenesisExecutionResultParentID = ZeroID

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

// MakeID creates an ID from the hash of encoded data.
func MakeID(body interface{}) Identifier {
	data := encoding.DefaultEncoder.MustEncode(body)
	hasher := hash.NewSHA3_256()
	hash := hasher.ComputeHash(data)
	return HashToID(hash)
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
