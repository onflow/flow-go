// +build relic

// this file is about the core functions required by the BLS signature scheme

#ifndef _REL_BLS_INCLUDE_H
#define _REL_BLS_INCLUDE_H

#include "relic.h"
#include "bls12381_utils.h"

// Signature, Public key and Private key lengths 
#define FULL_SIGNATURE_LEN  G1_BYTES
#define FULL_PK_LEN         G2_BYTES
#define SIGNATURE_LEN       (FULL_SIGNATURE_LEN/(G1_SERIALIZATION+1))
#define PK_LEN              (FULL_PK_LEN/(G2_SERIALIZATION+1))
#define SK_BITS             (Fr_BITS)
#define SK_LEN              BITS_TO_BYTES(SK_BITS)    

// Simultaneous Pairing in verification
#define DOUBLE_PAIRING 1
#define SINGLE_PAIRING (DOUBLE_PAIRING^1)

// Signature and public key membership check
#define MEMBERSHIP_CHECK 1

// algorithm choice for the hashing to G1 
// both methods are similar implementations of the same optimzed SSWU 
// but offer different timings.
#define RELIC_SSWU 1  // relic library implementation
#define LOCAL_SSWU 2       // local implementation
#define hashToPoint LOCAL_SSWU

// bls core (functions in bls_core.c)
int      get_signature_len();
int      get_pk_len();
int      get_sk_len();  

void     bls_sign(byte*, const bn_t, const byte*, const int);
int      bls_verify(const ep2_t, const byte*, const byte*, const int);
int      bls_verifyPerDistinctMessage(const byte*, const int, const byte*, const uint32_t*,
                         const uint32_t*, const ep2_st*);
int      bls_verifyPerDistinctKey(const byte*, 
                         const int, const ep2_st*, const uint32_t*,
                         const byte*, const uint32_t*);
void     bls_batchVerify(const int, byte*, const ep2_st*,
            const byte*, const byte*, const int);

#endif
