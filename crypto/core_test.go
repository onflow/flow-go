// +build relic

package crypto

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPRGSeeding(t *testing.T) {
	// 2 keys generated with the same seed should be equal
	seed := []byte{1, 2, 3, 4}
	sk1, err := GeneratePrivateKey(BLS_BLS12381, seed)
	assert.Nil(t, err)
	pk1Bytes, _ := sk1.PublicKey().Encode()
	sk2, err := GeneratePrivateKey(BLS_BLS12381, seed)
	assert.Nil(t, err)
	pk2Bytes, _ := sk2.PublicKey().Encode()
	assert.Equal(t, pk1Bytes, pk2Bytes)
}

// TestG1 helps debugging but is not a unit test
func TestG1(t *testing.T) {
	NewSigner(BLS_BLS12381)

	seedRelic([]byte{0})
	var expo scalar
	randZr(&expo)
	var res pointG1
	_G1scalarGenMult(&res, &expo)

}

// G1 bench
func BenchmarkG1(b *testing.B) {
	NewSigner(BLS_BLS12381)
	seedRelic([]byte{0})
	var expo scalar
	randZr(&expo)
	var res pointG1

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_G1scalarGenMult(&res, &expo)
	}
	b.StopTimer()
	return
}

// TestG2 helps debugging but is not a unit test
func TestG2(t *testing.T) {

	NewSigner(BLS_BLS12381)

	var expo scalar
	(&expo).setInt(1)
	var res pointG2
	_G2scalarGenMult(&res, &expo)

}

// G2 bench
func BenchmarkG2(b *testing.B) {
	NewSigner(BLS_BLS12381)
	seedRelic([]byte{0})
	var expo scalar
	randZr(&expo)
	var res pointG2

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_G2scalarGenMult(&res, &expo)
	}
	b.StopTimer()
	return
}

// Hashing to G1 bench
func BenchmarkHashToG1(b *testing.B) {
	NewSigner(BLS_BLS12381)
	input := []byte("Bench input")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hashToG1(input)
	}
	b.StopTimer()
	return
}

// Hashing to G1 bench
func TestHashToG1(t *testing.T) {
	NewSigner(BLS_BLS12381)
	// msg is split into 2 halves to accomodate testing the optimized SwU algo
	msg0 := "0e58bd6d947af8aec009ff396cd83a3636614f917423db76e8948e9c25130ae04e721beb924efca3ce585540b2567cf6"
	msg1 := "0082bd2ed5473b191da55420c9b4df9031a50445b28c17115d614ad6993d7037d6792dd2211e4b485761a6fe2df17582"
	input, _ := hex.DecodeString(msg0 + msg1)
	hashToG1(input)
	return
}
