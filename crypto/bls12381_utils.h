// +build relic

// this file contains utility functions for the curve BLS 12-381
// these tools are shared by the BLS signature scheme, the BLS based threshold signature
// and the BLS distributed key generation protcols

#ifndef _REL_MISC_INCLUDE_H
#define _REL_MISC_INCLUDE_H

#include "relic.h"

typedef uint8_t byte;

#define VALID   RLC_OK
#define INVALID RLC_ERR

#define BITS_TO_BYTES(x) ((x+7)>>3)
#define BITS_TO_DIGITS(x) ((x+63)>>6)
#define BYTES_TO_DIGITS(x) ((x+7)>>3)
#define MIN(a,b) ((a)>(b)?(b):(a))

// Fields and Group serialization lengths
#define SEC_BITS  128
#define Fp_BITS   381
#define Fr_BITS   255
#define Fp_BYTES  BITS_TO_BYTES(Fp_BITS)
#define Fp_DIGITS BITS_TO_DIGITS(Fp_BITS)
#define Fr_BYTES  BITS_TO_BYTES(Fr_BITS)

#define G1_BYTES (2*Fp_BYTES)
#define G2_BYTES (4*Fp_BYTES)

// Compressed and uncompressed points
#define COMPRESSED      1
#define UNCOMPRESSED    0
#define SERIALIZATION   COMPRESSED

// Structure of precomputed data
#if (hashToPoint == OPSWU)
    #define ELLP_Nx_LEN 12
    #define ELLP_Dx_LEN 10
    #define ELLP_Ny_LEN 16
    #define ELLP_Dy_LEN 15
#endif

typedef struct prec_ {
    #if (hashToPoint == OPSWU)
    // coefficients of E1(Fp)
    fp_st a1;
    fp_st b1; 
    // coefficients of the isogeny map
    fp_st iso_Nx[ELLP_Nx_LEN];
    fp_st iso_Dx[ELLP_Dx_LEN];
    fp_st iso_Ny[ELLP_Ny_LEN];
    fp_st iso_Dy[ELLP_Dy_LEN];
    #endif
    bn_st p_3div4;
    fp_st p_1div2;
} prec_st;

// Utility functions
int      get_valid();
int      get_invalid();

ctx_t*   relic_init_BLS12_381();
prec_st* init_precomputed_data_BLS12_381();
void     precomputed_data_set(prec_st* p);
void     seed_relic(byte*, int);

int      ep_read_bin_compact(ep_st*, const byte *, const int);
void     ep_write_bin_compact(byte *, const ep_st *,  const int);
int      ep2_read_bin_compact(ep2_st* , const byte *,  const int);
void     ep2_write_bin_compact(byte *, const ep2_st *,  const int);

void     ep_mult_gen(ep_st*, const bn_st*);
void     ep_mult(ep_st*, const ep_st*, const bn_st*);
void     ep2_mult_gen(ep2_st*, const bn_st*);

void     bn_randZr(bn_t);
void     bn_randZr_star(bn_t);
void     bn_map_to_Zr_star(bn_st*, const uint8_t*, int);

// Debugging related functions
void     bytes_print_(char*, byte*, int);
void     fp_print_(char*, fp_t);
void     bn_print_(char*, bn_st*);
void     ep_print_(char*, ep_st*);
void     ep2_print_(char*, ep2_st*);

#endif