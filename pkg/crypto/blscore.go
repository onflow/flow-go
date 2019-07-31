package crypto

// #cgo CFLAGS: -g -Wall -I./ -I./relic/include/ -I./relic/include/low
// #cgo LDFLAGS: -Lrelic/lib/ -l relic_s
// #include "relic_include.h"
import "C"

// TODO: relic configs to check:
// BN_KARAT: try 2, if karatsuba is used
// try sliding window expo with 4-5-6
import (
	"errors"
	"math/big"
	"unsafe"
	//"fmt"
)

// Go wrappers to C types
type pointG1 C.ep_st
type scalar C.bn_st
type ctx *C.ctx_t

// init sets the context of BLS12381 curve
func (a *BLS_BLS12381Algo) init() {
	C.core_init()
	//C.ep_param_set(B12_P381)
	C.ep_param_set_any_pairf() // sets B12_P381 if FP_PRIME = 381 !
	a.context = ctx(C.core_get())
}

// reinit the context of BLS12381 curve assuming there was a previous call to init()
// should be called at every a. operation
func (a *BLS_BLS12381Algo) reinit() {
	if ctx(C.core_get()) != a.context {
		C.core_set(a.context)
	}
}

func (p *pointG1) _G1scalarPointMult(b big.Int) *pointG1 {
	var expo scalar
	C._bn_new((*C.bn_st)(&expo))
	defer C._bn_free((*C.bn_st)(&expo))

	byteScalar := b.Bytes()
	if len(byteScalar) == 0 {
		byteScalar = []byte{0}
	}
	C.bn_read_bin((*C.bn_st)(&expo), (*C.uint8_t)((unsafe.Pointer)(&byteScalar[0])), C.int(len(byteScalar)))

	var res pointG1
	C.ep_mul_basic((*C.ep_st)(&res), (*C.ep_st)(p), (*C.bn_st)(&expo))
	return &res
}

// _G1scalarGenMult is a test function
func _G1scalarGenMult(scalar big.Int) *pointG1 {

	var p pointG1
	C._ep_new((*C.ep_st)(&p))
	defer C._ep_free((*C.ep_st)(&p))
	C.ep_curve_get_gen((*C.ep_st)(&p))
	C._fp_print(&(*C.ep_st)(&p).x)
	res := (&p)._G1scalarPointMult(scalar)
	C._fp_print(&res.x)

	return res
}


func randZr(seed []byte) *scalar {
	var x *C.bn_st
	x = C.bn_randZr((*C.char)((unsafe.Pointer)(&seed[0]))) // to define the length of seed
	defer C.free((unsafe.Pointer)(x))
	
	if x == nil {
		panic(errors.New("randZr output is not allocated"))
	}

	return (*scalar)(x)
}
