package crypto

import "C"
import (
	"crypto/rand"
	"fmt"
	"github.com/onflow/flow-go/crypto/bn254"
	bn256cf "github.com/onflow/flow-go/crypto/bn254/cloudflare"
	"github.com/onflow/flow-go/crypto/hash"
	"golang.org/x/crypto/sha3"
	"math/big"
)

var (
	Big0 = big.NewInt(0)
	Big1 = big.NewInt(1)
	Big2 = big.NewInt(2)
	Big3 = big.NewInt(3)
	Big4 = big.NewInt(4)
)

const (
	PubKeyLenBLSBN256 = 128

	SignatureLengthBLSBN256 = 64

	KeyGenSeedMaxLenBLSBN256 = 2048 // large enough constant accepted by the implementation

)

var G2, _ = BuildG2(
	BigFromBase10("11559732032986387107991004021392285783925812861821192530917403151452391805634"),
	BigFromBase10("10857046999023057135944570762232829481370756359578518086990519993285655852781"),
	BigFromBase10("4082367875863433681332203403145435568316851327593401208105741076214120093531"),
	BigFromBase10("8495653923123431417604973247489272438418190587263600148770280649306958101930"))

// P is a prime over which we form a basic field: 36u⁴+36u³+24u²+6u+1.
var P = BigFromBase10("21888242871839275222246405745257275088696311157297823662689037894645226208583")

func BigFromBase10(s string) *big.Int {
	n, _ := new(big.Int).SetString(s, 10)
	return n
}

// BuildG1 create G1 point from big.Int(s)
func BuildG1(x, y *big.Int) (*bn254.G1, error) {
	// Each value is a 256-bit number.
	const numBytes = 256 / 8
	xBytes := new(big.Int).Mod(x, P).Bytes()
	yBytes := new(big.Int).Mod(y, P).Bytes()
	m := make([]byte, numBytes*2)
	copy(m[1*numBytes-len(xBytes):], xBytes)
	copy(m[2*numBytes-len(yBytes):], yBytes)
	point := new(bn254.G1)
	if _, err := point.Unmarshal(m); err != nil {
		return nil, err
	}
	return point, nil
}

// BuildG2 create G2 point from big.Int(s)
func BuildG2(xx, xy, yx, yy *big.Int) (*bn254.G2, error) {
	// Each value is a 256-bit number.
	const numBytes = 256 / 8
	xxBytes := new(big.Int).Mod(xx, P).Bytes()
	xyBytes := new(big.Int).Mod(xy, P).Bytes()
	yxBytes := new(big.Int).Mod(yx, P).Bytes()
	yyBytes := new(big.Int).Mod(yy, P).Bytes()

	m := make([]byte, numBytes*4)
	copy(m[1*numBytes-len(xxBytes):], xxBytes)
	copy(m[2*numBytes-len(xyBytes):], xyBytes)
	copy(m[3*numBytes-len(yxBytes):], yxBytes)
	copy(m[4*numBytes-len(yyBytes):], yyBytes)
	point := new(bn254.G2)
	if _, err := point.Unmarshal(m); err != nil {
		return nil, err
	}
	return point, nil
}

// blsBLS12381Algo, embeds SignAlgo
type blsBN256Algo struct {
	// the signing algo and parameters
	algo SigningAlgorithm
}

// BLS context on the BLS 12-381 curve
var blsBN256Instance *blsBN256Algo

// prKeyBLSBLS12381 is the private key of BLS using BLS12_381, it implements PrivateKey
type prKeyBLSBN256 struct {
	pk    *pubKeyBLSBN256
	s     *big.Int
	point *bn254.G1
}

// newPrKeyBLSBLS12381 creates a new BLS private key with the given scalar.
// If no scalar is provided, the function allocates an
// empty scalar.
func newPrKeyBLSBN256(s *big.Int) *prKeyBLSBN256 {
	if s == nil {
		k, p, _ := bn256cf.RandomG1(rand.Reader)
		return &prKeyBLSBN256{s: k, point: p}
	}

	return &prKeyBLSBN256{s: s, point: new(bn254.G1).ScalarBaseMult(s)}
}

// Algorithm returns the Signing Algorithm
func (sk *prKeyBLSBN256) Algorithm() SigningAlgorithm {
	return BLSBN256
}

// Size returns the private key length in bytes
func (sk *prKeyBLSBN256) Size() int {
	return PubKeyLenBLSBN256
}

// computePublicKey generates the public key corresponding to
// the input private key. The function makes sure the public key
// is valid in G2.
func (sk *prKeyBLSBN256) computePublicKey() {
	pk := new(bn254.G2).ScalarBaseMult(sk.s)

	sk.pk = &pubKeyBLSBN256{point: pk}
}

// PublicKey returns the public key corresponding to the private key
func (sk *prKeyBLSBN256) PublicKey() PublicKey {
	if sk.pk != nil {
		return sk.pk
	}

	sk.computePublicKey()
	return sk.pk
}

// BigToBytes convert big int to byte array
// `minLen` is the minimum length of the array
func BigToBytes(bi *big.Int, minLen int) []byte {
	b := bi.Bytes()
	if minLen <= len(b) {
		return b
	}
	m := make([]byte, minLen)
	copy(m[minLen-len(b):], b)
	return m
}

// Encode returns a byte encoding of the private key.
// The encoding is a raw encoding in big endian padded to the group order
func (sk *prKeyBLSBN256) Encode() []byte {
	return BigToBytes(sk.s, 32)
}

// Equals checks is two public keys are equal.
func (sk *prKeyBLSBN256) Equals(other PrivateKey) bool {
	otherBLS, ok := other.(*prKeyBLSBN256)
	if !ok {
		return false
	}

	return otherBLS.String() == sk.String()
}

func (sk *prKeyBLSBN256) Sign(data []byte, hasher hash.Hasher) (Signature, error) {
	m := hasher.ComputeHash(data)
	hm := HashToG1(m)
	g1 := new(bn254.G1)
	g1.ScalarMult(hm, sk.s)
	return g1.Marshal(), nil
}

// String returns the hex string representation of the key.
func (sk *prKeyBLSBN256) String() string {
	return fmt.Sprintf("%#x", sk.Encode())
}

// pubKeyBLSBN256 is the public key of BLS using BN256,
// it implements PublicKey.
type pubKeyBLSBN256 struct {
	// public key G2 point
	point *bn254.G2
}

// Algorithm returns the Signing Algorithm
func (pk *pubKeyBLSBN256) Algorithm() SigningAlgorithm {
	return BLSBN256
}

// Size returns the public key lengh in bytes
func (pk *pubKeyBLSBN256) Size() int {
	return PubKeyLenBLSBN256
}

// Encode returns a byte encoding of the public key.
// Since we use a compressed encoding by default, this delegates to EncodeCompressed
func (pk *pubKeyBLSBN256) Encode() []byte {
	dest := make([]byte, PubKeyLenBLSBN256)
	_, _ = pk.point.Unmarshal(dest)
	return dest
}

// Equals checks is two public keys are equal
func (pk *pubKeyBLSBN256) Equals(other PublicKey) bool {
	otherBLS, ok := other.(*pubKeyBLSBN256)
	if !ok {
		return false
	}
	return pk.point.String() == otherBLS.point.String()
}

// String returns the hex string representation of the key.
func (pk *pubKeyBLSBN256) String() string {
	return fmt.Sprintf("%#x", pk.Encode())
}

func (pk *pubKeyBLSBN256) EncodeCompressed() []byte {
	return pk.Encode()
}

func (pk *pubKeyBLSBN256) Verify(s Signature, data []byte, hasher hash.Hasher) (bool, error) {
	// hash the input to 128 bytes
	h := hasher.ComputeHash(data)

	hm := new(bn254.G1).Neg(HashToG1(h))
	a := make([]*bn254.G1, 2)
	b := make([]*bn254.G2, 2)

	sig := new(bn254.G1)
	if _, err := sig.Unmarshal(s); err != nil {
		return false, err
	}

	a[0], b[0] = hm, pk.point
	a[1], b[1] = sig, G2
	return bn254.PairingCheck(a, b), nil
}

func Keccak256(m []byte) []byte {
	sha := sha3.NewLegacyKeccak256()
	sha.Write(m)
	return sha.Sum(nil)
}

func g1XToYSquared(x *big.Int) *big.Int {
	result := new(big.Int)
	result.Exp(x, Big3, P)
	result.Add(result, Big3)
	return result
}

// Currently implementing first method from
// http://mathworld.wolfram.com/QuadraticResidue.html
// Experimentally, this seems to always return the canonical square root,
// however I haven't seen a proof of this.
func calcQuadRes(ySqr *big.Int, q *big.Int) *big.Int {
	resMod4 := new(big.Int).Mod(q, Big4)
	if resMod4.Cmp(Big3) == 0 {
		k := new(big.Int).Sub(q, Big3)
		k.Div(k, Big4)
		exp := new(big.Int).Add(k, Big1)
		result := new(big.Int)
		result.Exp(ySqr, exp, q)
		return result
	}
	// TODO: ADD CODE TO CALC QUADRATIC RESIDUE IN OTHER CASES
	return Big0
}

// HashToG1 try and increment hashing data to a G1 point
func HashToG1(m []byte) *bn254.G1 {
	px := new(big.Int)
	py := new(big.Int)

	h := m
	//h := Keccak256(m)
	bf := append([]byte{0}, h...)
	for {
		h = Keccak256(bf)
		px.SetBytes(h[:32])
		px.Mod(px, P)
		ySqr := g1XToYSquared(px)
		root := calcQuadRes(ySqr, P)
		rootSqr := new(big.Int).Exp(root, Big2, P)
		if rootSqr.Cmp(ySqr) == 0 {
			py = root
			bf[0] = byte(255)
			signY := Keccak256(bf)[31] % 2
			if signY == 1 {
				py.Sub(P, py)
			}
			break
		}
		bf[0]++
	}
	p, err := BuildG1(px, py)
	if err != nil {
		panic(err)
	}
	return p
}

func (a *blsBN256Algo) decodePublicKey(publicKeyBytes []byte) (PublicKey, error) {
	p := new(bn254.G2)
	if _, err := p.Unmarshal(publicKeyBytes); err != nil {
		return nil, err
	}
	return &pubKeyBLSBN256{point: p}, nil
}

// generatePrivateKey generates a private key for BLS on BLS12-381 curve.
// The minimum size of the input seed is 48 bytes.
//
// It is recommended to use a secure crypto RNG to generate the seed.
// The seed must have enough entropy and should be sampled uniformly at random.
//
// The generated private key (resp. its corresponding public key) are guaranteed
// not to be equal to the identity element of Z_r (resp. G2).
func (a *blsBN256Algo) generatePrivateKey(seed []byte) (PrivateKey, error) {
	if len(seed) == 0 {
		return newPrKeyBLSBN256(nil), nil
	}

	if len(seed) > KeyGenSeedMaxLenBLSBN256 {
		return nil, invalidInputsErrorf("seed byte length should be between %d and %d",
			0, KeyGenSeedMaxLenECDSA)
	}

	k := new(big.Int).SetBytes(seed)

	sk := newPrKeyBLSBN256(k)

	return sk, nil
}

func (a *blsBN256Algo) decodePrivateKey(privateKeyBytes []byte) (PrivateKey, error) {
	return a.generatePrivateKey(privateKeyBytes)
}

func (a *blsBN256Algo) decodePublicKeyCompressed(publicKeyBytes []byte) (PublicKey, error) {
	return a.decodePublicKey(publicKeyBytes)
}
