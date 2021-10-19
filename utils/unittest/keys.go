package unittest

import (
	"encoding/hex"

	"github.com/onflow/flow-go/crypto"

	"github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/encodable"
)

func NetworkingKeys(n int) []crypto.PrivateKey {
	keys := make([]crypto.PrivateKey, 0, n)

	for i := 0; i < n; i++ {
		key := NetworkingPrivKeyFixture()
		keys = append(keys, key)
	}

	return keys
}

func StakingKeys(n int) []crypto.PrivateKey {
	keys := make([]crypto.PrivateKey, 0, n)

	for i := 0; i < n; i++ {
		key := StakingPrivKeyFixture()
		keys = append(keys, key)
	}

	return keys
}

func DKGParticipantPriv() *dkg.DKGParticipantPriv {
	privKey := StakingPrivKeyFixture()
	randBeaconKey := encodable.RandomBeaconPrivKey{
		PrivateKey: privKey,
	}
	return &dkg.DKGParticipantPriv{
		NodeID:              IdentifierFixture(),
		RandomBeaconPrivKey: randBeaconKey,
	}
}

func MustDecodePublicKeyHex(algo crypto.SigningAlgorithm, keyHex string) crypto.PublicKey {
	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		panic(err)
	}
	key, err := crypto.DecodePublicKey(algo, keyBytes)
	if err != nil {
		panic(err)
	}
	return key
}

func MustDecodeSignatureHex(sigHex string) crypto.Signature {
	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		panic(err)
	}
	return sigBytes
}
