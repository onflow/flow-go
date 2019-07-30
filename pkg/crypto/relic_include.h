#ifndef _REL_INCLUDE_H
#define _REL_INCLUDE_H

#include "relic_ep.h"
#include "relic_bn.h"
#include "relic_fp.h"
#include "relic_core.h"

// Most of the functions are written for ALLOC=AUTO not ALLOC=DYNAMIC

// macro wrapper functions
void _bn_new(bn_t);                       
void _bn_free(bn_t);
void _ep_new(ep_t);
void _ep_free(ep_t);

// Debug related functions
void _fp_print(fp_st*);
void _bn_print(bn_st*);

// G1 



#endif