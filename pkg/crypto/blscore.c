
#include "include.h"

#define DOUBADD 0 
#define SLIDW   1 
#define MONTG   2 
#define PRECOM  3 
#define WREG    4 
#define WNAF    5 
#define RELIC   6 

#define G1MULT RELIC
#define G2MULT RELIC

// Initializes Relic context with BLS12-381 parameters
ctx_t* _relic_init_BLS12_381() { 
    // check Relic was compiled with the right conf 
    if (ALLOC != AUTO) return NULL;

    // init relic core
	if (core_init() != RLC_OK) return NULL;

    // init BLS curve
    int ret = RLC_OK;
    #if (FP_PRIME == 381)
    ret = ep_param_set_any_pairf(); // sets B12_P381 if FP_PRIME = 381 in relic config
    #else
    ep_param_set(B12_P381);
    ep2_curve_set_twist(EP_MTYPE);  // Multiplicative twist 
    #endif 
    
    if (ret != RLC_OK) return NULL;
    return core_get();
}

// Exponentiation of random p in G1
void _G1scalarPointMult(ep_st* res, ep_st* p, bn_st *expo) {
    // Using window NAF of size 2
    #if (EP_MUL	== LWNAF)
        g1_mul(res, p, expo);
    #else 
        ep1_mul_lwnaf(res, p, expo);
    #endif
}

// Exponentiation of fixed g1 in G1
// This function is not called by BLS but is here for DEBUG/TESTs purposes
void _G1scalarGenMult(ep_st* res, bn_st *expo) {
    // Using precomputed table of size 4
    g1_mul_gen(res, expo);
}

// Exponentiation of random p in G2
void _G2scalarPointMult(ep2_st* res, ep2_st* p, bn_st *expo) {
    // Using window NAF of size 2
    #if (EP_MUL	== LWNAF)
        g2_mul(res, p, expo);
    #else 
        ep2_mul_lwnaf(res, p, expo);
    #endif
    
}

// Exponentiation of fixed g2 in G2
void _G2scalarGenMult(ep2_st* res, bn_st *expo) {
    // Using precomputed table of size 4
    g2_mul_gen(res, expo);
}


