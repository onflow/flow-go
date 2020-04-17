// +build relic

package crypto

// #cgo CFLAGS: -g -Wall -std=c99 -I./ -I./relic/build/include
// #cgo LDFLAGS: -Lrelic/build/lib -l relic_s
// #include "bls_include.h"
import "C"
import (
	"errors"
	"fmt"
)

// Go wrappers to Relic C types
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

// reInitContext re init the context of the C layer with pre-saved data
func (ct *ctx) reInitContext() {
	C.core_set(ct.relicCtx)
	C.precomputed_data_set(ct.precCtx)
}

// Exponentiation in G1 (scalar point multiplication)
func (p *pointG1) _G1scalarPointMult(res *pointG1, expo *scalar) {
	C._G1scalarPointMult((*C.ep_st)(res), (*C.ep_st)(p), (*C.bn_st)(expo))
}

// Exponentiation of g1 in G1
// This function is for DEBUG/TESTs purpose only
func _G1scalarGenMult(res *pointG1, expo *scalar) {
	C._G1scalarGenMult((*C.ep_st)(res), (*C.bn_st)(expo))
}

// Exponentiation of g2 in G2
func _G2scalarGenMult(res *pointG2, expo *scalar) {
	C._G2scalarGenMult((*C.ep2_st)(res), (*C.bn_st)(expo))
}

// comparison in Zp (both scalars are reduced mod p)
func (x *scalar) equals(other *scalar) bool {
	return C.bn_cmp((*C.bn_st)(x), (*C.bn_st)(other)) == valid
}

// comparison in G2
func (p *pointG2) equals(other *pointG2) bool {
	return C.ep2_cmp((*C.ep2_st)(p), (*C.ep2_st)(other)) == valid
}

// seeds the internal relic random function
func seedRelic(seed []byte) error {
	if len(seed) < (securityBits / 8) {
		return fmt.Errorf("seed length needs to be larger than %d",
			securityBits/8)
	}
	if len(seed) > maxRelicPrgSeed {
		return fmt.Errorf("seed length needs to be less than %x",
			maxRelicPrgSeed)
	}
	C.seed_relic((*C.uchar)(&seed[0]), (C.int)(len(seed)))
	return nil
}

// returns a random number in Zr
func randZr(x *scalar) error {
	C.bn_randZr((*C.bn_st)(x))
	if x == nil {
		return errors.New("the memory allocation of the random number has failed")
	}
	return nil
}

// mapKeyZr reads a private key from a slice of bytes and maps it to Zr
// the resulting scalar is in the range 0 < k < r
func mapKeyZr(x *scalar, src []byte) error {
	if len(src) > maxScalarSize {
		return fmt.Errorf("input slice length must be less than %d", maxScalarSize)
	}
	C.bn_privateKey_mod_r((*C.bn_st)(x),
		(*C.uchar)(&src[0]),
		(C.int)(len(src)))
	return nil
}

// TEST/DEBUG/BENCH
// returns the hash to G1 point
func hashToG1(data []byte) *pointG1 {
	l := len(data)
	var h pointG1
	C.mapToG1((*C.ep_st)(&h), (*C.uchar)(&data[0]), (C.int)(l))
	return &h
}

// sets a scalar to a small integer
func (x *scalar) setInt(a int) {
	C.bn_set_dig((*C.bn_st)(x), (C.uint64_t)(a))
}

// writeScalar writes a G2 point in a slice of bytes
func writeScalar(dest []byte, x *scalar) {
	C.bn_write_bin((*C.uchar)(&dest[0]),
		(C.int)(prKeyLengthBlsBls12381),
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
func writePointG2(dest []byte, a *pointG2) {
	C._ep2_write_bin_compact((*C.uchar)(&dest[0]),
		(*C.ep2_st)(a),
		(C.int)(pubKeyLengthBlsBls12381),
	)
}

// readVerifVector reads a G2 point from a slice of bytes
func readPointG2(a *pointG2, src []byte) error {
	if C.ep2_read_bin_compact((*C.ep2_st)(a),
		(*C.uchar)(&src[0]),
		(C.int)(len(src)),
	) != valid {
		return errors.New("reading a G2 point has failed")
	}
	return nil
}
