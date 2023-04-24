//go:build relic
// +build relic

package crypto

// this file contains utility functions for the curve BLS 12-381
// these tools are shared by the BLS signature scheme, the BLS based threshold signature
// and the BLS distributed key generation protocols

// #cgo CFLAGS: -g -Wall -std=c99 -I${SRCDIR}/ -I${SRCDIR}/relic/build/include -I${SRCDIR}/relic/include -I${SRCDIR}/relic/include/low
// #cgo LDFLAGS: -L${SRCDIR}/relic/build/lib -l relic_s
// #include "bls12381_utils.h"
// #include "bls_include.h"
import "C"
import (
	"errors"
)

// Go wrappers to Relic C types
// Relic is compiled with ALLOC=AUTO
type pointG1 C.ep_st
type pointG2 C.ep2_st
type scalar C.bn_st

// context required for the BLS set-up
type ctx struct {
	relicCtx *C.ctx_t
	precCtx  *C.prec_st
}

// get some constants from the C layer
// (Cgo does not export C macros)
var valid = C.get_valid()
var invalid = C.get_invalid()

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

// setContext sets the context (previously initialized) of the C layer with
// pre-saved data.
func (ct *ctx) setContext() {
	C.core_set(ct.relicCtx)
	C.precomputed_data_set(ct.precCtx)
}

// Exponentiation in G1 (scalar point multiplication)
func (p *pointG1) scalarMultG1(res *pointG1, expo *scalar) {
	C.ep_mult((*C.ep_st)(res), (*C.ep_st)(p), (*C.bn_st)(expo))
}

// This function is for TEST only
// Exponentiation of g1 in G1
func generatorScalarMultG1(res *pointG1, expo *scalar) {
	C.ep_mult_gen_bench((*C.ep_st)(res), (*C.bn_st)(expo))
}

// This function is for TEST only
// Generic Exponentiation G1
func genericScalarMultG1(res *pointG1, expo *scalar) {
	C.ep_mult_generic_bench((*C.ep_st)(res), (*C.bn_st)(expo))
}

// Exponentiation of g2 in G2
func generatorScalarMultG2(res *pointG2, expo *scalar) {
	C.ep2_mult_gen((*C.ep2_st)(res), (*C.bn_st)(expo))
}

// comparison in Zr where r is the group order of G1/G2
// (both scalars should be reduced mod r)
func (x *scalar) equals(other *scalar) bool {
	return C.bn_cmp((*C.bn_st)(x), (*C.bn_st)(other)) == valid
}

// comparison in G2
func (p *pointG2) equals(other *pointG2) bool {
	return C.ep2_cmp((*C.ep2_st)(p), (*C.ep2_st)(other)) == valid
}

// Comparison to zero in Zr.
// Scalar must be already reduced modulo r
func (x *scalar) isZero() bool {
	return C.bn_is_zero((*C.bn_st)(x)) == 1
}

// Comparison to point at infinity in G2.
func (p *pointG2) isInfinity() bool {
	return C.ep2_is_infty((*C.ep2_st)(p)) == 1
}

// returns a random number in Zr
func randZr(x *scalar) {
	C.bn_randZr((*C.bn_st)(x))
}

// returns a random non-zero number in Zr
func randZrStar(x *scalar) {
	C.bn_randZr_star((*C.bn_st)(x))
}

// mapToZr reads a scalar from a slice of bytes and maps it to Zr.
// The resulting scalar `k` satisfies 0 <= k < r.
// It returns true if scalar is zero and false otherwise.
func mapToZr(x *scalar, src []byte) bool {
	isZero := C.bn_map_to_Zr((*C.bn_st)(x),
		(*C.uchar)(&src[0]),
		(C.int)(len(src)))
	return isZero == valid
}

// writeScalar writes a G2 point in a slice of bytes
func writeScalar(dest []byte, x *scalar) {
	C.bn_write_bin((*C.uchar)(&dest[0]),
		(C.ulong)(prKeyLengthBLSBLS12381),
		(*C.bn_st)(x),
	)
}

// readScalar reads a scalar from a slice of bytes
func readScalar(x *scalar, src []byte) {
	C.bn_read_bin((*C.bn_st)(x),
		(*C.uchar)(&src[0]),
		(C.ulong)(len(src)),
	)
}

// writePointG2 writes a G2 point in a slice of bytes
// The slice should be of size PubKeyLenBLSBLS12381 and the serialization will
// follow the Zcash format specified in draft-irtf-cfrg-pairing-friendly-curves
func writePointG2(dest []byte, a *pointG2) {
	C.ep2_write_bin_compact((*C.uchar)(&dest[0]),
		(*C.ep2_st)(a),
		(C.int)(pubKeyLengthBLSBLS12381),
	)
}

// writePointG1 writes a G1 point in a slice of bytes
// The slice should be of size SignatureLenBLSBLS12381 and the serialization will
// follow the Zcash format specified in draft-irtf-cfrg-pairing-friendly-curves
func writePointG1(dest []byte, a *pointG1) {
	C.ep_write_bin_compact((*C.uchar)(&dest[0]),
		(*C.ep_st)(a),
		(C.int)(signatureLengthBLSBLS12381),
	)
}

// readPointG2 reads a G2 point from a slice of bytes
// The slice is expected to be of size PubKeyLenBLSBLS12381 and the deserialization will
// follow the Zcash format specified in draft-irtf-cfrg-pairing-friendly-curves
func readPointG2(a *pointG2, src []byte) error {
	switch C.ep2_read_bin_compact((*C.ep2_st)(a),
		(*C.uchar)(&src[0]),
		(C.int)(len(src))) {
	case valid:
		return nil
	case invalid:
		return invalidInputsErrorf("input is not a G2 point")
	default:
		return errors.New("reading a G2 point failed")
	}
}

// readPointG1 reads a G1 point from a slice of bytes
// The slice should be of size SignatureLenBLSBLS12381 and the deserialization will
// follow the Zcash format specified in draft-irtf-cfrg-pairing-friendly-curves
func readPointG1(a *pointG1, src []byte) error {
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
func checkMembershipG1(pt *pointG1) int {
	return int(C.check_membership_G1((*C.ep_st)(pt)))
}

// checkMembershipG2 wraps a call to a subgroup check in G2 since cgo can't be used
// in go test files.
func checkMembershipG2(pt *pointG2) int {
	return int(C.check_membership_G2((*C.ep2_st)(pt)))
}

// randPointG1 wraps a call to C since cgo can't be used in go test files.
// It generates a random point in G1 and stores it in input point.
func randPointG1(pt *pointG1) {
	C.ep_rand_G1((*C.ep_st)(pt))
}

// randPointG1Complement wraps a call to C since cgo can't be used in go test files.
// It generates a random point in E1\G1 and stores it in input point.
func randPointG1Complement(pt *pointG1) {
	C.ep_rand_G1complement((*C.ep_st)(pt))
}

// randPointG2 wraps a call to C since cgo can't be used in go test files.
// It generates a random point in G2 and stores it in input point.
func randPointG2(pt *pointG2) {
	C.ep2_rand_G2((*C.ep2_st)(pt))
}

// randPointG1Complement wraps a call to C since cgo can't be used in go test files.
// It generates a random point in E2\G2 and stores it in input point.
func randPointG2Complement(pt *pointG2) {
	C.ep2_rand_G2complement((*C.ep2_st)(pt))
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
	var point pointG1
	C.map_to_G1((*C.ep_st)(&point), (*C.uchar)(&hash[0]), (C.int)(len(hash)))

	// serialize the point
	pointBytes := make([]byte, signatureLengthBLSBLS12381)
	writePointG1(pointBytes, &point)
	return pointBytes
}
