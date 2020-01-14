// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"fmt"
	"reflect"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/storage/merkle"
)

// Identifier represents a 32-byte unique identifier for a node.
type Identifier [32]byte

var ZeroID = Identifier{}

func HashToID(hash []byte) Identifier {
	var id Identifier
	copy(id[:], hash)
	return id
}

// MakeID creates an ID from the hash of encoded data.
func MakeID(body interface{}) Identifier {
	data := encoding.DefaultEncoder.MustEncode(body)
	hasher, _ := crypto.NewHasher(crypto.SHA3_256)
	hash := hasher.ComputeHash(data)
	return HashToID(hash)
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
	hasher, _ := crypto.NewHasher(crypto.SHA3_256)
	for _, id := range ids {
		hasher.Add(id[:])
	}
	hash := hasher.SumHash()
	copy(sum[:], hash)
	return sum
}

func CheckConcatSum(sum Identifier, fps ...Identifier) bool {
	computed := ConcatSum(fps...)
	return sum == computed
}
