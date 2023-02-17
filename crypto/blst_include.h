// +build relic

#ifndef __BLST_INCLUDE_H__
#define __BLST_INCLUDE_H__

// blst related definitions
// eventually this file would replace blst.h

#include "blst.h" // TODO: should be deleted
#include "point.h"
#include "consts.h"

// field elements F_r
typedef struct {limb_t limbs[4];} Fr; // also used as vec256;
// Subroup G1 in E1
typedef POINTonE1 G1;
// Subroup G1 in E2
typedef POINTonE2 G2;

#endif