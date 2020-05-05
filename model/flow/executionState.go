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

const RootAccountPrivateKeyHex = "e3a08ae3d0461cfed6d6f49bfc25fa899351c39d1bd21fdba8c87595b6c49bb4cc430201"

// Pre-calculated state commitment with root account with the above private key
const GenesistStateCommitmentHex = "6420cf162f6790c3e8fc09b66ae9a441bd7d7512065b2a9bccb9b4edb09a49e8"

var GenesisStateCommitment StateCommitment
var RootAccountPrivateKey AccountPrivateKey

func init() {
	var err error
	GenesisStateCommitment, err = hex.DecodeString(GenesistStateCommitmentHex)
	if err != nil {
		panic("error while hex decoding hardcoded state commitment")
	}

	rootAccountPrivateKeyBytes, err := hex.DecodeString(RootAccountPrivateKeyHex)
	if err != nil {
		panic("error while hex decoding hardcoded root key")
	}

	RootAccountPrivateKey, err = DecodeAccountPrivateKey(rootAccountPrivateKeyBytes)
	if err != nil {
		panic("error while decoding hardcoded root key bytes")
	}
}
