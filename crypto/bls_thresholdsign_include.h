// +build relic

#ifndef _REL_THRESHOLD_INCLUDE_H
#define _REL_THRESHOLD_INCLUDE_H

#include "bls_include.h"

// the highest k such that fact(MAX_IND)/fact(MAX_IND-k) < r 
// (approximately Fr_bits/MAX_IND_BITS)
#define MAX_IND_LOOPS   32 

int G1_lagrangeInterpolateAtZero(byte*, const byte* , const uint8_t*, const int);
extern void Zr_polynomialImage(bn_t out, ep2_t y, const bn_st* a, const int a_size, const byte x);

#endif
