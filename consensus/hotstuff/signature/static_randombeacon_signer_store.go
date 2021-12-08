package signature

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/module"
)

// StaticRandomBeaconSignerStore is a simple implementation of module.RandomBeaconKeyStore
// that returns same key for each view. This structure was implemented for bootstrap process
// and should be used only for it.
type StaticRandomBeaconSignerStore struct {
	beaconKey crypto.PrivateKey
}

var _ module.RandomBeaconKeyStore = (*StaticRandomBeaconSignerStore)(nil)

func NewStaticRandomBeaconSignerStore(beaconKey crypto.PrivateKey) *StaticRandomBeaconSignerStore {
	return &StaticRandomBeaconSignerStore{
		beaconKey: beaconKey,
	}
}

func (s *StaticRandomBeaconSignerStore) ByView(_ uint64) (crypto.PrivateKey, error) {
	return s.beaconKey, nil
}
