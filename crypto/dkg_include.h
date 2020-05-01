// +build relic

#ifndef _REL_DKG_INCLUDE_H
#define _REL_DKG_INCLUDE_H

#include "bls12381_utils.h"

// the highest index of a DKG node
#define MAX_IND         255
#define MAX_IND_BITS    8

void Zr_polynomialImage(byte* out, ep2_st* y, const bn_st* a, const int a_size, const byte x);
void G2_polynomialImages(ep2_st* y, const int len_y, const ep2_st* A, const int len_A);
void ep2_vector_write_bin(byte* out, const ep2_st* A, const int len);
int  ep2_vector_read_bin(ep2_st* A, const byte* src, const int len);
int  verifyshare(const bn_st* x, const ep2_st* y);
void bn_sum_vector(bn_st* jointx, bn_st* x, int len);
void ep2_sum_vector(ep2_st* jointx, ep2_st* x, int len);

#endif