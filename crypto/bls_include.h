// this file is about the core functions required by the BLS signature scheme

#ifndef _BLS_INCLUDE_H
#define _BLS_INCLUDE_H

#include "bls12381_utils.h"

// BLS signature core (functions in bls_core.c)
int bls_sign(byte *, const Fr *, const byte *, const int);
int bls_verify(const E2 *, const byte *, const byte *, const int);
int bls_verifyPerDistinctMessage(const byte *, const int, const byte *,
                                 const uint32_t *, const uint32_t *,
                                 const E2 *);
int bls_verifyPerDistinctKey(const byte *, const int, const E2 *,
                             const uint32_t *, const byte *, const uint32_t *);
void bls_batch_verify(const int, byte *, const E2 *, const byte *, const byte *,
                      const int, const byte *);

// BLS based SPoCK
int bls_spock_verify(const E2 *, const byte *, const E2 *, const byte *);

#endif
