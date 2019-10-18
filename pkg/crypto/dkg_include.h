
#ifndef _REL_DKG_INCLUDE_H
#define _REL_DKG_INCLUDE_H

#include "relic.h"
#include "misc.h"

void Zr_polynomialImage(byte* out, ep2_st* y, const bn_st* a, const int a_size, const int x);
void G2_polynomialImages(ep2_st* y, int len_y, ep2_st* A, int len_A);
void ep2_vector_write_bin(byte* out, const ep2_st* A, const int len);
void ep2_vector_read_bin(ep2_st* A, const byte* src, const int len);
int verifyshare(bn_st* x, ep2_st* y);

#endif