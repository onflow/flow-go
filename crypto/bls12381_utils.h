// +build relic

// this file contains utility functions for the curve BLS 12-381
// these tools are shared by the BLS signature scheme, the BLS based threshold signature
// and the BLS distributed key generation protocols

#ifndef _REL_MISC_INCLUDE_H
#define _REL_MISC_INCLUDE_H

#include "relic.h"
#include "blst_include.h"

#define VALID     RLC_OK
#define INVALID   RLC_ERR
#define UNDEFINED (((VALID&1)^1) | ((INVALID&2)^2)) // different value than RLC_OK and RLC_ERR

#define BITS_TO_BYTES(x) ((x+7)>>3)
#define BITS_TO_LIMBS(x) ((x+63)>>6)
#define BYTES_TO_LIMBS(x) ((x+7)>>3)
#define LIMBS_TO_BYTES(x) ((x)<<3)
#define MIN(a,b) ((a)>(b)?(b):(a))

// Fields and Group serialization lengths
#define SEC_BITS  128
#define Fp_BITS   381
#define Fp2_BYTES (2*Fp_BYTES)
#define Fp_LIMBS  BITS_TO_LIMBS(Fp_BITS)
#define Fp_BYTES  LIMBS_TO_BYTES(Fp_LIMBS) // BLST implements Fp as a limb array
#define Fr_BITS   255
#define Fr_LIMBS  BITS_TO_LIMBS(Fr_BITS)
#define Fr_BYTES  LIMBS_TO_BYTES(Fr_LIMBS) // BLST implements Fr as a limb array

#define G1_BYTES (2*Fp_BYTES)
#define G2_BYTES (2*Fp2_BYTES)

// Compressed and uncompressed points
#define COMPRESSED      1
#define UNCOMPRESSED    0
#define G1_SERIALIZATION    (COMPRESSED)
#define G2_SERIALIZATION    (COMPRESSED)
#define G1_SER_BYTES        (G1_BYTES/(G1_SERIALIZATION+1))
#define G2_SER_BYTES        (G2_BYTES/(G2_SERIALIZATION+1))

// Subgroup membership check method
#define EXP_ORDER 0
#define BOWE 1
#define MEMBERSHIP_CHECK_G1 BOWE
#define MEMBERSHIP_CHECK_G2 EXP_ORDER


// constants used in the optimized SWU hash to curve
#if (hashToPoint == LOCAL_SSWU)
    #define ELLP_Nx_LEN 12
    #define ELLP_Dx_LEN 10
    #define ELLP_Ny_LEN 16
    #define ELLP_Dy_LEN 15
#endif


// Structure of precomputed data
typedef struct prec_ {
    #if (hashToPoint == LOCAL_SSWU)
    // constants needed in optimized SSWU
    bn_st p_3div4;
    fp_st sqrt_z;
    // related hardcoded constants for faster access,
    // where a1 is the coefficient of isogenous curve E1
    fp_st minus_a1;
    fp_st a1z;
    // coefficients of the isogeny map
    fp_st iso_Nx[ELLP_Nx_LEN];
    fp_st iso_Ny[ELLP_Ny_LEN];
    #endif
    #if  (MEMBERSHIP_CHECK_G1 == BOWE)
    bn_st beta;
    bn_st z2_1_by3;
    #endif
    // other field-related constants
    bn_st p_1div2;
    fp_t r;   // Montgomery multiplication constant
} prec_st;

// TODO: to delete when Relic is removed
bn_st* Fr_blst_to_relic(const Fr* x);
Fr*  Fr_relic_to_blst(const bn_st* x);
ep2_st* E2_blst_to_relic(const E2* x);

int      get_valid();
int      get_invalid();
int      get_Fr_BYTES();

// BLS based SPoCK
int bls_spock_verify(const E2*, const byte*, const E2*, const byte*);

// hash to curve functions (functions in bls12381_hashtocurve.c)
void     map_to_G1(ep_t, const byte*, const int);

// Fr utilities
extern const limb_t BLS12_381_rR[Fr_LIMBS];
bool_t      Fr_is_zero(const Fr* a);
bool_t      Fr_is_equal(const Fr* a, const Fr* b);
void        Fr_set_limb(Fr*, const limb_t);
void        Fr_copy(Fr*, const Fr*);
void        Fr_set_zero(Fr*);
void        Fr_add(Fr *res, const Fr *a, const Fr *b);
void        Fr_sub(Fr *res, const Fr *a, const Fr *b);
void        Fr_neg(Fr *res, const Fr *a);
void        Fr_sum_vector(Fr*, const Fr x[], const int);
void        Fr_mul_montg(Fr *res, const Fr *a, const Fr *b);
void        Fr_squ_montg(Fr *res, const Fr *a);
void        Fr_to_montg(Fr *res, const Fr *a);
void        Fr_from_montg(Fr *res, const Fr *a);
void        Fr_exp_montg(Fr *res, const Fr* base, const limb_t* expo, const int expo_len);
void        Fr_inv_montg_eucl(Fr *res, const Fr *a);
void        Fr_inv_exp_montg(Fr *res, const Fr *a);
BLST_ERROR  Fr_read_bytes(Fr* a, const byte *bin, int len);
BLST_ERROR  Fr_star_read_bytes(Fr* a, const byte *bin, int len);
void        Fr_write_bytes(byte *bin, const Fr* a);
bool        map_bytes_to_Fr(Fr*, const byte*, int);

// Fp utilities

// E1 and G1 utilities
int      ep_read_bin_compact(ep_t, const byte *, const int);
void     ep_write_bin_compact(byte *, const ep_t,  const int);
void     ep_mult_gen_bench(ep_t, const Fr*);
void     ep_mult_generic_bench(ep_t, const Fr*);
void     ep_mult(ep_t, const ep_t, const Fr*);
void     ep_sum_vector(ep_t, ep_st*, const int);
int      ep_sum_vector_byte(byte*, const byte*, const int);
int      G1_check_membership(const ep_t);
int      G1_simple_subgroup_check(const ep_t);
void     ep_rand_G1(ep_t);
void     ep_rand_G1complement( ep_t);
#if  (MEMBERSHIP_CHECK_G1 == BOWE)
int      bowe_subgroup_check_G1(const ep_t);
#endif

// E2 and G2 utilities
void        E2_set_infty(E2* p);
bool_t      E2_is_infty(const E2*);
bool_t      E2_affine_on_curve(const E2*);
bool_t      E2_is_equal(const E2* p1, const E2* p2);
void        E2_copy(E2*, const E2*);
void        E2_to_affine(E2*, const E2*);
BLST_ERROR  E2_read_bytes(E2*, const byte *,  const int); 
void        E2_write_bytes(byte *, const E2*);
void        G2_mult_gen(E2*, const Fr*);
void        E2_mult(E2*, const E2*, const Fr*);
void        E2_mult_small_expo(E2*, const E2*, const byte);
void        E2_add(E2* res, const E2* a, const E2* b);
void        E2_sum_vector(E2*, const E2*, const int);

void     ep2_mult(ep2_t res, const ep2_t p, const Fr* expo); 

void     E2_subtract_vector(E2* res, const E2* x, const E2* y, const int len);
int      G2_check_membership(const E2*);
int      G2_simple_subgroup_check(const ep2_t);
void     ep2_rand_G2(ep2_t);
void     ep2_rand_G2complement( ep2_t);

// Utility functions
ctx_t*   relic_init_BLS12_381();
prec_st* init_precomputed_data_BLS12_381();
void     precomputed_data_set(const prec_st* p);
void     seed_relic(byte*, int);

// utility testing function
void xmd_sha256(byte *, int, byte *, int, byte *, int);

// Debugging related functions
void     bytes_print_(char*, byte*, int);
void     Fr_print_(char*, Fr*);
void     Fp_print_(char*, Fp*);
void     Fp2_print_(char*, const Fp2*);
void     E2_print_(char*, const E2*);
void     fp_print_(char*, fp_t);
void     bn_print_(char*, bn_st*);
void     ep_print_(char*, ep_st*);
void     ep2_print_(char*, ep2_st*);

#endif