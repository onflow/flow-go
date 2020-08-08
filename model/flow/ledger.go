package flow

import (
	"fmt"

	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/x/bsonx"
)

// RegisterID (key part of key value)
type RegisterID []byte

// RegisterValue (value part of Register)
type RegisterValue []byte

// StorageProof (proof of a read or update to the state, Merkle path of some sort)
type StorageProof = []byte

// StateCommitment holds the root hash of the tree (Snapshot)
type StateCommitment []byte

func (s StateCommitment) MarshalBSONValue() (bsontype.Type, []byte, error) {
	return bsonx.String(fmt.Sprintf("%x", s)).MarshalBSONValue()
}

func (s RegisterID) MarshalBSONValue() (bsontype.Type, []byte, error) {
	return bsonx.String(fmt.Sprintf("%x", s)).MarshalBSONValue()
}

func (s RegisterValue) MarshalBSONValue() (bsontype.Type, []byte, error) {
	return bsonx.String(fmt.Sprintf("%x", s)).MarshalBSONValue()
}
