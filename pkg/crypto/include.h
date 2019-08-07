#ifndef _REL_INCLUDE_H
#define _REL_INCLUDE_H

#include "relic_ep.h"
#include "relic_bn.h"
#include "relic_fp.h"
#include "relic_core.h"
#include "relic_pc.h"

// Most of the functions are written for ALLOC=AUTO not ALLOC=DYNAMIC

// macro wrapper functions
/* void _bn_new(bn_t);                       
void _bn_free(bn_t);
void _ep_new(ep_t);
void _ep_free(ep_t); */

// Debug related functions
void _fp_print(char*, fp_st*);
void _bn_print(char*, bn_st*);
void _ep_print(char*, ep_st*);
void _ep2_print(char*, ep2_st*);
void _bn_randZr(bn_t, char*);

// bls core
ctx_t* _relic_init_BLS12_381();
void _G1scalarPointMult(ep_st*, ep_st*, bn_st*);
void _G1scalarGenMult(ep_st*, bn_st*);
void _G2scalarGenMult(ep2_st*, bn_st*);


#endif