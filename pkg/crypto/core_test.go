package crypto

import (
	"testing"
)

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
