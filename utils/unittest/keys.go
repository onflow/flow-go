package unittest

import (
	"encoding/hex"

	"github.com/onflow/flow-go/crypto"

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

func RandomBeaconPriv() *encodable.RandomBeaconPrivKey {
	privKey := StakingPrivKeyFixture()
	return &encodable.RandomBeaconPrivKey{
		PrivateKey: privKey,
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
