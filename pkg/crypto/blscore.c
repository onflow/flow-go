
#include "relic_include.h"

// Exponentiation of g1 in G1
void _G1scalarPointMult(ep_st* res, ep_st* p, bn_st* expo) {
    // TODO: try sliding window mult
    g1_mul(res, p, expo);
}

// Exponentiation of g1 in G1
void _G1scalarGenMult(ep_st* res, bn_st* expo) {
    // TODO: try sliding window mult
    g1_mul_gen(res, expo);
}

// Exponentiation of g2 in G2
void _G2scalarGenMult(ep2_st* res, bn_st* expo) {
    // TODO: try sliding window mult
    g2_mul_gen(res, expo);
}


