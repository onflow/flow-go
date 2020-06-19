// +build relic

// this file is about the core functions required by the BLS signature scheme

#ifndef _REL_BLS_INCLUDE_H
#define _REL_BLS_INCLUDE_H

#include "relic.h"
#include "bls12381_utils.h"

// Signature, Public key and Private key lengths 
#define FULL_SIGNATURE_LEN  G1_BYTES
#define FULL_PK_LEN         G2_BYTES
#define SIGNATURE_LEN       (FULL_SIGNATURE_LEN/(SERIALIZATION+1))
#define PK_LEN              (FULL_PK_LEN/(SERIALIZATION+1))
#define SK_BITS             (Fr_BITS)
#define SK_LEN              BITS_TO_BYTES(SK_BITS)    

// Simultaneous Pairing in verification
#define DOUBLE_PAIRING 1
#define SINGLE_PAIRING (DOUBLE_PAIRING^1)

// Signature membership check
#define MEMBERSHIP_CHECK 1

// algorithm choice for the hashing to G1 
#define HASHCHECK 1
#define SWU 2
#define OPSWU 3
#define hashToPoint OPSWU


// bls core
int      get_signature_len();
int      get_pk_len();
int      get_sk_len(); 

void     bls_sign(byte*, const bn_t, const byte*, const int);
int      bls_verify(const ep2_t, const byte*, const byte*, const int);

int      check_membership_Zr(const bn_t);
int      check_membership_G2(const ep2_t);

void     map_to_G1(ep_t, const byte*, const int);
void     opswu_test(uint8_t *, const uint8_t *, int);
#endif
