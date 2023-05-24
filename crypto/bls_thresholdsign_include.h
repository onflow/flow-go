#ifndef _REL_THRESHOLD_INCLUDE_H
#define _REL_THRESHOLD_INCLUDE_H

#include "bls_include.h"

int             E1_lagrange_interpolate_at_zero_write(byte*, const byte* , const uint8_t[], const int);
extern void     Fr_polynomial_image(Fr* out, E2* y, const Fr* a, const int a_size, const byte x);

#endif
