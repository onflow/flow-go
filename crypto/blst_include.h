// +build relic

#ifndef __BLST_INCLUDE_H__
#define __BLST_INCLUDE_H__

// extra tools to use BLST low level that are needed by the Flow crypto library
// eventually this file would replace blst.h

#include "bls12381_utils.h"
#include "point.h"
#include "fields.h"
#include "consts.h"
#include "errors.h"
#include "sha256.h"

// types used by the Flow crypto library that are imported from BLST
// these type definitions are used as an abstraction from BLST internal types

// Parts of this file have been copied from blst.h in the BLST repo
/*
 * Copyright Supranational LLC
 * Licensed under the Apache License, Version 2.0, see LICENSE for details.
 * SPDX-License-Identifier: Apache-2.0
 */

#ifdef __SIZE_TYPE__
typedef __SIZE_TYPE__ size_t;
#else
#include <stddef.h>
#endif

#if defined(__UINT8_TYPE__) && defined(__UINT32_TYPE__) \
                            && defined(__UINT64_TYPE__)
typedef __UINT8_TYPE__  uint8_t;
typedef __UINT32_TYPE__ uint32_t;
typedef __UINT64_TYPE__ uint64_t;
#else
#include <stdint.h>
#endif

typedef uint8_t byte;

#ifdef __cplusplus
extern "C" {
#elif defined(__BLST_CGO__)
typedef _Bool bool; /* it's assumed that cgo calls modern enough compiler */
#elif defined(__STDC_VERSION__) && __STDC_VERSION__>=199901
# define bool _Bool
#else
# define bool int
#endif

#ifdef SWIG
# define DEFNULL =NULL
#elif defined __cplusplus
# define DEFNULL =0
#else
# define DEFNULL
#endif

// TODO: add sanity checks that BLST_PK_IS_INFINITY is indeed the last
// enum value (eventually submit a fix to BLST)
#define BLST_BAD_SCALAR ((BLST_PK_IS_INFINITY)+1)

// field elements F_r
// where `r` is the order of G1/G2.
// F_r elements are represented as big numbers reduced modulo `r`. Big numbers
// are represented as a little endian vector of limbs.
// `Fr` is equivalent to type `vec256` (used internally by BLST for F_r elements).
// `Fr` is defined as a struct to be exportable through cgo to the Go layer.
#define R_BITS 255
typedef struct {limb_t limbs[(R_BITS+63)/64];} Fr; // TODO: use Fr_LIMBS

// field elements F_p
// F_p elements are represented as big numbers reduced modulo `p`. Big numbers
// are represented as a little endian vector of limbs.
// `Fp` is equivalent to type `vec384` (used internally by BLST for F_p elements).
// `Fp` does not need to be exported to cgo.
typedef vec384 Fp;

// curve E_1 (over F_p)
// E_1 points are represented in Jacobian coordinates (x,y,z), 
// where x, y, x are elements of F_p (type `Fp`).
// `E1` is equivelent to type `POINTonE1` (used internally by BLST for Jacobian E1 elements)
// `E1` is defined as a struct to be exportable through cgo to the Go layer.
// `E1` is also used to represent all subgroup G_1 elements. 
typedef struct {Fp x,y,z;} E1;

// field elements F_p^2
// F_p^2 elements are represented as a vector of two F_p elements.
// `Fp2` is equivalent to type `vec384x` (used internally by BLST for F_p^2 elements).
// `Fp2` does not need to be exported to cgo.
typedef vec384x Fp2;   
// helpers to get "real" and "imaginary" Fp elements from Fp2 pointers
#define real(p)  ((*(p))[0])  
#define imag(p)  ((*(p))[1]) 


// curve E_2 (over F_p^2)
// E_2 points are represented in Jacobian coordinates (x,y,z), 
// where x, y, x are elements of F_p (type `Fp`).
// `E2` is equivelent to type `POINTonE2` (used internally by BLST for Jacobian E2 elements)
// `E2` is defined as a struct to be exportable through cgo to the Go layer.
// `E2` is also used to represent all subgroup G_2 elements. 
typedef struct {Fp2 x,y,z;} E2;

#endif
