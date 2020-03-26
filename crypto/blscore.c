// +build relic

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

ctx_t bls_ctx;
prec_st bls_prec_st;
prec_st* bls_prec = NULL;

#if (hashToPoint == OPSWU)
extern const uint64_t a1_data[6];
extern const uint64_t b1_data[6];
extern const uint64_t iso_Nx_data[ELLP_Nx_LEN][6];
extern const uint64_t iso_Dx_data[ELLP_Dx_LEN][6];
extern const uint64_t iso_Ny_data[ELLP_Ny_LEN][6];
extern const uint64_t iso_Dy_data[ELLP_Dy_LEN][6];
#endif

void precomputed_data_set(prec_st* p) {
    bls_prec = p;
}

prec_st* init_precomputed_data_BLS12_381() {
    bls_prec = &bls_prec_st;
    #if (hashToPoint == OPSWU)
        fp_read_raw(bls_prec->a1, a1_data, 6);
        fp_read_raw(bls_prec->b1, b1_data, 6);
        for (int i=0; i<ELLP_Dx_LEN; i++)  
            fp_read_raw(bls_prec->iso_Dx[i], iso_Dx_data[i], 6);
        for (int i=0; i<ELLP_Nx_LEN; i++)  
            fp_read_raw(bls_prec->iso_Nx[i], iso_Nx_data[i], 6);
        for (int i=0; i<ELLP_Dy_LEN; i++)  
            fp_read_raw(bls_prec->iso_Dy[i], iso_Dy_data[i], 6);
        for (int i=0; i<ELLP_Ny_LEN; i++)  
            fp_read_raw(bls_prec->iso_Ny[i], iso_Ny_data[i], 6);
    #endif
    // (p-3)/4
    bn_read_raw(&bls_prec->p_3div4, fp_prime_get(), Fp_DIGITS);
    bn_sub_dig(&bls_prec->p_3div4, &bls_prec->p_3div4, 3);
    bn_rsh(&bls_prec->p_3div4, &bls_prec->p_3div4, 2);
    // (p-1)/2
    fp_sub_dig(bls_prec->p_1div2, fp_prime_get(), 1);
    fp_rsh(bls_prec->p_1div2, bls_prec->p_1div2, 1);
    return bls_prec;
}

// Initializes Relic context with BLS12-381 parameters
ctx_t* relic_init_BLS12_381() { 
    // check Relic was compiled with the right conf 
    if (ALLOC != AUTO) return NULL;

    // initialize relic core with a new context
    core_set(&bls_ctx);
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
void _G1scalarPointMult(ep_st* res, const ep_st* p, const bn_st *expo) {
    // Using window NAF of size 2
    #if (EP_MUL	== LWNAF)
        g1_mul(res, p, expo);
    #else 
        ep_mul_lwnaf(res, p, expo);
    #endif
}

// Exponentiation of fixed g1 in G1
// This function is not called by BLS but is here for DEBUG/TESTs purposes
void _G1scalarGenMult(ep_st* res, const bn_st *expo) {
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
void _G2scalarGenMult(ep2_st* res, const bn_st *expo) {
    // Using precomputed table of size 4
    g2_mul_gen(res, (bn_st*)expo);
}

// checks an input scalar is less than the groups order (r)
int checkMembership_Zr(const bn_st* a){
    bn_t r;
    bn_new(r); 
    g2_get_ord(r);
    int res = bn_cmp(a,r);
    if (res == RLC_LT) return VALID;
    return INVALID;
}


// checks if input point s is on the curve E1 
// and is in the subgroup G1
static int checkMembership_G1(const ep_t p){
    // check p is on curve
    if (!ep_is_valid(p))
        return INVALID;
    // check p is in G1
    ep_st inf;
    ep_new(&inf);
    // check p^order == infinity
    // use basic double & add as lwnaf reduces the expo modulo r
    // TODO : write a simple lwnaf without reduction
    ep_mul_basic(&inf, p, &core_get()->ep_r);
    if (!ep_is_infty(&inf)){
        ep_free(&inf);
        return INVALID;
    }
    ep_free(&inf);
    return VALID;
}

// checks if input point s is on the curve E2 
// and is in the subgroup G2
int checkMembership_G2(const ep2_st* p){
#if MEMBERSHIP_CHECK
    // check p is on curve
    if (!ep2_is_valid((ep2_st*)p))
        return INVALID;
    // check p is in G2
    ep2_st inf;
    ep2_new(&inf);
    // check p^order == infinity
    // use basic double & add as lwnaf reduces the expo modulo r
    // TODO : write a simple lwnaf without reduction
    ep2_mul_basic(&inf, (ep2_st*)p, &core_get()->ep_r);
    if (!ep2_is_infty(&inf)){
        ep2_free(&inf);
        return INVALID;
    }
    ep2_free(&inf);
#endif
    return VALID;
}


// Computes BLS signature
void _blsSign(byte* s, const bn_st *sk, const byte* data, const int len) {
    ep_st h;
    ep_new(&h);
    // hash to G1
    mapToG1(&h, data, len);
    // s = p^sk
	_G1scalarPointMult(&h, &h, sk);  
    _ep_write_bin_compact(s, &h, SIGNATURE_LEN);
    ep_free(&p);
}

// Verifies the validity of a BLS signature
int _blsVerify(const ep2_st *pk, const byte* sig, const byte* data, const int len) { 
    // TODO : check pk is on curve  (should be done offline)
	// TODO : check pk is in G2 (should be done offline) 
    ep_t elemsG1[2];
    ep2_t elemsG2[2];

    // elemsG1[0] = s
    ep_new(elemsG1[0]);
    if (ep_read_bin_compact(elemsG1[0], sig, SIGNATURE_LEN) != RLC_OK) 
        return INVALID;

 #if MEMBERSHIP_CHECK
    // check s is on curve and in G1
    if (checkMembership_G1(elemsG1[0]) != VALID)
        return INVALID;
 #endif

    // elemsG1[1] = h
    ep_new(elemsG1[1]);
    // hash to G1 
    mapToG1(elemsG1[1], data, len); 

    // elemsG2[1] = pk
    ep2_new(elemsG2[1]);
    ep2_copy(elemsG2[1], (ep2_st*)pk);

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
        return VALID;
    else 
        return INVALID;
}
