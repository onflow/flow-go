package cmd

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
)

func TestEncodableNetworkPubKey(t *testing.T) {
	netw, err := crypto.GeneratePrivateKey(crypto.ECDSA_SECp256k1, generateRandomSeed())
	require.NoError(t, err)
	key := EncodableNetworkPubKey{netw.PublicKey()}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.NotEmpty(t, enc)

	var dec EncodableNetworkPubKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.Equal(t, key, dec)
}

func TestEncodableNetworkPubKeyNil(t *testing.T) {
	key := EncodableNetworkPubKey{}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.Equal(t, "null", string(enc))

	var dec EncodableNetworkPubKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.Equal(t, key, dec)
}

func TestEncodableNetworkPrivKey(t *testing.T) {
	netw, err := crypto.GeneratePrivateKey(crypto.ECDSA_SECp256k1, generateRandomSeed())
	require.NoError(t, err)
	key := EncodableNetworkPrivKey{netw}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.NotEmpty(t, enc)

	var dec EncodableNetworkPrivKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.Equal(t, key, dec)
}

func TestEncodableNetworkPrivKeyNil(t *testing.T) {
	key := EncodableNetworkPrivKey{}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.Equal(t, "null", string(enc))

	var dec EncodableNetworkPrivKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.Equal(t, key, dec)
}

func TestEncodableStakingPubKey(t *testing.T) {
	stak, err := crypto.GeneratePrivateKey(crypto.BLS_BLS12381, generateRandomSeed())
	require.NoError(t, err)
	key := EncodableStakingPubKey{stak.PublicKey()}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.NotEmpty(t, enc)

	var dec EncodableStakingPubKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.Equal(t, key, dec)
}

func TestEncodableStakingPubKeyNil(t *testing.T) {
	key := EncodableStakingPubKey{}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.Equal(t, "null", string(enc))

	var dec EncodableStakingPubKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.Equal(t, key, dec)
}

func TestEncodableStakingPrivKey(t *testing.T) {
	stak, err := crypto.GeneratePrivateKey(crypto.BLS_BLS12381, generateRandomSeed())
	require.NoError(t, err)
	key := EncodableStakingPrivKey{stak}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.NotEmpty(t, enc)

	var dec EncodableStakingPrivKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)

	require.True(t, key.Equals(dec), "encoded/decoded key equality check failed")
}

func TestEncodableStakingPrivKeyNil(t *testing.T) {
	key := EncodableStakingPrivKey{}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.Equal(t, "null", string(enc))

	var dec EncodableStakingPrivKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.Equal(t, key, dec)
}

func TestEncodableRandomBeaconPubKey(t *testing.T) {
	randbeac, err := crypto.GeneratePrivateKey(crypto.BLS_BLS12381, generateRandomSeed())
	require.NoError(t, err)
	key := EncodableRandomBeaconPubKey{randbeac.PublicKey()}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.NotEmpty(t, enc)

	var dec EncodableRandomBeaconPubKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.Equal(t, key, dec)
}

func TestEncodableRandomBeaconPubKeyNil(t *testing.T) {
	key := EncodableRandomBeaconPubKey{}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.Equal(t, "null", string(enc))

	var dec EncodableRandomBeaconPubKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.Equal(t, key, dec)
}

func TestEncodableRandomBeaconPrivKey(t *testing.T) {
	randbeac, err := crypto.GeneratePrivateKey(crypto.BLS_BLS12381, generateRandomSeed())
	require.NoError(t, err)
	key := EncodableRandomBeaconPrivKey{randbeac}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.NotEmpty(t, enc)

	var dec EncodableRandomBeaconPrivKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)

	require.True(t, key.Equals(dec), "encoded/decoded key equality check failed")
}

func TestEncodableRandomBeaconPrivKeyNil(t *testing.T) {
	key := EncodableRandomBeaconPrivKey{}

	enc, err := json.Marshal(key)
	require.NoError(t, err)
	require.Equal(t, "null", string(enc))

	var dec EncodableRandomBeaconPrivKey
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.Equal(t, key, dec)
}
