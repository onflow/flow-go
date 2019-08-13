package crypto

// #cgo CFLAGS: -g -Wall -I./ -I./relic/include/ -I./relic/include/low
// #cgo LDFLAGS: -Lrelic/lib/ -l relic_s
// #include "include.h"
import "C"

// TODO: remove -wall after reaching a stable version
// TDOD: enable QUIET in relic
import (
	"unsafe"

	log "github.com/sirupsen/logrus"
)

// Go wrappers to C types
type pointG1 C.ep_st
type pointG2 C.ep2_st
type scalar C.bn_st
type ctx *C.ctx_t

func signatureLengthBLS_BLS12381() int {
	return int(C._getSignatureLengthBLS_BLS12381())
}

func pubKeyLengthBLS_BLS12381() int {
	return int(C._getPubKeyLengthBLS_BLS12381())
}

func prKeyLengthBLS_BLS12381() int {
	return int(C._getPrKeyLengthBLS_BLS12381())
}

// init sets the context of BLS12381 curve
func (a *BLS_BLS12381Algo) init() {
	// sanity checks of lengths
	if a.PrKeyLength != PrKeyLengthBLS_BLS12381 ||
		a.PubKeyLength != PubKeyLengthBLS_BLS12381 ||
		a.SignatureLength != SignatureLengthBLS_BLS12381 {
		log.Warn("BLS Lengths in types.go are not matching include.h")
	}

	// Inits relic context and sets the B12_381 context
	c := C._relic_init_BLS12_381()
	if c == nil {
		panic("Relic core init failed")
	}
	a.context = c
}

// reinit the context of BLS12381 curve assuming there was a previous call to init()
// should be called at every a. operation
func (a *BLS_BLS12381Algo) reinit() {
	if ctx(C.core_get()) != a.context {
		C.core_set(a.context)
	}
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

// TEST/DEBUG
// returns a random number on Z/Z.r
func randZr(x *scalar, seed []byte) {
	C._bn_randZr((*C.bn_st)(x), (*C.uchar)((unsafe.Pointer)(&seed[0]))) // to define the length of seed
	if x == nil {
		panic("the memory allocation of the random number has failed")
	}
}

// TEST/DEBUG/BENCH
// returns the hash to G1 point
func hashToG1(data []byte) {
	var l int
	l = len(data)
	_ = C._hashToG1((*C.uchar)((unsafe.Pointer)(&data[0])), (C.int)(l))
}

// sets a scalar to a small integer
func (x *scalar) setInt(a int) {
	C.bn_set_dig((*C.bn_st)(x), (C.uint64_t)(a))
}

// computes a bls signature
func (a *BLS_BLS12381Algo) blsSign(sk *scalar, data []byte) Signature {
	var s Signature
	s = make([]byte, a.SignatureLength) // 25ns

	C._blsSign((*C.uchar)((unsafe.Pointer)(&s[0])), // 2.1 ms
		(*C.bn_st)(sk),
		(*C.uchar)((unsafe.Pointer)(&data[0])),
		(C.int)(len(data)))
	return s
}

// Checks the validity of a bls signature
func (a *BLS_BLS12381Algo) blsVerify(pk *pointG2, s Signature, data []byte) bool {
	verif := C._blsVerify((*C.ep2_st)(pk),
		(*C.uchar)((unsafe.Pointer)(&s[0])),
		(*C.uchar)((unsafe.Pointer)(&data[0])),
		(C.int)(len(data)))

	const SIG_VALID = 1 // same value as in include.h
	const SIG_ERR = 0xFF

	if verif == SIG_ERR {
		panic("Relic memory allocation failed")
	}
	return (verif == SIG_VALID)
}
