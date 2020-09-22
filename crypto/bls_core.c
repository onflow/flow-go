// +build relic

#include "bls_include.h"

// this file is about the core functions required by the BLS signature scheme

// The functions are tested for ALLOC=AUTO (not for ALLOC=DYNAMIC)

// functions to export macros to the Go layer (because cgo does not import macros)
int get_signature_len() {
    return SIGNATURE_LEN;
}

int get_pk_len() {
    return PK_LEN;
}

int get_sk_len() {
    return SK_LEN;
}

// checks an input scalar is less than the groups order (r)
int check_membership_Zr(const bn_t a){
    bn_t r;
    bn_new(r); 
    g2_get_ord(r);
    int res = bn_cmp(a,r);
    if (res == RLC_LT) return VALID;
    return INVALID;
}

// checks if input point s is on the curve E1 
// and is in the subgroup G1
// membership check in G1 is using a naive scalar multiplication by the group order 
static int check_membership_G1(const ep_t p){
#if MEMBERSHIP_CHECK
    // check p is on curve
    if (!ep_is_valid(p))
        return INVALID;
    // check p is in G1
    #if MEMBERSHIP_CHECK_G1 == EXP_ORDER
    return simple_subgroup_check_G1(p);
    #elif MEMBERSHIP_CHECK_G1 == BOWE
    // section 3.2 from https://eprint.iacr.org/2019/814.pdf
    return bowe_subgroup_check_G1(p);
    #else
    return INVALID;
    #endif
#endif
    return VALID;
}

// checks if input point s is on the curve E2 
// and is in the subgroup G2
// membership check in G2 is using a naive scalar multiplication by the group order
// TODO: switch to the faster Bowe check 
int check_membership_G2(const ep2_t p){
#if MEMBERSHIP_CHECK
    // check p is on curve
    if (!ep2_is_valid((ep2_st*)p))
        return INVALID;
    // check p is in G2
    #if MEMBERSHIP_CHECK_G2 == EXP_ORDER
    return simple_subgroup_check_G2(p);
    #elif MEMBERSHIP_CHECK_G2 == BOWE
    // TODO: implement Bowe's check
    return INVALID;
    #else
    return INVALID;
    #endif
#endif
    return VALID;
}

// Computes a BLS signature
void bls_sign(byte* s, const bn_t sk, const byte* data, const int len) {
    ep_t h;
    ep_new(h);
    // hash to G1
    map_to_G1(h, data, len);
    // s = h^sk
	ep_mult(h, h, sk);  
    ep_write_bin_compact(s, h, SIGNATURE_LEN);
    ep_free(h);
}

// Verifies the validity of a BLS signature
// membership check of the signature in G1 is verified in this function
// membership check of pk in G2 is not verified in this function
// the membership check is separated to allow optimizing multiple verifications using the same pk
int bls_verify(const ep2_t pk, const byte* sig, const byte* data, const int len) {  
    ep_t elemsG1[2];
    ep2_t elemsG2[2];

    // elemsG1[0] = s
    ep_new(elemsG1[0]);
    if (ep_read_bin_compact(elemsG1[0], sig, SIGNATURE_LEN) != RLC_OK) 
        return INVALID;

    // check s is on curve and in G1
    if (check_membership_G1(elemsG1[0]) != VALID) // only enabled if MEMBERSHIP_CHECK==1
        return INVALID;

    // elemsG1[1] = h
    ep_new(elemsG1[1]);
    // hash to G1 
    map_to_G1(elemsG1[1], data, len); 

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
