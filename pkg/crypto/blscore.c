
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


// Computes BLS signature
void _blsSign(byte* s, bn_st *sk, byte* data, int len) {
    
    ep_st h;
    ep_new(&h);
    // hash to G1 (construction 2 in https://eprint.iacr.org/2019/403.pdf)
    ep_map(&h, data, len); 
    // s = p^sk
    //_G1scalarGenMult(&h, sk);
	_G1scalarPointMult(&h, &h, sk);
    ep_write_bin_compact(s, &h);

    ep_free(&p);
}

// Verifies the validity of a BLS signature
int _blsVerify(ep2_st *pk, byte* sig, byte* data, int len) {

    // TODO : check s is on curve
	// TODO : check s is in G1
	// TODO : check pk is on curve  (can be done offline)
	// TODO : check pk is in G2 (can be done offline)
    ep_st h;
    ep_new(&h);
    // hash to G1 (construction 2 in https://eprint.iacr.org/2019/403.pdf)
    ep_map(&h, data, len); 

    ep_st s;
    ep_new(s);
    ep_read_bin_compact(&s, sig);

    //_ep_print("H", &h);
    _ep_print("S", &s);

#if 0
    ep2_st neg_g2;
    ep2_new(&neg_g2);
    ep2_neg(&neg_g2, &core_get()->ep2_g); // could be hardcoded

    ep_st** elemsG1 = (ep_st**) malloc (2*sizeof(ep_st));
    ep2_st** elemsG2 = (ep2_st**) malloc (2*sizeof(ep2_st));
    if (!elemsG1 || !elemsG2) {
        THROW(ERR_NO_MEMORY);
        return SIG_ERROR;
    }

    elemsG1[0] = &s;   
    elemsG2[0] = &neg_g2;  
    elemsG1[1] = &h;
    elemsG2[1] = pk;

    fp12_t pair;
    fp12_new(&pair);
    // double pairing with Optimal Ate 
    pp_map_sim_oatep_k12(pair, (ep_t*)(elemsG1) , (ep2_t*)(elemsG2), 2);

    // compare the result to 1
    int res = fp12_cmp_dig(pair, 1);

    free(elemsG1);
    free(elemsG2);
#else
    fp12_t pair1, pair2;
    fp12_new(&pair1); fp12_new(&pair2);
    pp_map_oatep_k12(pair1, &s, &core_get()->ep2_g);
    pp_map_oatep_k12(pair2, &h, pk);

    int res = fp12_cmp(pair1, pair2);
#endif

    ep_free(&p);
    ep_free(&s);
    fp12_free(&one)
    
    if (res == RLC_EQ && core_get()->code == RLC_OK) 
        return SIG_VALID;
    else 
        return SIG_INVALID;
}


