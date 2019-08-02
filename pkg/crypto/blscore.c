
#include "relic_include.h"

#define DOUBADD 0 
#define SLIDW   1 
#define MONTG   2 
#define PRECOM  3 
#define WREG    4 
#define WNAF    5 
#define RELIC   6 

#define G1MULT RELIC
#define G2MULT RELIC

// Exponentiation of g1 in G1
void _G1scalarPointMult(ep_st* res, ep_st* p, bn_st* expo) {
    g1_mul(res, p, expo);
}

// Exponentiation of g1 in G1
void _G1scalarGenMult(ep_st* res, bn_st* expo) {
    ep_st g1;
    ep_curve_get_gen(&g1);
    
    #if (G1MULT==DOUBADD)
    ep_mul_basic(res, &g1, expo);
    #elif (G1MULT==SLIDW)
    ep_mul_slide(res, &g1, expo);
    #elif (G1MULT==MONTG)
    ep_mul_monty(res, &g1, expo);
    #elif (G1MULT==PRECOM)
    ep_mul_fix(res, &g1, expo);
    #elif (G1MULT==WREG)
    ep_mul_lwreg(res, &g1, expo);
    #elif (G1MULT==WNAF)
    ep_mul_lwnaf(res, &g1, expo);
    #elif (G1MULT==RELIC)
    ep_mul(res, &g1, expo);
    #endif
}

// Exponentiation of g2 in G2
void _G2scalarGenMult(ep2_st* res, bn_st* expo) {
    ep2_st g2;
    ep2_curve_get_gen(&g2);

    #if (G2MULT==DOUBADD)
    ep2_mul_basic(res, &g2, expo);
    #elif (G2MULT==SLIDW)
    ep2_mul_slide(res, &g2, expo);
    #elif (G2MULT==MONTG)
    ep2_mul_monty(res, &g2, expo);
    #elif (G2MULT==WNAF)
    ep2_mul_lwnaf(res, &g2, expo);
    #elif (G2MULT==RELIC)
    g2_mul_gen(res, expo);
    #endif
}


