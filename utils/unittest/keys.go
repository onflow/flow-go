package unittest

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/onflow/flow-go/crypto"

	"github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/encodable"
)

func NetworkingKey() (crypto.PrivateKey, error) {
	return ECDSAKey()
}

func NetworkingKeys(n int) ([]crypto.PrivateKey, error) {
	keys := make([]crypto.PrivateKey, 0, n)

	for i := 0; i < n; i++ {
		key, err := NetworkingKey()
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}

	return keys, nil
}

func ECDSAKey() (crypto.PrivateKey, error) {
	seed := make([]byte, crypto.KeyGenSeedMinLenECDSAP256)
	n, err := rand.Read(seed)
	if err != nil || n != crypto.KeyGenSeedMinLenECDSAP256 {
		return nil, err
	}

	sk, err := crypto.GeneratePrivateKey(crypto.ECDSAP256, seed)
	return sk, err
}

func StakingKey() (crypto.PrivateKey, error) {
	seed := make([]byte, crypto.KeyGenSeedMinLenBLSBLS12381)
	n, err := rand.Read(seed)
	if err != nil || n != crypto.KeyGenSeedMinLenBLSBLS12381 {
		return nil, err
	}

	sk, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
	return sk, err
}

func StakingKeys(n int) ([]crypto.PrivateKey, error) {
	keys := make([]crypto.PrivateKey, 0, n)

	for i := 0; i < n; i++ {
		key, err := StakingKey()
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}

	return keys, nil
}

func DKGParticipantPriv() *dkg.DKGParticipantPriv {
	privKey, _ := StakingKey()
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
