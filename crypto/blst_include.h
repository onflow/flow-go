// +build relic

#ifndef __BLST_INCLUDE_H__
#define __BLST_INCLUDE_H__

// extra tools to use BLST low level that are needed by the Flow crypto library
// eventually this file would replace blst.h

#include "blst.h" // TODO: should be deleted
#include "point.h"
#include "consts.h"

// types used by the Flow crypto library that are imported from BLST
// these type definitions are used as an abstraction from BLST internal types

// field elements F_r
typedef struct {limb_t limbs[4];} Fr; // also used as vec256 (little endian limbs)
// Subroup G1 in E1
typedef POINTonE1 G1;
// Subroup G1 in E2
typedef POINTonE2 G2;


// extra functions and tools that are needed by the Flow crypto library 
// and that are not exported in the desired form by BLST

void pow256_from_be_bytes(pow256 ret, const unsigned char a[32]);
void vec256_from_be_bytes(vec256 out, const unsigned char *bytes, size_t n);


#endif