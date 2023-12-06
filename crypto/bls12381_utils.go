package crypto

// this file contains utility functions for the curve BLS 12-381
// these tools are shared by the BLS signature scheme, the BLS based threshold signature
// and the BLS distributed key generation protocols

// #cgo CFLAGS: -I${SRCDIR}/ -I${SRCDIR}/blst_src -I${SRCDIR}/blst_src/build -D__BLST_CGO__ -Wall -fno-builtin-memcpy -fno-builtin-memset -Wno-unused-function -Wno-unused-macros -Wno-unused-variable
// #cgo amd64 CFLAGS: -D__ADX__ -mno-avx
// #cgo mips64 mips64le ppc64 ppc64le riscv64 s390x CFLAGS: -D__BLST_NO_ASM__
// #include "bls12381_utils.h"
//
// #if defined(__x86_64__) && (defined(__unix__) || defined(__APPLE__))
// # include <signal.h>
// # include <unistd.h>
// # include <string.h>
// static void handler(int signum)
// {	char text[1024] = "Caught SIGILL in blst_cgo_init, BLST library (used by flow-go/crypto) requires ADX support, build with CGO_CFLAGS=\"-O -D__BLST_PORTABLE__\"\n";
//		ssize_t n = write(2, &text, strlen(text));
//      _exit(128+SIGILL);
//      (void)n;
// }
// __attribute__((constructor)) static void flow_crypto_cgo_init()
// {   Fp temp = { 0 };
//     struct sigaction act = {{ handler }}, oact;
//     sigaction(SIGILL, &act, &oact);
//     Fp_squ_montg(&temp, &temp);
//     sigaction(SIGILL, &oact, NULL);
// }
// #endif
//
import "C"
import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/crypto/random"
)

// Go wrappers around BLST C types
type pointE1 C.E1
type pointE2 C.E2
type scalar C.Fr

// Note that scalars and field elements F_r are represented in Go by the same type
// called `scalar`, which is internally represented by C type `Fr`. Scalars used by the
// Go layer are all reduced modulo the curve order `r`.

const (
	// BLS12-381 related lengths imported from the C layer
	frBytesLen = int(C.Fr_BYTES)
	fpBytesLen = int(C.Fp_BYTES)
	g1BytesLen = int(C.G1_SER_BYTES)
	g2BytesLen = int(C.G2_SER_BYTES)

	// error constants imported from the C layer
	valid           = C.VALID
	invalid         = C.INVALID
	badEncoding     = C.BAD_ENCODING
	badValue        = C.BAD_VALUE
	pointNotOnCurve = C.POINT_NOT_ON_CURVE
)

// header of the point at infinity serializations
var g1SerHeader byte // g1 (G1 identity)
var g2SerHeader byte // g2 (G2 identity)

// `g1` serialization
var g1Serialization []byte

var g2PublicKey pubKeyBLSBLS12381

// initialization of BLS12-381 curve
func initBLS12381() {
	C.types_sanity()

	if isG1Compressed() {
		g1SerHeader = 0xC0
	} else {
		g1SerHeader = 0x40
	}
	g1Serialization = append([]byte{g1SerHeader}, make([]byte, g1BytesLen-1)...)
	if isG2Compressed() {
		g2SerHeader = 0xC0
	} else {
		g2SerHeader = 0x40
	}
	// set a global point to infinity
	C.E2_set_infty((*C.E2)(&g2PublicKey.point))
	g2PublicKey.isIdentity = true
}

// String returns a hex-encoded representation of the scalar.
func (a *scalar) String() string {
	encoding := make([]byte, frBytesLen)
	writeScalar(encoding, a)
	return fmt.Sprintf("%#x", encoding)
}

// String returns a hex-encoded representation of the E2 point.
func (p *pointE2) String() string {
	encoding := make([]byte, g2BytesLen)
	writePointE2(encoding, p)
	return fmt.Sprintf("%#x", encoding)
}

// Scalar multiplication of a generic point `p` in E1
func (p *pointE1) scalarMultE1(res *pointE1, expo *scalar) {
	C.E1_mult((*C.E1)(res), (*C.E1)(p), (*C.Fr)(expo))
}

// Scalar multiplication of generator g1 in G1
func generatorScalarMultG1(res *pointE1, expo *scalar) {
	C.G1_mult_gen((*C.E1)(res), (*C.Fr)(expo))
}

// Scalar multiplication of generator g2 in G2
//
// This often results in a public key that is used in
// multiple pairing computation. Therefore, convert the
// resulting point to affine coordinate to save pre-pairing
// conversions.
func generatorScalarMultG2(res *pointE2, expo *scalar) {
	C.G2_mult_gen_to_affine((*C.E2)(res), (*C.Fr)(expo))
}

// comparison in Fr where r is the group order of G1/G2
// (both scalars should be reduced mod r)
func (x *scalar) equals(other *scalar) bool {
	return bool(C.Fr_is_equal((*C.Fr)(x), (*C.Fr)(other)))
}

// comparison in E1
func (p *pointE1) equals(other *pointE1) bool {
	return bool(C.E1_is_equal((*C.E1)(p), (*C.E1)(other)))
}

// comparison in E2
func (p *pointE2) equals(other *pointE2) bool {
	return bool(C.E2_is_equal((*C.E2)(p), (*C.E2)(other)))
}

// Comparison to zero in Fr.
// Scalar must be already reduced modulo r
func (x *scalar) isZero() bool {
	return bool(C.Fr_is_zero((*C.Fr)(x)))
}

// Comparison to point at infinity in G2.
func (p *pointE2) isInfinity() bool {
	return bool(C.E2_is_infty((*C.E2)(p)))
}

// generates a random element in F_r using input random source,
// and saves the random in `x`.
// returns `true` if generated element is zero.
func randFr(x *scalar, rand random.Rand) bool {
	// use extra 128 bits to reduce the modular reduction bias
	bytes := make([]byte, frBytesLen+securityBits/8)
	rand.Read(bytes)
	// modular reduction
	return mapToFr(x, bytes)
}

// generates a random element in F_r* using input random source,
// and saves the random in `x`.
func randFrStar(x *scalar, rand random.Rand) {
	isZero := true
	// extremely unlikely this loop runs more than once,
	// but force the output to be non-zero instead of propagating an error.
	for isZero {
		isZero = randFr(x, rand)
	}
}

// mapToFr reads a scalar from a slice of bytes and maps it to Fr using modular reduction.
// The resulting element `k` therefore satisfies 0 <= k < r.
// It returns true if scalar is zero and false otherwise.
func mapToFr(x *scalar, src []byte) bool {
	isZero := C.map_bytes_to_Fr((*C.Fr)(x),
		(*C.uchar)(&src[0]),
		(C.int)(len(src)))
	return bool(isZero)
}

// writeScalar writes a scalar in a slice of bytes
func writeScalar(dest []byte, x *scalar) {
	C.Fr_write_bytes((*C.uchar)(&dest[0]), (*C.Fr)(x))
}

// writePointE2 writes a G2 point in a slice of bytes
// The slice should be of size g2BytesLen and the serialization
// follows the Zcash format specified in draft-irtf-cfrg-pairing-friendly-curves
func writePointE2(dest []byte, a *pointE2) {
	C.E2_write_bytes((*C.uchar)(&dest[0]), (*C.E2)(a))
}

// writePointE1 writes a G1 point in a slice of bytes
// The slice should be of size g1BytesLen and the serialization
// follows the Zcash format specified in draft-irtf-cfrg-pairing-friendly-curves
func writePointE1(dest []byte, a *pointE1) {
	C.E1_write_bytes((*C.uchar)(&dest[0]), (*C.E1)(a))
}

// read an Fr* element from a byte slice
// and stores it into a `scalar` type element.
func readScalarFrStar(a *scalar, src []byte) error {
	read := C.Fr_star_read_bytes(
		(*C.Fr)(a),
		(*C.uchar)(&src[0]),
		(C.int)(len(src)))

	switch read {
	case valid:
		return nil
	case badEncoding:
		return invalidInputsErrorf("input length must be %d, got %d",
			frBytesLen, len(src))
	case badValue:
		return invalidInputsErrorf("scalar is not in the correct range")
	default:
		return invalidInputsErrorf("reading the scalar failed")
	}
}

// readPointE2 reads a E2 point from a slice of bytes
// The slice is expected to be of size g2BytesLen and the deserialization
// follows the Zcash format specified in draft-irtf-cfrg-pairing-friendly-curves.
// No G2 membership check is performed.
func readPointE2(a *pointE2, src []byte) error {
	read := C.E2_read_bytes((*C.E2)(a),
		(*C.uchar)(&src[0]),
		(C.int)(len(src)))

	switch read {
	case valid:
		return nil
	case badEncoding, badValue:
		return invalidInputsErrorf("input could not deserialize to an E2 point")
	case pointNotOnCurve:
		return invalidInputsErrorf("input is not a point on curve E2")
	default:
		return errors.New("reading E2 point failed")
	}
}

// readPointE1 reads a E1 point from a slice of bytes
// The slice should be of size g1BytesLen and the deserialization
// follows the Zcash format specified in draft-irtf-cfrg-pairing-friendly-curves.
// No G1 membership check is performed.
func readPointE1(a *pointE1, src []byte) error {
	read := C.E1_read_bytes((*C.E1)(a),
		(*C.uchar)(&src[0]),
		(C.int)(len(src)))

	switch read {
	case valid:
		return nil
	case badEncoding, badValue:
		return invalidInputsErrorf("input could not deserialize to a E1 point")
	case pointNotOnCurve:
		return invalidInputsErrorf("input is not a point on curve E1")
	default:
		return errors.New("reading E1 point failed")
	}
}

// checkMembershipG1 wraps a call to a subgroup check in G1 since cgo can't be used
// in go test files.
func checkMembershipG1(pt *pointE1) bool {
	return bool(C.E1_in_G1((*C.E1)(pt)))
}

// checkMembershipG2 wraps a call to a subgroup check in G2 since cgo can't be used
// in go test files.
func checkMembershipG2(pt *pointE2) bool {
	return bool(C.E2_in_G2((*C.E2)(pt)))
}

// This is only a TEST/DEBUG/BENCH function.
// It returns the hash-to-G1 point from a slice of 128 bytes
func mapToG1(data []byte) *pointE1 {
	l := len(data)
	var h pointE1
	if C.map_to_G1((*C.E1)(&h), (*C.uchar)(&data[0]), (C.int)(l)) != valid {
		return nil
	}
	return &h
}

// mapToG1 is a test function, it wraps a call to C since cgo can't be used in go test files.
// It maps input bytes to a point in G2 and stores it in input point.
// THIS IS NOT the kind of mapping function that is used in BLS signature.
func unsafeMapToG1(pt *pointE1, seed []byte) {
	C.unsafe_map_bytes_to_G1((*C.E1)(pt), (*C.uchar)(&seed[0]), (C.int)(len(seed)))
}

// unsafeMapToG1Complement is a test function, it wraps a call to C since cgo can't be used in go test files.
// It generates a random point in E2\G2 and stores it in input point.
func unsafeMapToG1Complement(pt *pointE1, seed []byte) {
	C.unsafe_map_bytes_to_G1complement((*C.E1)(pt), (*C.uchar)(&seed[0]), (C.int)(len(seed)))
}

// unsafeMapToG2 is a test function, it wraps a call to C since cgo can't be used in go test files.
// It maps input bytes to a point in G2 and stores it in input point.
// THIS IS NOT the kind of mapping function that is used in BLS signature.
func unsafeMapToG2(pt *pointE2, seed []byte) {
	C.unsafe_map_bytes_to_G2((*C.E2)(pt), (*C.uchar)(&seed[0]), (C.int)(len(seed)))
}

// unsafeMapToG2Complement is a test function, it wraps a call to C since cgo can't be used in go test files.
// It generates a random point in E2\G2 and stores it in input point.
func unsafeMapToG2Complement(pt *pointE2, seed []byte) {
	C.unsafe_map_bytes_to_G2complement((*C.E2)(pt), (*C.uchar)(&seed[0]), (C.int)(len(seed)))
}

// This is only a TEST function.
// It hashes `data` to a G1 point using the tag `dst` and returns the G1 point serialization.
// The function uses xmd with SHA256 in the hash-to-field.
func hashToG1Bytes(data, dst []byte) []byte {
	hash := make([]byte, expandMsgOutput)

	inputLength := len(data)
	if len(data) == 0 {
		data = make([]byte, 1)
	}

	// XMD using SHA256
	C.xmd_sha256((*C.uchar)(&hash[0]),
		(C.int)(expandMsgOutput),
		(*C.uchar)(&data[0]), (C.int)(inputLength),
		(*C.uchar)(&dst[0]), (C.int)(len(dst)))

	// map the hash to G1
	var point pointE1
	if C.map_to_G1((*C.E1)(&point), (*C.uchar)(&hash[0]), (C.int)(len(hash))) != valid {
		return nil
	}

	// serialize the point
	pointBytes := make([]byte, g1BytesLen)
	writePointE1(pointBytes, &point)
	return pointBytes
}

func isG1Compressed() bool {
	return g1BytesLen == fpBytesLen
}

func isG2Compressed() bool {
	return g2BytesLen == 2*fpBytesLen
}
