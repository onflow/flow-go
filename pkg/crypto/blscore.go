package crypto

// #cgo CFLAGS: -g -Wall -I./ -I./relic/include/ -I./relic/include/low
// #cgo LDFLAGS: -Lrelic/lib/ -l relic_s
// #include "include.h"
import "C"

// TODO: remove -wall after reaching a stable version
// TDOD: enable QUIET in relic
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
	C._bn_print(C.CString("k"), (*C.bn_st)(expo))
	C._G1scalarPointMult((*C.ep_st)(res), (*C.ep_st)(p), (*C.bn_st)(expo))
	C._ep_print(C.CString("kG1"), (*C.ep_st)(res))
}

// Exponentiation of g1 in G1
// This function is for DEBUG/TESTs purpose only
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
// returns a random number on Z/Z.r
func randZr(x *scalar, seed []byte) {
	C._bn_randZr((*C.bn_st)(x), (*C.char)((unsafe.Pointer)(&seed[0]))) // to define the length of seed
	if x == nil {
		panic("the memory allocation of the random number has failed")
	}
}

func (x *scalar) setInt(a int) {
	C.bn_set_dig((*C.bn_st)(x), (C.uint64_t)(a))
}
