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
void    _G1scalarPointMult(ep_st*, const ep_st*, const bn_st*);
void    _G2scalarGenMult(ep2_st*, const bn_st*);
void    _blsSign(byte*, const bn_st*, const byte*, const int);
int     _blsVerify(const ep2_st *, const byte*, const byte*, const int);
void    _ep_read_bin_compact(ep_st*, const byte *, const int);
void    _ep_write_bin_compact(byte *, const ep_st *,  const int);
void    _ep2_read_bin_compact(ep2_st* , const byte *,  const int);
void    _ep2_write_bin_compact(byte *, const ep2_st *,  const int);
void    mapToG1_swu(ep_t, const uint8_t *, const int);

#endif
