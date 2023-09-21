#ifndef __BLST_INCLUDE_H__
#define __BLST_INCLUDE_H__

// BLST src headers
#include "consts.h"
#include "fields.h"
#include "point.h"

// types used by the Flow crypto library that are imported from BLST.
// these type definitions are used as an abstraction from BLST internal types.

// field elements F_r
// where `r` is the order of G1/G2.
// F_r elements are represented as big numbers reduced modulo `r`. Big numbers
// are represented as a little endian vector of limbs.
// `Fr` is equivalent to type `vec256` (used internally by BLST for F_r
// elements). `Fr` is defined as a struct so that it can be exportable through
// cgo to the Go layer.
#define R_BITS 255 // equal to Fr_bits in bls12381_utils.h
typedef struct {
  limb_t limbs[(R_BITS + 63) / 64];
} Fr;

// field elements F_p
// F_p elements are represented as big numbers reduced modulo `p`. Big numbers
// are represented as a little endian vector of limbs.
// `Fp` is equivalent to type `vec384` (used internally by BLST for F_p
// elements). `Fp` does not need to be exported to cgo.
typedef vec384 Fp;

// curve E_1 (over F_p)
// E_1 points are represented in Jacobian coordinates (x,y,z),
// where x, y, z are elements of F_p (type `Fp`).
// `E1` is equivalent to type `POINTonE1` (used internally by BLST for Jacobian
// E1 elements) `E1` is defined as a struct to be exportable through cgo to the
// Go layer. `E1` is also used to represent all subgroup G_1 elements.
typedef struct {
  Fp x, y, z;
} E1;

// field elements F_p^2
// F_p^2 elements are represented as a vector of two F_p elements.
// `Fp2` is equivalent to type `vec384x` (used internally by BLST for F_p^2
// elements). `Fp2` does not need to be exported to cgo.
typedef vec384x Fp2;
// helpers to get "real" and "imaginary" Fp elements from Fp2 pointers
#define real(p) ((*(p))[0])
#define imag(p) ((*(p))[1])

// curve E_2 (over F_p^2)
// E_2 points are represented in Jacobian coordinates (x,y,z),
// where x, y, z are elements of F_p (type `Fp`).
// `E2` is equivelent to type `POINTonE2` (used internally by BLST for Jacobian
// E2 elements) `E2` is defined as a struct to be exportable through cgo to the
// Go layer. `E2` is also used to represent all subgroup G_2 elements.
typedef struct {
  Fp2 x, y, z;
} E2;

// Fp12 is the codomain of the pairing function `e`, specifically the subgroup
// G_T of Fp12.
// Fp12 represents G_T elements and is equivalent to `vec384fp12` (used
// internally by BLST)
typedef vec384fp12 Fp12;
#endif
