// +build relic

package crypto

// this file contains utility functions for the curve BLS 12-381
// these tools are shared by the BLS signature scheme, the BLS based threshold signature
// and the BLS distributed key generation protcols

// #cgo CFLAGS: -g -Wall -std=c99 -I./ -I./relic/build/include
// #cgo LDFLAGS: -Lrelic/build/lib -l relic_s
// #include "bls12381_utils.h"
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
		return newInvalidInputsError(
			"seed length needs to be larger than %d",
			securityBits/8)
	}
	if len(seed) > maxRelicPrgSeed {
		return newInvalidInputsError(
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
func genScalarMultG1(res *pointG1, expo *scalar) {
	C.ep_mult_gen((*C.ep_st)(res), (*C.bn_st)(expo))
}

// Exponentiation of g2 in G2
func genScalarMultG2(res *pointG2, expo *scalar) {
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

// returns a random number in Zr
func randZr(x *scalar) {
	C.bn_randZr((*C.bn_st)(x))
}

// returns a random non-zero number in Zr
func randZrStar(x *scalar) {
	C.bn_randZr_star((*C.bn_st)(x))
}

// mapToZr reads a scalar from a slice of bytes and maps it to Zr
// the resulting scalar is in the range 0 < k < r
func mapToZr(x *scalar, src []byte) error {
	if len(src) > maxScalarSize {
		return newInvalidInputsError(
			"input slice length must be less than %d",
			maxScalarSize)
	}
	C.bn_map_to_Zr_star((*C.bn_st)(x),
		(*C.uchar)(&src[0]),
		(C.int)(len(src)))
	return nil
}

// writeScalar writes a G2 point in a slice of bytes
func writeScalar(dest []byte, x *scalar) {
	C.bn_write_bin((*C.uchar)(&dest[0]),
		(C.int)(prKeyLengthBLSBLS12381),
		(*C.bn_st)(x),
	)
}

// readScalar reads a scalar from a slice of bytes
func readScalar(x *scalar, src []byte) {
	C.bn_read_bin((*C.bn_st)(x),
		(*C.uchar)(&src[0]),
		(C.int)(len(src)),
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
		return newInvalidInputsError("input is not a G2 point")
	default:
		return errors.New("reading a G2 point has failed")
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
		return newInvalidInputsError("input is not a G1 point")
	default:
		return errors.New("reading a G1 point has failed")
	}
}

// This is only a TEST function.
// It wraps calls to subgroup checks since cgo can't be used
// in go test files.
// if inG1 is true, the function tests the membership of a point in G1,
// otherwise, a point in E1\G1 membership is tested.
// method is the index of the membership check method as defined in bls12381_utils.h
func checkG1Test(inG1 int, method int) bool {
	return C.subgroup_check_G1_test((C.int)(inG1), (C.int)(method)) == valid
}

// This is only a TEST function.
// It wraps a call to a subgroup check in G1 since cgo can't be used
// in go test files.
func checkInG1Test(pt *pointG1) bool {
	return C.check_membership_G1((*C.ep_st)(pt)) == valid
}

// This is only a TEST function.
// It wraps calls to subgroup checks since cgo can't be used
// in go test files.
func benchG1Test() {
	_ = C.subgroup_check_G1_bench()
}

// This is only a TEST function.
// It hashes `data` to a G1 point using the tag `dst` and returns the G1 point serialization.
// The function uses xmd with SHA256 in the hash-to-field.
func hashToG1Bytes(data, dst []byte) []byte {
	hash := make([]byte, opSwUInputLenBLSBLS12381)
	// XMD using SHA256
	C.xmd_sha256((*C.uchar)(&hash[0]),
		(C.int)(opSwUInputLenBLSBLS12381),
		(*C.uchar)(&data[0]), (C.int)(len(data)),
		(*C.uchar)(&dst[0]), (C.int)(len(dst)))

	// map the hash to G1
	var point pointG1
	C.map_to_G1((*C.ep_st)(&point), (*C.uchar)(&hash[0]), (C.int)(len(hash)))

	// serialize the point
	pointBytes := make([]byte, signatureLengthBLSBLS12381)
	writePointG1(pointBytes, &point)
	return pointBytes
}
