// +build relic

#ifndef _REL_DKG_INCLUDE_H
#define _REL_DKG_INCLUDE_H

#include "bls12381_utils.h"

void        Fr_polynomial_image_export(byte* out, G2* y, const Fr* a, const int a_size, const byte x);
void        Fr_polynomial_image(Fr* out, G2* y, const Fr* a, const int a_size, const byte x);
void        G2_polynomial_images(G2* y, const int len_y, const G2* A, const int len_A);
void        G2_vector_write_bytes(byte* out, const G2* A, const int len);
BLST_ERROR  G2_vector_read_bytes(G2* A, const byte* src, const int len);
bool_t      verify_share(const Fr* x, const G2* y);

#endif
