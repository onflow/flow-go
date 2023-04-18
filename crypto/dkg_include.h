// +build relic

#ifndef _REL_DKG_INCLUDE_H
#define _REL_DKG_INCLUDE_H

#include "bls12381_utils.h"

void        Fr_generate_polynomial(Fr* a, const int degree, const byte* seed, const int seed_len);
void        Fr_polynomial_image_write(byte* out, E2* y, const Fr* a, const int a_size, const byte x);
void        Fr_polynomial_image(Fr* out, E2* y, const Fr* a, const int a_size, const byte x);
void        E2_polynomial_images(E2* y, const int len_y, const E2* A, const int len_A);
void        G2_vector_write_bytes(byte* out, const E2* A, const int len);
BLST_ERROR  E2_vector_read_bytes(E2* A, const byte* src, const int len);
bool_t      G2_check_log(const Fr* x, const E2* y);

#endif
