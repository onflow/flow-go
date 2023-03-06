// +build relic

#ifndef __BLST_INCLUDE_H__
#define __BLST_INCLUDE_H__

// extra tools to use BLST low level that are needed by the Flow crypto library
// eventually this file would replace blst.h

//#include "blst.h" // TODO: should be deleted
#include "point.h"
#include "consts.h"

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

typedef enum {
    BLST_SUCCESS = 0,
    BLST_BAD_ENCODING,
    BLST_POINT_NOT_ON_CURVE,
    BLST_POINT_NOT_IN_GROUP,
    BLST_AGGR_TYPE_MISMATCH,
    BLST_VERIFY_FAIL,
    BLST_PK_IS_INFINITY,
    BLST_BAD_SCALAR,
} BLST_ERROR;

typedef uint8_t byte;

// field elements F_r
// where `r` is the order of G1/G2.
// F_r elements are represented as big numbers reduced modulo `r`. Big numbers
// are represented as a little endian vector of limbs.
// `Fr` is equivalent to type vec256 (used internally by BLST for F_r elements).
typedef struct {limb_t limbs[4];} Fr;
// Subroup G1 in E1
typedef POINTonE1 G1;
// Subroup G1 in E2
typedef POINTonE2 G2;

#endif
