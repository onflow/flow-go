package flow

import (
	"encoding/hex"
)

// RegisterID (key part of key value)
type RegisterID = []byte

// RegisterValue (value part of Register)
type RegisterValue = []byte

// StorageProof (proof of a read or update to the state, Merkle path of some sort)
type StorageProof = []byte

// StateCommitment holds the root hash of the tree (Snapshot)
type StateCommitment = []byte

// Below won't be needed once we agree the content of genesis block
// but for now it allows root account to exist

// Used below with random root key
//privateKey := flow.AccountPrivateKey{
//	PrivateKey: rootKey,
//	SignAlgo:   crypto.ECDSAP256,
//	HashAlgo:   hash.SHA2_256,
//}

const ServiceAccountPrivateKeyHex = "e3a08ae3d0461cfed6d6f49bfc25fa899351c39d1bd21fdba8c87595b6c49bb4cc430201"

// Pre-calculated state commitment with root account with the above private key
const GenesistStateCommitmentHex = "71a3275f197df8bd959f3641c562e42df2247d2c2a25050f901eee0a80c14bb1"

var GenesisStateCommitment StateCommitment
var ServiceAccountPrivateKey AccountPrivateKey
var ServiceAccountPublicKey AccountPublicKey

func init() {
	var err error
	GenesisStateCommitment, err = hex.DecodeString(GenesistStateCommitmentHex)
	if err != nil {
		panic("error while hex decoding hardcoded state commitment")
	}

	serviceAccountPrivateKeyBytes, err := hex.DecodeString(ServiceAccountPrivateKeyHex)
	if err != nil {
		panic("error while hex decoding hardcoded root key")
	}

	ServiceAccountPrivateKey, err = DecodeAccountPrivateKey(serviceAccountPrivateKeyBytes)
	if err != nil {
		panic("error while decoding hardcoded root key bytes")
	}

	// Cannot import virtual machine, due to circular dependency. Just use the value of
	// virtualmachine.AccountKeyWeightThreshold here
	ServiceAccountPublicKey = ServiceAccountPrivateKey.PublicKey(1000)
}
