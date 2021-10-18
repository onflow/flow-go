package encodable

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
)

func isHexString(enc []byte) error {
	str := string(enc)
	if len(str) <= 2 {
		if str == "\"\"" {
			return nil
		}
		return fmt.Errorf("invalid hex: %v", str)
	}

	if str == "null" {
		return nil
	}

	// remove the "
	str = str[1 : len(str)-1]
	_, err := hex.DecodeString(str)
	return err
}

func TestIsHexString(t *testing.T) {
	require.NoError(t, isHexString([]byte("abcd")))
	require.NoError(t, isHexString([]byte("\"\"")))
	require.NoError(t, isHexString([]byte("null")))
	require.Error(t, isHexString([]byte("\"")))
	require.Error(t, isHexString([]byte("QEVX=")))
}

func TestEncodableNetworkPubKey(t *testing.T) {
	netw, err := crypto.GeneratePrivateKey(crypto.ECDSAP256, generateRandomSeed(t))
	require.NoError(t, err)
	key := NetworkPubKey{netw.PublicKey()}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.NotEmpty(t, enc)

	var dec NetworkPubKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.True(t, key.Equals(dec.PublicKey))

	require.NoError(t, isHexString(enc))
}

func TestEncodableNetworkPubKeyNil(t *testing.T) {
	key := NetworkPubKey{}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.Equal(t, "null", string(enc))

	var dec NetworkPubKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.Equal(t, key, dec)

	require.NoError(t, isHexString(enc))
}

func TestEncodableNetworkPrivKey(t *testing.T) {
	netw, err := crypto.GeneratePrivateKey(crypto.ECDSAP256, generateRandomSeed(t))
	require.NoError(t, err)
	key := NetworkPrivKey{netw}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.NotEmpty(t, enc)

	var dec NetworkPrivKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.True(t, key.Equals(dec.PrivateKey))

	require.NoError(t, isHexString(enc))
}

func TestEncodableNetworkPrivKeyNil(t *testing.T) {
	key := NetworkPrivKey{}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.Equal(t, "null", string(enc))

	var dec NetworkPrivKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.Equal(t, key, dec)

	require.NoError(t, isHexString(enc))
}

func TestEncodableStakingPubKey(t *testing.T) {
	stak, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, generateRandomSeed(t))
	require.NoError(t, err)
	key := StakingPubKey{stak.PublicKey()}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.NotEmpty(t, enc)

	var dec StakingPubKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.True(t, key.Equals(dec.PublicKey))

	require.NoError(t, isHexString(enc))
}

func TestEncodableStakingPubKeyNil(t *testing.T) {
	key := StakingPubKey{}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.Equal(t, "null", string(enc))

	var dec StakingPubKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.Equal(t, key, dec)

	require.NoError(t, isHexString(enc))
}

func TestEncodableStakingPrivKey(t *testing.T) {
	stak, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, generateRandomSeed(t))
	require.NoError(t, err)
	key := StakingPrivKey{stak}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.NotEmpty(t, enc)

	var dec StakingPrivKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)

	require.True(t, key.Equals(dec.PrivateKey), "encoded/decoded key equality check failed")

	require.NoError(t, isHexString(enc))
}

func TestEncodableStakingPrivKeyNil(t *testing.T) {
	key := StakingPrivKey{}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.Equal(t, "null", string(enc))

	var dec StakingPrivKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.Equal(t, key, dec)

	require.NoError(t, isHexString(enc))
}

func TestEncodableRandomBeaconPubKey(t *testing.T) {
	randbeac, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, generateRandomSeed(t))
	require.NoError(t, err)
	key := RandomBeaconPubKey{randbeac.PublicKey()}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.NotEmpty(t, enc)

	var dec RandomBeaconPubKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.True(t, key.Equals(dec.PublicKey))

	require.NoError(t, isHexString(enc))
}

func TestEncodableRandomBeaconPubKeyNil(t *testing.T) {
	key := RandomBeaconPubKey{}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.Equal(t, "null", string(enc))

	var dec RandomBeaconPubKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.Equal(t, key, dec)

	require.NoError(t, isHexString(enc))
}

func TestEncodableRandomBeaconPrivKey(t *testing.T) {
	randbeac, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, generateRandomSeed(t))
	require.NoError(t, err)
	key := RandomBeaconPrivKey{randbeac}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.NotEmpty(t, enc)

	var dec RandomBeaconPrivKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)

	require.True(t, key.Equals(dec.PrivateKey), "encoded/decoded key equality check failed")

	require.NoError(t, isHexString(enc))
}

func TestEncodableRandomBeaconPrivKeyNil(t *testing.T) {
	key := RandomBeaconPrivKey{}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.Equal(t, "null", string(enc))

	var dec RandomBeaconPrivKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.Equal(t, key, dec)

	require.NoError(t, isHexString(enc))
}

func TestEncodableRandomBeaconPrivKeyMsgPack(t *testing.T) {
	randbeac, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, generateRandomSeed(t))
	require.NoError(t, err)
	key := RandomBeaconPubKey{randbeac.PublicKey()}
	oldPubKey := key.PublicKey

	b, err := key.MarshalMsgpack()
	require.NoError(t, err)

	fmt.Printf("bytes: %d\n", len(b))

	err = key.UnmarshalMsgpack(b)
	require.NoError(t, err)

	require.Equal(t, oldPubKey, key.PublicKey)
}

func generateRandomSeed(t *testing.T) []byte {
	seed := make([]byte, 48)
	n, err := rand.Read(seed)
	require.Nil(t, err)
	require.Equal(t, n, 48)
	return seed
}
