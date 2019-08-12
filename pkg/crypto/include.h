#ifndef _REL_INCLUDE_H
#define _REL_INCLUDE_H

#include "relic_ep.h"
#include "relic_bn.h"
#include "relic_fp.h"
#include "relic_core.h"
#include "relic_pc.h"

// constants
#define BITS_TO_BYTES(x) ((x+7)/8)
#define FP_BITS  381
#define FP_BYTES BITS_TO_BYTES(FP_BITS)

#define COMPRESSED      1
#define UNCOMPRESSED    0
#define SERIALIZATION   COMPRESSED

#define FULL_SIGNATURE_LEN  (2*FP_BYTES)
#define FULL_PK_LEN         (4*FP_BYTES)
#define SIGNATURE_LEN       (FULL_SIGNATURE_LEN/(SERIALIZATION+1))
#define PK_LEN              (FULL_PK_LEN/(SERIALIZATION+1))
#define SK_BITS             256
#define SK_LEN              BITS_TO_BYTES(SK_BITS)    

#define SIG_VALID   1
#define SIG_INVALID 0
#define SIG_ERROR   0xFF

typedef uint8_t byte;

// Most of the functions are written for ALLOC=AUTO not ALLOC=DYNAMIC

// Debug related functions
void _bytes_print(char*, byte*, int);
void _fp_print(char*, fp_st*);
void _bn_print(char*, bn_st*);
void _ep_print(char*, ep_st*);
void _ep2_print(char*, ep2_st*);
void _bn_randZr(bn_t, byte*);
void _G1scalarGenMult(ep_st*, bn_st*);
ep_st* _hashToG1(byte* data, int len);


// bls core
ctx_t* _relic_init_BLS12_381();
int _getSignatureLengthBLS_BLS12381();
int _getPubKeyLengthBLS_BLS12381();
int _getPrKeyLengthBLS_BLS12381(); 
void _G1scalarPointMult(ep_st*, ep_st*, bn_st*);
void _G2scalarGenMult(ep2_st*, bn_st*);
void _blsSign(byte*, bn_st*, byte*, int);
int  _blsVerify(ep2_st *, byte*, byte*, int);
void ep_read_bin_compact(ep_st*, byte *);
void ep_write_bin_compact(byte *, const ep_st *);

#endif