// +build relic

// this file contains utility functions for the curve BLS 12-381
// these tools are shared by the BLS signature scheme, the BLS based threshold signature
// and the BLS distributed key generation protocols

#ifndef _REL_MISC_INCLUDE_H
#define _REL_MISC_INCLUDE_H

#include "relic.h"
#include "blst_include.h"

#define SEC_BITS  128
#define VALID     RLC_OK
#define INVALID   RLC_ERR
#define UNDEFINED (((VALID&1)^1) | ((INVALID&2)^2)) // different value than RLC_OK and RLC_ERR

#define BITS_TO_BYTES(x) ((x+7)>>3)
#define BITS_TO_LIMBS(x) ((x+63)>>6)
#define BYTES_TO_LIMBS(x) ((x+7)>>3)
#define LIMBS_TO_BYTES(x) ((x)<<3)
#define MIN(a,b) ((a)>(b)?(b):(a))

// Fields and Group serialization lengths
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


// TODO: to delete when Relic is removed
ep2_st*     E2_blst_to_relic(const E2* x);
ep_st*      E1_blst_to_relic(const E1* x);

int      get_valid();
int      get_invalid();
int      get_Fr_BYTES();
int      get_mapToG1_input_len();

// BLS based SPoCK
int bls_spock_verify(const E2*, const byte*, const E2*, const byte*);

// hash to curve functions (functions in bls12381_hashtocurve.c)
#define MAP_TO_G1_INPUT_LEN (2*(Fp_BYTES + SEC_BITS/8))
int     map_to_G1(E1*, const byte*, const int);

// Fr utilities
extern const Fr BLS12_381_rR;
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
bool_t      map_bytes_to_Fr(Fr*, const byte*, int);

// Fp utilities
void        Fp_mul_montg(Fp *, const Fp *, const Fp *);
void        Fp_squ_montg(Fp *, const Fp *);

// E1 and G1 utilities
void        E1_copy(E1*, const E1*);
bool_t      E1_is_equal(const E1*, const E1*);
void        E1_set_infty(E1*);
bool_t      E1_is_infty(const E1*);
void        E1_to_affine(E1*, const E1*);
bool_t      E1_affine_on_curve(const E1*);
bool_t      E1_in_G1(const E1*);
void        E1_mult(E1*, const E1*, const Fr*);
void        E1_add(E1*, const E1*, const E1*);
void        E1_sum_vector(E1*, const E1*, const int);
int         E1_sum_vector_byte(byte*, const byte*, const int);
void        G1_mult_gen(E1*, const Fr*);
BLST_ERROR  E1_read_bytes(E1*, const byte *,  const int); 
void        E1_write_bytes(byte *, const E1*);
void        unsecure_map_bytes_to_G1(E1*, const byte*, int);
BLST_ERROR  unsecure_map_bytes_to_G1complement(E1*, const byte*, int);

// E2 and G2 utilities
void        E2_set_infty(E2* p);
bool_t      E2_is_infty(const E2*);
bool_t      E2_affine_on_curve(const E2*);
bool_t      E2_is_equal(const E2*, const E2*);
void        E2_copy(E2*, const E2*);
void        E2_to_affine(E2*, const E2*);
BLST_ERROR  E2_read_bytes(E2*, const byte *,  const int); 
void        E2_write_bytes(byte *, const E2*);
void        G2_mult_gen(E2*, const Fr*);
void        E2_mult(E2*, const E2*, const Fr*);
void        E2_mult_small_expo(E2*, const E2*, const byte);
void        E2_add(E2* res, const E2* a, const E2* b);
void        E2_sum_vector(E2*, const E2*, const int);
void        E2_subtract_vector(E2* res, const E2* x, const E2* y, const int len);
bool_t      E2_in_G2(const E2*);
void        unsecure_map_bytes_to_G2(E2*, const byte*, int);
BLST_ERROR  unsecure_map_bytes_to_G2complement(E2*, const byte*, int);

// Utility functions
ctx_t*   relic_init_BLS12_381();

// utility testing function
void xmd_sha256(byte *, int, byte *, int, byte *, int);

// Debugging related functions
void     bytes_print_(char*, byte*, int);
void     Fr_print_(char*, Fr*);
void     Fp_print_(char*, const Fp*);
void     Fp2_print_(char*, const Fp2*);
void     E1_print_(char*, const E1*, const int);
void     E2_print_(char*, const E2*, const int);
void     fp_print_(char*, fp_t);
void     bn_print_(char*, bn_st*);
void     ep_print_(char*, ep_st*);
void     ep2_print_(char*, ep2_st*);

#endif