// +build relic

#include "bls_thresholdsign_include.h"

// the highest index of a threshold participant
#define MAX_IND         255
#define MAX_IND_BITS    8   // equal to ceiling(log_2(MAX_IND))

// Computes the Lagrange coefficient L_i(0) in Fr with regards to the range [indices(0)..indices(t)]
// and stores it in `res`, where t is the degree of the polynomial P.
// `len` is equal to `t+1` where `t` is the polynomial degree.
static void Fr_lagrangeCoefficientAtZero(Fr* res, const int i, const uint8_t indices[], const int len){

    // coefficient is computed as N * D^(-1)
    Fr numerator;  // eventually would represent N*R^k  
    Fr denominator; // eventually would represent D*R^k 

    // Initialize N and D to Montgomery constant R
    Fr_copy(&numerator, (Fr*)BLS12_381_rR);
    Fr_copy(&denominator, (Fr*)BLS12_381_rR);

    // sign of D: 0 for positive and 1 for negative
    int sign = 0; 

    // the highest k such that fact(MAX_IND)/fact(MAX_IND-k) < 2^64 (approximately 64/MAX_IND_BITS)
    // this means we can multiply up to (k) indices in a limb (64 bits) without overflowing.
    #define MAX_IND_LOOPS   64/MAX_IND_BITS
    const int loops = MAX_IND_LOOPS;
    int k,j = 0;
    Fr tmp;
    while (j<len) {
        limb_t limb_numerator = 1;
        limb_t limb_denominator = 1;
        for (k = j; j < MIN(len, k+loops); j++){ // batch up to `loops` elements in one limb
            if (j==i) 
                continue;
            if (indices[j] < indices[i]) {
                sign ^= 1;
                limb_denominator *= indices[i]-indices[j];
            } else {
                limb_denominator *= indices[j]-indices[i];
            }
            limb_numerator *= indices[j];
        }
        // numerator and denominator are both computed in Montgomery form.
        // update numerator
        Fr_set_limb(&tmp, limb_numerator); // L_N
        Fr_to_montg(&tmp, &tmp);  // L_N*R
        Fr_mul_montg(&numerator, &numerator, &tmp); // N*R
        // update denominator
        Fr_set_limb(&tmp, limb_denominator); // L_D
        Fr_to_montg(&tmp, &tmp);  // L_D*R
        Fr_mul_montg(&denominator, &denominator, &tmp); // D*R
    }
    if (sign) {
        Fr_neg(&denominator, &denominator);
    }

    // at this point, denominator = D*R , numertaor = N*R
    // inversion inv(x) = x^(-1)R
    Fr_inv_montg_eucl(&denominator, &denominator); // (DR)^(-1)*R = D^(-1)
    Fr_mul_montg(res, &numerator, &denominator); // N*D^(-1)     
}

// Computes the Langrange interpolation at zero P(0) = LI(0) with regards to the indices [indices(0)..indices(t)] 
// and their G1 images [shares(0)..shares(t)], and stores the resulting G1 point in `dest`.
// `len` is equal to `t+1` where `t` is the polynomial degree.
static void G1_lagrangeInterpolateAtZero(ep_st* dest, const ep_st shares[], const uint8_t indices[], const int len) {
    // Purpose is to compute Q(0) where Q(x) = A_0 + A_1*x + ... +  A_t*x^t in G1 
    // where A_i = g1 ^ a_i

    // Q(0) = share_i0 ^ L_i0(0) + share_i1 ^ L_i1(0) + .. + share_it ^ L_it(0)
    // where L is the Lagrange coefficient
    
    // temp variables
    ep_t mult;
    ep_new(mult);         
    ep_set_infty(dest);

    Fr fr_lagr_coef;
    for (int i=0; i < len; i++) {
        Fr_lagrangeCoefficientAtZero(&fr_lagr_coef, i, indices, len);
        bn_st* bn_lagr_coef = Fr_blst_to_relic(&fr_lagr_coef);
        ep_mul_lwnaf(mult, &shares[i], bn_lagr_coef);
        free(bn_lagr_coef);
        ep_add_jacob(dest, dest, mult);
    }
    // free the temp memory
    ep_free(mult);
}

// Computes the Langrange interpolation at zero LI(0) with regards to the indices [indices(0)..indices(t)] 
// and their G1 concatenated serializations [shares(1)..shares(t+1)], and stores the serialized result in `dest`.
// `len` is equal to `t+1` where `t` is the polynomial degree.
int G1_lagrangeInterpolateAtZero_serialized(byte* dest, const byte* shares, const uint8_t indices[], const int len) {
    int read_ret;
    // temp variables
    ep_t res;
    ep_new(res);
    ep_st* ep_shares = malloc(sizeof(ep_t) * len);

    for (int i=0; i < len; i++) {
        ep_new(ep_shares[i]);
        read_ret = ep_read_bin_compact(&ep_shares[i], &shares[SIGNATURE_LEN*i], SIGNATURE_LEN);
        if (read_ret != RLC_OK) goto out;
            
    }
    // G1 interpolation at 0
    // computes Q(x) = A_0 + A_1*x + ... +  A_t*x^t  in G1,
    // where A_i = g1 ^ a_i
    G1_lagrangeInterpolateAtZero(res, ep_shares, indices, len);

    // export the result
    ep_write_bin_compact(dest, res, SIGNATURE_LEN);
    read_ret = VALID;

out:
    // free the temp memory
    ep_free(res);
    for (int i=0; i < len; i++) {
        ep_free(ep_shares[i]);
    } 
    free(ep_shares); 
    return read_ret;
}

