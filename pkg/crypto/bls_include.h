#ifndef _REL_BLS_INCLUDE_H
#define _REL_BLS_INCLUDE_H

#include "relic.h"
#include "misc.h"

// Fp length

#define Fp_BITS  381
#define Fr_BITS  256  

#define Fp_BYTES BITS_TO_BYTES(Fp_BITS)
#define Fr_BYTES BITS_TO_BYTES(Fr_BITS)

#define G1_BYTES (2*Fp_BYTES)
#define G2_BYTES (4*Fp_BYTES)

// Signature, Public key and Private key lengths 
#define FULL_SIGNATURE_LEN  G1_BYTES
#define FULL_PK_LEN         G2_BYTES
#define SIGNATURE_LEN       (FULL_SIGNATURE_LEN/(SERIALIZATION+1))
#define PK_LEN              (FULL_PK_LEN/(SERIALIZATION+1))
#define SK_BITS             (Fr_BITS)
#define SK_LEN              BITS_TO_BYTES(SK_BITS)    

#define SIG_VALID   1
#define SIG_INVALID 0
#define SIG_ERROR   0xFF

// Compressed and uncompressed points
#define COMPRESSED      1
#define UNCOMPRESSED    0
#define SERIALIZATION   COMPRESSED

// Simultaneous Pairing 
#define DOUBLE_PAIRING 1
#define SINGLE_PAIRING (DOUBLE_PAIRING^1)

// Signature membership check
#define MEMBERSHIP_CHECK 0

// Most of the functions are written for ALLOC=AUTO not ALLOC=DYNAMIC

// bls core
ctx_t*  _relic_init_BLS12_381();
int     _getSignatureLengthBLS_BLS12381();
int     _getPubKeyLengthBLS_BLS12381();
int     _getPrKeyLengthBLS_BLS12381(); 
void    _G1scalarPointMult(ep_st*, ep_st*, bn_st*);
void    _G2scalarGenMult(ep2_st*, bn_st*);
void    _blsSign(byte*, bn_st*, byte*, int);
int     _blsVerify(ep2_st *, byte*, byte*, int);
void    _ep_read_bin_compact(ep_st*, byte *);
void    _ep_write_bin_compact(byte *, const ep_st *);

#endif
