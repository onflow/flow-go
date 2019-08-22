package crypto

import (
	"bytes"
	"encoding/hex"
	"testing"

	log "github.com/sirupsen/logrus"
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

	alg, err := NewHashAlgo(SHA3_256)
	if err != nil {
		log.Error(err.Error())
		return
	}
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
	alg, _ := NewHashAlgo(SHA3_256)
	for i := 0; i < b.N; i++ {
		alg.ComputeBytesHash(a)
	}
	return
}

func benchVerify(b *testing.B, salg Signer, halg Hasher) {
	seed := []byte("keyseed")
	sk, _ := salg.GeneratePrKey(seed)
	pk := sk.Pubkey()

	input := []byte("Bench input")
	s, _ := salg.SignBytes(sk, input, halg)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, _ = salg.VerifyBytes(pk, s, input, halg)
	}

	b.StopTimer()
}

func benchSign(b *testing.B, salg Signer, halg Hasher) {
	seed := []byte("keyseed")
	sk, _ := salg.GeneratePrKey(seed)

	input := []byte("Bench input")

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, _ = salg.SignBytes(sk, input, halg)
	}
	b.StopTimer()
}

func testSignBytes(t *testing.T, salg Signer, halg Hasher, sk PrKey, input []byte) {
	s, err := salg.SignBytes(sk, input, halg)
	if err != nil {
		log.Error(err.Error())
		return
	}
	pk := sk.Pubkey()
	result, err := salg.VerifyBytes(pk, s, input, halg)
	if err != nil {
		log.Error(err.Error())
		return
	}
	if result == false {
		t.Errorf("Verification failed:\n signature is %s", s)
	} else {
		t.Logf("Verification passed:\n signature is %s", s)
	}
}

func testSignStruct(t *testing.T, salg Signer, halg Hasher, sk PrKey, input Encoder) {
	s, err := salg.SignStruct(sk, input, halg)
	if err != nil {
		log.Error(err.Error())
		return
	}
	pk := sk.Pubkey()
	result, err := salg.VerifyStruct(pk, s, input, halg)
	if err != nil {
		log.Error(err.Error())
		return
	}

	if result == false {
		t.Errorf("Verification failed:\n signature is %x", s)
	} else {
		t.Logf("Verification passed:\n signature is %x", s)
	}
}

// BLS tests
func TestBLS_BLS12381(t *testing.T) {
	salg, err := NewSignatureAlgo(BLS_BLS12381)
	if err != nil {
		log.Error(err.Error())
		return
	}
	seed := []byte{1, 2, 3, 4}
	sk, err := salg.GeneratePrKey(seed)
	if err != nil {
		log.Error(err.Error())
		return
	}
	input := []byte("test")
	testSignBytes(t, salg, nil, sk, input)

	inputStruct := &testStruct{"te", "st"}
	testSignStruct(t, salg, nil, sk, inputStruct)
}

// TestG1 helps debugging but is not a unit test
func TestG1(t *testing.T) {
	_, _ = NewSignatureAlgo(BLS_BLS12381)

	var expo scalar
	randZr(&expo, []byte{0})
	var res pointG1
	_G1scalarGenMult(&res, &expo)

}

// G1 bench
func BenchmarkG1(b *testing.B) {
	_, _ = NewSignatureAlgo(BLS_BLS12381)
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

	_, _ = NewSignatureAlgo(BLS_BLS12381)

	var expo scalar
	(&expo).setInt(1)
	var res pointG2
	_G2scalarGenMult(&res, &expo)

}

// G2 bench
func BenchmarkG2(b *testing.B) {
	_, _ = NewSignatureAlgo(BLS_BLS12381)
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

	salg, err := NewSignatureAlgo(BLS_BLS12381)
	if err != nil {
		log.Error(err.Error())
		return
	}
	benchSign(b, salg, nil)
}

// Verifying bench
func BenchmarkBLS_BLS12381Verify(b *testing.B) {
	salg, err := NewSignatureAlgo(BLS_BLS12381)
	if err != nil {
		log.Error(err.Error())
		return
	}
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
	ECDSAcurves := []AlgoName{ECDSA_P256, ECDSA_SECp256k1}
	for _, curve := range ECDSAcurves {
		t.Logf("Testing ECDSA for curve %s", curve)
		salg, err := NewSignatureAlgo(curve)
		if err != nil {
			log.Error(err.Error())
			return
		}
		halg, err := NewHashAlgo(SHA3_256)
		if err != nil {
			log.Error(err.Error())
			return
		}
		seed := []byte{1, 2, 3, 4}
		sk, err := salg.GeneratePrKey(seed)
		if err != nil {
			log.Error(err.Error())
			return
		}
		input := []byte("test")
		testSignBytes(t, salg, halg, sk, input)

		inputStruct := &testStruct{"te", "st"}
		testSignStruct(t, salg, halg, sk, inputStruct)
	}
}

// Signing bench
func BenchmarkECDSASign(b *testing.B) {
	salg, _ := NewSignatureAlgo(ECDSA_SECp256k1)
	//salg, _ := NewSignatureAlgo(ECDSA_P256)
	halg, _ := NewHashAlgo(SHA3_256)
	benchSign(b, salg, halg)
}

// Verifying bench
func BenchmarkECDSAVerify(b *testing.B) {
	salg, _ := NewSignatureAlgo(ECDSA_SECp256k1)
	//salg, _ := NewSignatureAlgo(ECDSA_P256)
	halg, _ := NewHashAlgo(SHA3_256)
	benchVerify(b, salg, halg)
}
