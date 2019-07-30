package crypto

// #cgo CFLAGS: -g -Wall -I./ -I./relic/include/ -I./relic/include/low
// #cgo LDFLAGS: -Lrelic/lib/ -l relic_s
// #include "relic_include.h"
import "C"

// TODO: relic configs to check:   
// BN_KARAT: try 2, if karatsuba is used
// try sliding window expo with 4-5-6
import (
	"math/big"
	"unsafe"
	//"fmt"
)

type _G1point struct {
	p C.ep_st
}

func (p *_G1point) _G1_scalarPointMult(scalar big.Int) *_G1point {
	var bn_scalar C.bn_st
	C._bn_new(&bn_scalar)
	defer C._bn_free(&bn_scalar)

	byteScalar := scalar.Bytes()
	if len(byteScalar) == 0 {
		byteScalar = []byte{0}
	}
	C.bn_read_bin(&bn_scalar, (*C.uint8_t)((unsafe.Pointer)(&byteScalar[0])), C.int(len(byteScalar)))

	var res _G1point
	C.ep_mul_basic(&res.p, &p.p, &bn_scalar)
	return &res
}

// B12_381 from relic_fp.h
const B12_P381 = 22

// _G1_scalarGenMult is a test function
func _G1_scalarGenMult(scalar big.Int) *_G1point {
	C.core_init()
	//C.ep_param_set(B12_P381)
	C.ep_param_set_any_pairf() // sets B12_P381 if FP_PRIME = 381 !

	var ep_g1 C.ep_st
	C._ep_new(&ep_g1)
	defer C._ep_free(&ep_g1)
	C.ep_curve_get_gen(&ep_g1)
	g1 := _G1point{ep_g1}
	C._fp_print(&g1.p.x)
	res := g1._G1_scalarPointMult(scalar)
	C._fp_print(&res.p.x)

	return res
}
