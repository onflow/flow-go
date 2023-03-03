// +build relic

#ifndef __BLST_INCLUDE_H__
#define __BLST_INCLUDE_H__

// extra tools to use BLST low level that are needed by the Flow crypto library
// eventually this file would replace blst.h

#include "blst.h" // TODO: should be deleted
#include "point.h"
#include "consts.h"

// types used by the Flow crypto library that are imported from BLST
// these type definitions are used as an abstraction from BLST internal types

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
