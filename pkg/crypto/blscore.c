
#include "bls_include.h"

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

int _getSignatureLengthBLS_BLS12381() {
    return SIGNATURE_LEN;
}

int _getPubKeyLengthBLS_BLS12381() {
    return PK_LEN;
}

int _getPrKeyLengthBLS_BLS12381() {
    return SK_LEN;
}

// Exponentiation of random p in G1
void _G1scalarPointMult(ep_st* res, ep_st* p, bn_st *expo) {
    // Using window NAF of size 2
    #if (EP_MUL	== LWNAF)
        g1_mul(res, p, expo);
    #else 
        ep_mul_lwnaf(res, p, expo);
    #endif
}

// Exponentiation of fixed g1 in G1
// This function is not called by BLS but is here for DEBUG/TESTs purposes
void _G1scalarGenMult(ep_st* res, bn_st *expo) {
#define GENERIC_POINT 0
#define FIXED_MULT    (GENERIC_POINT^1)

#if GENERIC_POINT
    _G1scalarPointMult(res, &core_get()->ep_g, expo);
#elif FIXED_MULT
    // Using precomputed table of size 4
    g1_mul_gen(res, expo);
#endif
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


// Computes BLS signature
void _blsSign(byte* s, bn_st *sk, byte* data, int len) {
    ep_st h;
    ep_new(&h);
    // hash to G1 (construction 2 in https://eprint.iacr.org/2019/403.pdf)
    ep_map(&h, data, len); 
    // s = p^sk
	_G1scalarPointMult(&h, &h, sk);  
    _ep_write_bin_compact(s, &h);
    ep_free(&p);
}

// Verifies the validity of a BLS signature
int _blsVerify(ep2_st *pk, byte* sig, byte* data, int len) { 
    // TODO : check pk is on curve  (should be done offline)
	// TODO : check pk is in G2 (should be done offline) 

    ep_t elemsG1[2];
    ep2_t elemsG2[2];

    // elemsG1[0] = s
    ep_new(elemsG1[0]);
    _ep_read_bin_compact(elemsG1[0], sig);

 #if MEMBERSHIP_CHECK
    // check s is on curve
    if (!ep_is_valid(elemsG1[0]))
        return SIG_INVALID;
    // check s is in G1
    ep_st inf;
    ep_new(&inf);
    // check s^order == infinity
    // use basic double & add as lwnaf reduces the expo modulo r
    // TODO : write a simple lwnaf without reduction
    ep_mul_basic(&inf, elemsG1[0], &core_get()->ep_r);
    if (!ep_is_infty(&inf)){
        ep_free(&inf);
        return SIG_INVALID;
    }
    ep_free(&inf);
 #endif

    // elemsG1[1] = h
    ep_new(elemsG1[1]);
    // hash to G1 (construction 2 in https://eprint.iacr.org/2019/403.pdf)
    ep_map(elemsG1[1], data, len); 

    // elemsG2[1] = pk
    ep2_new(elemsG2[1]);
    ep2_copy(elemsG2[1], pk);

#if DOUBLE_PAIRING  
    // elemsG2[0] = -g2
    ep2_new(&elemsG2[0]);
    ep2_neg(elemsG2[0], &core_get()->ep2_g); // could be hardcoded 

    fp12_t pair;
    fp12_new(&pair);
    // double pairing with Optimal Ate 
    pp_map_sim_oatep_k12(pair, (ep_t*)(elemsG1) , (ep2_t*)(elemsG2), 2);

    // compare the result to 1
    int res = fp12_cmp_dig(pair, 1);

#elif SINGLE_PAIRING   
    fp12_t pair1, pair2;
    fp12_new(&pair1); fp12_new(&pair2);
    pp_map_oatep_k12(pair1, elemsG1[0], &core_get()->ep2_g);
    pp_map_oatep_k12(pair2, elemsG1[1], elemsG2[1]);

    int res = fp12_cmp(pair1, pair2);
#endif
    fp12_free(&one);
    ep_free(elemsG1[0]);
    ep_free(elemsG1[1]);
    ep2_free(elemsG2[0]);
    ep2_free(elemsG2[1]);
    
    if (res == RLC_EQ && core_get()->code == RLC_OK) 
        return SIG_VALID;
    else 
        return SIG_INVALID;
}
