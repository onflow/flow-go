//go:build relic
// +build relic

package crypto

// this file contains utility functions for the curve BLS 12-381
// these tools are shared by the BLS signature scheme, the BLS based threshold signature
// and the BLS distributed key generation protocols

// #cgo CFLAGS: -I${SRCDIR}/ -I${SRCDIR}/relic/build/include -I${SRCDIR}/relic/include -I${SRCDIR}/relic/include/low -I${SRCDIR}/blst_src -I${SRCDIR}/blst_src/build -D__BLST_CGO__ -fno-builtin-memcpy -fno-builtin-memset -Wall -Wno-unused-function -Wno-unused-macros
// #cgo LDFLAGS: -L${SRCDIR}/relic/build/lib -l relic_s
// #cgo amd64 CFLAGS: -D__ADX__ -mno-avx
// #cgo mips64 mips64le ppc64 ppc64le riscv64 s390x CFLAGS: -D__BLST_NO_ASM__
// #include "bls12381_utils.h"
import "C"
import (
	"crypto/rand"
	"errors"
)

// Go wrappers around BLST C types
// Go wrappers around Relic C types
type pointE1 C.ep_st
type pointE2 C.E2
type scalar C.Fr

// BLS12-381 related lengths
var frBytesLen = int(C.get_Fr_BYTES())

// TODO: For now scalars are represented as field elements Fr since all scalars
// are less than r - check if distinguishing two types in necessary
//type pointG1_blst C.E1
//type pointG2_blst C.E2

// context required for the BLS set-up
type ctx struct {
	relicCtx *C.ctx_t
	precCtx  *C.prec_st
}

// get some constants from the C layer
// (Cgo does not export C macros)
var valid = C.get_valid()
var invalid = C.get_invalid()

// get some constants from the C layer
// var blst_errors = C.blst_get_errors()
var blst_valid = (int)(C.BLST_SUCCESS)
var blst_bad_encoding = (int)(C.BLST_BAD_ENCODING)
var blst_bad_scalar = (int)(C.BLST_BAD_SCALAR)
var blst_point_not_on_curve = (int)(C.BLST_POINT_NOT_ON_CURVE)

// initContext sets relic B12_381 parameters and precomputes some data in the C layer
func (ct *ctx) initContext() error {
	c := C.relic_init_BLS12_381()
	if c == nil {
		return errors.New("Relic core init failed")
	}
	ct.relicCtx = c
	ct.precCtx = C.init_precomputed_data_BLS12_381()
	return nil
}

// seeds the internal relic random function.
// relic context must be initialized before seeding.
func seedRelic(seed []byte) error {
	if len(seed) < (securityBits / 8) {
		return invalidInputsErrorf(
			"seed length needs to be larger than %d",
			securityBits/8)
	}
	if len(seed) > maxRelicPrgSeed {
		return invalidInputsErrorf(
			"seed length needs to be less than %x",
			maxRelicPrgSeed)
	}
	C.seed_relic((*C.uchar)(&seed[0]), (C.int)(len(seed)))
	return nil
}

// Exponentiation in G1 (scalar point multiplication)
func (p *pointE1) scalarMultG1(res *pointE1, expo *scalar) {
	C.ep_mult((*C.ep_st)(res), (*C.ep_st)(p), (*C.Fr)(expo))
}

// This function is for TEST only
// Exponentiation of g1 in G1
func generatorScalarMultG1(res *pointE1, expo *scalar) {
	C.ep_mult_gen_bench((*C.ep_st)(res), (*C.Fr)(expo))
}

// This function is for TEST only
// Generic Exponentiation G1
func genericScalarMultG1(res *pointE1, expo *scalar) {
	C.ep_mult_generic_bench((*C.ep_st)(res), (*C.Fr)(expo))
}

// Exponentiation of g2 in G2
func generatorScalarMultG2(res *pointE2, expo *scalar) {
	C.G2_mult_gen((*C.E2)(res), (*C.Fr)(expo))
}

// comparison in Fr where r is the group order of G1/G2
// (both scalars should be reduced mod r)
func (x *scalar) equals(other *scalar) bool {
	return C.Fr_is_equal((*C.Fr)(x), (*C.Fr)(other)) != 0
}

// comparison in G2
func (p *pointE2) equals(other *pointE2) bool {
	return C.E2_is_equal((*C.E2)(p), (*C.E2)(other)) != 0
}

// Comparison to zero in Fr.
// Scalar must be already reduced modulo r
func (x *scalar) isZero() bool {
	return C.Fr_is_zero((*C.Fr)(x)) != 0
}

// Comparison to point at infinity in G2.
func (p *pointE2) isInfinity() bool {
	return C.E2_is_infty((*C.E2)(p)) != 0
}

// returns a random element of Fr in input pointer
func randFr(x *scalar) error {
	bytes := make([]byte, frBytesLen+securityBits/8)
	_, err := rand.Read(bytes) // checking one output is enough
	if err != nil {
		return errors.New("internal rng failed")
	}
	_ = mapToZr(x, bytes)
	return nil
}

// writes a random element of Fr* in input pointer
func randFrStar(x *scalar) error {
	bytes := make([]byte, frBytesLen+securityBits/8)
	isZero := true
	for isZero {
		_, err := rand.Read(bytes) // checking one output is enough
		if err != nil {
			return errors.New("internal rng failed")
		}
		isZero = mapToZr(x, bytes)
	}
	return nil
}

// mapToZr reads a scalar from a slice of bytes and maps it to Zr.
// The resulting element `k` therefore satisfies 0 <= k < r.
// It returns true if scalar is zero and false otherwise.
func mapToZr(x *scalar, src []byte) bool {
	isZero := C.map_bytes_to_Fr((*C.Fr)(x),
		(*C.uchar)(&src[0]),
		(C.int)(len(src)))
	return bool(isZero)
}

// writeScalar writes a scalar in a slice of bytes
func writeScalar(dest []byte, x *scalar) {
	C.Fr_write_bytes((*C.uchar)(&dest[0]), (*C.Fr)(x))
}

// writePointG2 writes a G2 point in a slice of bytes
// The slice should be of size PubKeyLenBLSBLS12381 and the serialization
// follows the Zcash format specified in draft-irtf-cfrg-pairing-friendly-curves
func writePointG2(dest []byte, a *pointE2) {
	C.E2_write_bytes((*C.uchar)(&dest[0]), (*C.E2)(a))
}

// writePointG1 writes a G1 point in a slice of bytes
// The slice should be of size SignatureLenBLSBLS12381 and the serialization will
// follow the Zcash format specified in draft-irtf-cfrg-pairing-friendly-curves
func writePointG1(dest []byte, a *pointE1) {
	C.ep_write_bin_compact((*C.uchar)(&dest[0]),
		(*C.ep_st)(a),
		(C.int)(signatureLengthBLSBLS12381),
	)
}

// read an Fr* element from a byte slice
// and stores it into a `scalar` type element.
func readScalarFrStar(a *scalar, src []byte) error {
	read := C.Fr_star_read_bytes(
		(*C.Fr)(a),
		(*C.uchar)(&src[0]),
		(C.int)(len(src)))

	switch int(read) {
	case blst_valid:
		return nil
	case blst_bad_encoding:
		return invalidInputsErrorf("input length must be %d, got %d",
			frBytesLen, len(src))
	case blst_bad_scalar:
		return invalidInputsErrorf("scalar is not in the correct range w.r.t the BLS12-381 curve")
	default:
		return invalidInputsErrorf("reading the scalar failed")
	}

}

// readPointE2 reads a E2 point from a slice of bytes
// The slice is expected to be of size PubKeyLenBLSBLS12381 and the deserialization
// follows the Zcash format specified in draft-irtf-cfrg-pairing-friendly-curves.
// No G2 membership check is performed.
func readPointE2(a *pointE2, src []byte) error {
	read := C.E2_read_bytes((*C.E2)(a),
		(*C.uchar)(&src[0]),
		(C.int)(len(src)))

	switch int(read) {
	case blst_valid:
		return nil
	case blst_bad_encoding, blst_bad_scalar:
		return invalidInputsErrorf("input could not deserialize to a G2 point")
	case blst_point_not_on_curve:
		return invalidInputsErrorf("input is not a point on curve E2")
	default:
		return errors.New("reading a G2 point failed")
	}
}

// readPointE1 reads a E1 point from a slice of bytes
// The slice should be of size SignatureLenBLSBLS12381 and the deserialization
// follows the Zcash format specified in draft-irtf-cfrg-pairing-friendly-curves.
// No G1 membership check is performed.
func readPointE1(a *pointE1, src []byte) error {
	switch C.ep_read_bin_compact((*C.ep_st)(a),
		(*C.uchar)(&src[0]),
		(C.int)(len(src))) {
	case valid:
		return nil
	case invalid:
		return invalidInputsErrorf("input is not a G1 point")
	default:
		return errors.New("reading a G1 point failed")
	}
}

// checkMembershipG1 wraps a call to a subgroup check in G1 since cgo can't be used
// in go test files.
func checkMembershipG1(pt *pointE1) int {
	return int(C.G1_check_membership((*C.ep_st)(pt)))
}

// checkMembershipG2 wraps a call to a subgroup check in G2 since cgo can't be used
// in go test files.
func checkMembershipG2(pt *pointE2) int {
	return int(C.G2_check_membership((*C.E2)(pt)))
}

// randPointG1 wraps a call to C since cgo can't be used in go test files.
// It generates a random point in G1 and stores it in input point.
func randPointG1(pt *pointE1) {
	C.ep_rand_G1((*C.ep_st)(pt))
}

// randPointG1Complement wraps a call to C since cgo can't be used in go test files.
// It generates a random point in E1\G1 and stores it in input point.
func randPointG1Complement(pt *pointE1) {
	C.ep_rand_G1complement((*C.ep_st)(pt))
}

/*
// randPointG2 wraps a call to C since cgo can't be used in go test files.
// It generates a random point in G2 and stores it in input point.
func randPointG2(pt *pointE2) {
	C.ep2_rand_G2((*C.E2)(pt))
}

// randPointG1Complement wraps a call to C since cgo can't be used in go test files.
// It generates a random point in E2\G2 and stores it in input point.
func randPointG2Complement(pt *pointE2) {
	C.ep2_rand_G2complement((*C.E2)(pt))
}
*/

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
	C.map_to_G1((*C.ep_st)(&point), (*C.uchar)(&hash[0]), (C.int)(len(hash)))

	// serialize the point
	pointBytes := make([]byte, signatureLengthBLSBLS12381)
	writePointG1(pointBytes, &point)
	return pointBytes
}
