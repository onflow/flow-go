package storage

import (
	"github.com/onflow/flow-go/model/encodable"
)

type BeaconPrivateKeys interface {
	InsertMyBeaconPrivateKey(epochCounter uint64, key *encodable.RandomBeaconPrivKey) error
	RetrieveMyBeaconPrivateKey(epochCounter uint64) (*encodable.RandomBeaconPrivKey, error)
}
