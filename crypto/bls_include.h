// +build relic

#ifndef _REL_BLS_INCLUDE_H
#define _REL_BLS_INCLUDE_H

#include "relic.h"
#include "misc.h"

// Fp length
#define Fp_BITS   381
#define Fr_BITS   255
#define Fp_BYTES  BITS_TO_BYTES(Fp_BITS)
#define Fp_DIGITS BITS_TO_DIGITS(Fp_BITS)
#define Fr_BYTES  BITS_TO_BYTES(Fr_BITS)

#define G1_BYTES (2*Fp_BYTES)
#define G2_BYTES (4*Fp_BYTES)

// Signature, Public key and Private key lengths 
#define FULL_SIGNATURE_LEN  G1_BYTES
#define FULL_PK_LEN         G2_BYTES
#define SIGNATURE_LEN       (FULL_SIGNATURE_LEN/(SERIALIZATION+1))
#define PK_LEN              (FULL_PK_LEN/(SERIALIZATION+1))
#define SK_BITS             (Fr_BITS)
#define SK_LEN              BITS_TO_BYTES(SK_BITS)    

// Compressed and uncompressed points
#define COMPRESSED      1
#define UNCOMPRESSED    0
#define SERIALIZATION   COMPRESSED

// Simultaneous Pairing 
#define DOUBLE_PAIRING 1
#define SINGLE_PAIRING (DOUBLE_PAIRING^1)

// Signature membership check
#define MEMBERSHIP_CHECK 1

// algorithm choice for the hashing to G1 
#define HASHCHECK 1
#define SWU 2
#define OPSWU 3
#define hashToPoint OPSWU

#if (hashToPoint == OPSWU)
#define ELLP_Nx_LEN 12
#define ELLP_Dx_LEN 10
#define ELLP_Ny_LEN 16
#define ELLP_Dy_LEN 15
#endif

// Structure of precomputed data
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

// Most of the functions are written for ALLOC=AUTO not ALLOC=DYNAMIC

// bls core
ctx_t*   relic_init_BLS12_381();
prec_st* init_precomputed_data_BLS12_381();
void     precomputed_data_set(prec_st* p);
int      _getSignatureLengthBLS_BLS12381();
int      _getPubKeyLengthBLS_BLS12381();
int      _getPrKeyLengthBLS_BLS12381(); 
void     _G1scalarPointMult(ep_st*, const ep_st*, const bn_st*);
void     _G2scalarGenMult(ep2_st*, const bn_st*);
void     _blsSign(byte*, const bn_st*, const byte*, const int);
int      _blsVerify(const ep2_st *, const byte*, const byte*, const int);
void     _ep_read_bin_compact(ep_st*, const byte *, const int);
void     _ep_write_bin_compact(byte *, const ep_st *,  const int);
void     _ep2_read_bin_compact(ep2_st* , const byte *,  const int);
void     _ep2_write_bin_compact(byte *, const ep2_st *,  const int);
void     mapToG1(ep_st* h, const byte* data, const int len);
int      checkMembership_Zr(const bn_st* a);
int      checkMembership_G2(const ep2_t p);

#endif
