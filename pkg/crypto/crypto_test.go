package crypto

import (
	"bytes"
	"encoding/hex"
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
	input := []byte("test")
	expected, _ := hex.DecodeString("36f028580bb02cc8272a9a020f4200e346e276ae664e45ee80745574e2f5ab80")

	alg := NewHashAlgo(SHA3_256)
	hash := alg.ComputeBytesHash(input).Bytes()
	checkBytes(t, input, expected, hash)

	hash = alg.ComputeStructHash(&testStruct{"te", "st"}).Bytes()
	checkBytes(t, input, expected, hash)

	alg.Reset()
	alg.AddBytes([]byte("te"))
	alg.AddBytes([]byte("s"))
	alg.AddBytes([]byte("t"))
	hash = alg.SumHash().Bytes()
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

func benchVerify(b *testing.B, salg Signer, halg Hasher) {
	seed := []byte("keyseed")
	sk := salg.GeneratePrKey(seed)
	pk := sk.Pubkey()

	input := []byte("Bench input")

	s := salg.SignBytes(sk, input, halg)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_ = salg.VerifyBytes(pk, s, input, halg)
	}

	b.StopTimer()
}

func benchSign(b *testing.B, salg Signer, halg Hasher) {
	seed := []byte("keyseed")
	sk := salg.GeneratePrKey(seed)

	input := []byte("Bench input")

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_ = salg.SignBytes(sk, input, halg)
	}
	b.StopTimer()
}

func testSignBytes(t *testing.T, salg Signer, halg Hasher, sk PrKey, input []byte) {
	s := salg.SignBytes(sk, input, halg)
	pk := sk.Pubkey()
	result := salg.VerifyBytes(pk, s, input, halg)
	if result == false {
		t.Errorf("BLS Verification failed:\n signature is %s", s)
	} else {
		t.Logf("BLS Verification passed:\n signature is %s", s)
	}
}

func testSignStruct(t *testing.T, salg Signer, halg Hasher, sk PrKey, input Encoder) {
	s := salg.SignStruct(sk, input, halg)
	pk := sk.Pubkey()
	result := salg.VerifyStruct(pk, s, input, halg)

	if result == false {
		t.Errorf("BLS Verification failed:\n signature is %x", s)
	} else {
		t.Logf("BLS Verification passed:\n signature is %x", s)
	}
}

// BLS tests
func TestBLS_BLS12381(t *testing.T) {
	salg := NewSignatureAlgo(BLS_BLS12381)
	seed := []byte{1, 2, 3, 4}
	sk := salg.GeneratePrKey(seed)
	input := []byte("test")
	testSignBytes(t, salg, nil, sk, input)

	inputStruct := &testStruct{"te", "st"}
	testSignStruct(t, salg, nil, sk, inputStruct)
}

// TestG1 helps debugging but is not a unit test
func TestG1(t *testing.T) {
	_ = NewSignatureAlgo(BLS_BLS12381)

	var expo scalar
	randZr(&expo, []byte{0})
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

// TestG2 helps debugging but is not a unit test
func TestG2(t *testing.T) {

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

// Signing bench
func BenchmarkBLS_BLS12381Sign(b *testing.B) {

	salg := NewSignatureAlgo(BLS_BLS12381)
	benchSign(b, salg, nil)
}

// Verifying bench
func BenchmarkBLS_BLS12381Verify(b *testing.B) {
	salg := NewSignatureAlgo(BLS_BLS12381)
	benchVerify(b, salg, nil)
}

// Hashing to G1 bench
func BenchmarkHashToG1(b *testing.B) {
	input := []byte("Bench input")
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		hashToG1(input)
	}
	b.StopTimer()
	return
}

// ECDSA tests
func TestECDSA(t *testing.T) {
	salg := NewSignatureAlgo(ECDSA_P256)
	halg := NewHashAlgo(SHA3_256)
	seed := []byte{1, 2, 3, 4}
	sk := salg.GeneratePrKey(seed)

	input := []byte("test")
	testSignBytes(t, salg, halg, sk, input)

	inputStruct := &testStruct{"te", "st"}
	testSignStruct(t, salg, halg, sk, inputStruct)
}

// Signing bench
func BenchmarkECDSASign(b *testing.B) {
	salg := NewSignatureAlgo(ECDSA_P256)
	halg := NewHashAlgo(SHA3_256)
	benchSign(b, salg, halg)
}

// Verifying bench
func BenchmarkECDSAVerify(b *testing.B) {
	salg := NewSignatureAlgo(ECDSA_P256)
	halg := NewHashAlgo(SHA3_256)
	benchVerify(b, salg, halg)
}
