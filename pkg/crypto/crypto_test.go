package crypto

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"
)

// obsolete test struct implementing Encoder
type testStruct struct {
	x string
	y string
}

func (struc *testStruct) Encode() []byte {
	return []byte(struc.x + struc.y)
}

// Sanity checks of SHA3_256
func TestSha3_256(t *testing.T) {
	fmt.Println("testing Sha3_256:")
	input := []byte("test")
	expected, _ := hex.DecodeString("36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80")

	alg := NewHashAlgo(SHA3_256)
	hash := alg.ComputeBytesHash(input).ToBytes()
	checkBytes(t, input, expected, hash)

	hash = alg.ComputeStructHash(&testStruct{"te", "st"}).ToBytes()
	checkBytes(t, input, expected, hash)

	alg.Reset()
	alg.AddBytes([]byte("te"))
	alg.AddBytes([]byte("s"))
	alg.AddBytes([]byte("t"))
	hash = alg.SumHash().ToBytes()
	checkBytes(t, input, expected, hash)
}

func checkBytes(t *testing.T, input, expected, result []byte) {
	if !bytes.Equal(expected, result) {
		t.Errorf("hash mismatch: expect: %s have: %s, input is %s", expected, result, input)
	} else {
		t.Logf("hash test ok: expect: %s, input: %s", expected, input)
	}
}

// SHA3_256 bench
func BenchmarkSha3_256(b *testing.B) {
	a := []byte("Bench me!")
	alg := NewHashAlgo(SHA3_256)
	for i := 0; i < b.N; i++ {
		alg.ComputeBytesHash(a)
	}
	return
}

// BLS tests
func TestBLS_BLS12381(t *testing.T) {
	fmt.Println("testing BLS on bls12_381:")
	input := []byte("test")

	salg := NewSignatureAlgo(BLS_BLS12381)
	seed := []byte{1, 2, 3}
	sk := salg.GeneratePrKey(seed)
	pk := sk.Pubkey()

	s := salg.SignBytes(sk, input, nil)
	result := salg.VerifyBytes(pk, s, input, nil)

	if result == false {
		t.Errorf("Verification failed: signature is %s", s)
	} else {
		t.Logf("Verification passed: signature is %s", s)
	}

	/*message := &testStruct{"te", "st"}
	s = salg.SignStruct(sk, message, nil)
	result = salg.VerifyStruct(pk, s, message, nil)

	if result == false {
		t.Errorf("Verification failed: signature is %x", s)
	} else {
		t.Logf("Verification passed: signature is %x", s)
	}*/
}

// Testg1 contains tests of operations in G1
func TestG1(t *testing.T) {
	fmt.Println("testing G1")

	_ = NewSignatureAlgo(BLS_BLS12381)

	var expo scalar
	(&expo).setInt(1)
	var res pointG1
	_G1scalarGenMult(&res, &expo)

}

// G1 bench
func BenchmarkG1(b *testing.B) {
	_ = NewSignatureAlgo(BLS_BLS12381)
	var expo scalar
	randZr(&expo, []byte{0})
	var res pointG1

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_G1scalarGenMult(&res, &expo)
	}
	b.StopTimer()
	return
}

// Testg2 contains tests of operations in G2
func TestG2(t *testing.T) {
	fmt.Println("testing G2")

	_ = NewSignatureAlgo(BLS_BLS12381)

	var expo scalar
	(&expo).setInt(1)
	var res pointG2
	_G2scalarGenMult(&res, &expo)

}

// G2 bench
func BenchmarkG2(b *testing.B) {
	_ = NewSignatureAlgo(BLS_BLS12381)
	var expo scalar
	randZr(&expo, []byte{0})
	var res pointG2

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_G2scalarGenMult(&res, &expo)
	}
	b.StopTimer()
	return
}
