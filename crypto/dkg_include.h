// +build relic

#ifndef _REL_DKG_INCLUDE_H
#define _REL_DKG_INCLUDE_H

#include "bls12381_utils.h"

void Fr_polynomialImage_export(byte* out, ep2_t y, const Fr* a, const int a_size, const byte x);
void Fr_polynomialImage(Fr* out, ep2_t y, const Fr* a, const int a_size, const byte x);
void G2_polynomialImages(ep2_st* y, const int len_y, const ep2_st* A, const int len_A);
void ep2_vector_write_bin(byte* out, const ep2_st* A, const int len);
int  ep2_vector_read_bin(ep2_st* A, const byte* src, const int len);
int  verifyshare(const Fr* x, const ep2_t y);

#endif
