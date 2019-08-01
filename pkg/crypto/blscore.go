package crypto

// #cgo CFLAGS: -g -Wall -I./ -I./relic/include/ -I./relic/include/low
// #cgo LDFLAGS: -Lrelic/lib/ -l relic_s
// #include "relic_include.h"
import "C"

// TODO: relic configs to check:
// BN_KARAT: try 2, if karatsuba is used
// try sliding window expo with 4-5-6
import (
	_ "errors"
	_ "fmt"
	"unsafe"
)

// Go wrappers to C types
type pointG1 C.ep_st
type pointG2 C.ep2_st
type scalar C.bn_st
type ctx *C.ctx_t

// init sets the context of BLS12381 curve
func (a *BLS_BLS12381Algo) init() {
	C.core_init()
	//C.ep_param_set(B12_P381)
	C.ep_param_set_any_pairf() // sets B12_P381 if FP_PRIME = 381 in relic config
	a.context = ctx(C.core_get())
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
	C._bn_print(C.CString("k"), (*C.bn_st)(expo))
	C._G1scalarPointMult((*C.ep_st)(res), (*C.ep_st)(p), (*C.bn_st)(expo))
	C._ep_print(C.CString("kG1"), (*C.ep_st)(res))
}

// Exponentiation of g1 in G1
func _G1scalarGenMult(res *pointG1, expo *scalar) {
	C._G1scalarGenMult((*C.ep_st)(res), (*C.bn_st)(expo))
	C._ep_print(C.CString("kG1"), (*C.ep_st)(res))
}

// Exponentiation of g2 in G2
func _G2scalarGenMult(res *pointG2, expo *scalar) {
	C._G2scalarGenMult((*C.ep2_st)(res), (*C.bn_st)(expo))
	C._ep2_print(C.CString("kG2"), (*C.ep2_st)(res))
}

// TEST/DEBUG
// returnd a random number on Z/Z.r
func randZr(x *scalar, seed []byte) {
	C.bn_randZr((*C.bn_st)(x), (*C.char)((unsafe.Pointer)(&seed[0]))) // to define the length of seed
}

func (x *scalar) setInt(a int) {
	C.bn_set_dig((*C.bn_st)(x), (C.uint64_t)(a))
}
