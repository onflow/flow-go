// +build relic

// this file contains utility functions for the curve BLS 12-381
// these tools are shared by the BLS signature scheme, the BLS based threshold signature
// and the BLS distributed key generation protcols

#ifndef _REL_MISC_INCLUDE_H
#define _REL_MISC_INCLUDE_H

#include "relic.h"

typedef uint8_t byte;

#define VALID     RLC_OK
#define INVALID   RLC_ERR
#define UNDEFINED ((VALID^1) | (INVALID^2))

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

// Subgroup membership check method
#define EXP_ORDER 0
#define BOWE 1
#define MEMBERSHIP_CHECK_G1 BOWE
#define MEMBERSHIP_CHECK_G2 EXP_ORDER

// Structure of precomputed data
#if (hashToPoint == OPSWU)
    #define ELLP_Nx_LEN 12
    #define ELLP_Dx_LEN 10
    #define ELLP_Ny_LEN 16
    #define ELLP_Dy_LEN 15
#endif

typedef struct prec_ {
    #if (hashToPoint == OPSWU)
    bn_st p_3div4;
    fp_st fp_p_1div2; 
    // coefficients of E1(Fp)
    fp_st a1;
    fp_st b1; 
    // coefficients of the isogeny map
    fp_st iso_Nx[ELLP_Nx_LEN];
    fp_st iso_Dx[ELLP_Dx_LEN];
    fp_st iso_Ny[ELLP_Ny_LEN];
    fp_st iso_Dy[ELLP_Dy_LEN];
    #endif
    #if  (MEMBERSHIP_CHECK_G1 == BOWE)
    bn_st beta;
    bn_st z2_1_by3;
    #endif
    bn_st p_1div2;
} prec_st;

// Utility functions
int      get_valid();
int      get_invalid();

ctx_t*   relic_init_BLS12_381();
prec_st* init_precomputed_data_BLS12_381();
void     precomputed_data_set(prec_st* p);
void     seed_relic(byte*, int);

int      ep_read_bin_compact(ep_t, const byte *, const int);
void     ep_write_bin_compact(byte *, const ep_t,  const int);
int      ep2_read_bin_compact(ep2_t, const byte *,  const int);
void     ep2_write_bin_compact(byte *, const ep2_t,  const int);

void     ep_mult_gen(ep_t, const bn_t);
void     ep_mult(ep_t, const ep_t, const bn_t);
void     ep2_mult_gen(ep2_t, const bn_t);

void     bn_randZr(bn_t);
void     bn_randZr_star(bn_t);
void     bn_map_to_Zr_star(bn_t, const uint8_t*, int);

void     bn_sum_vector(bn_t, const bn_st*, const int);
void     ep_sum_vector(ep_t, ep_st*, const int);
void     ep2_sum_vector(ep2_t, ep2_st*, const int);
int      ep_sum_vector_byte(byte*, const byte*, const int);
void     ep2_subtract_vector(ep2_t res, ep2_t x, ep2_st* y, const int len);

int simple_subgroup_check_G1(const ep_t);
int simple_subgroup_check_G2(const ep2_t);
void ep_rand_G1(ep_t);
void ep_rand_G1complement( ep_t);
#if  (MEMBERSHIP_CHECK_G1 == BOWE)
int bowe_subgroup_check_G1(const ep_t);
#endif
int subgroup_check_G1_test(int, int);
int subgroup_check_G1_bench();

// Debugging related functions
void     bytes_print_(char*, byte*, int);
void     fp_print_(char*, fp_t);
void     bn_print_(char*, bn_st*);
void     ep_print_(char*, ep_st*);
void     ep2_print_(char*, ep2_st*);

// BLS based SPoCK
int bls_spock_verify(const ep2_t, const byte*, const ep2_t, const byte*);

#endif