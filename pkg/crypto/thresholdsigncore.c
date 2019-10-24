
#include "thresholdsign_include.h"
#include "dkg_include.h"

// Computes the Lagrange coefficiecient L(i) of (i+1) at 0 with regards to the range [signers(0)+1..signers(t)+1]
// and stores it in res
// where t is the degree of the polynomial P
static void Zr_lagrangeCoefficientAtZero(bn_st* res, const int i, const uint32_t* signers, const int len){
    //printf("index %d\n", i);
    bn_st r, r_2;
    bn_new(&r);
    g2_get_ord(&r);
    bn_new(&r_2);
    bn_sub_dig(&r_2, &r, 2);
    // Barett reduction constant
    // TODO: hardcode u
    bn_st u;
    bn_new(&u)
    bn_mod_pre_barrt(&u, &r);

    bn_st acc, inv, base, numerator;
    bn_new(&inv);
    bn_new(&base);
    bn_new_size(&base, BITS_TO_DIGITS(Fr_BITS))
    bn_new(&acc);
    bn_new(&numerator);
    bn_new_size(&acc, BITS_TO_DIGITS(3*Fr_BITS));

    bn_set_dig(&acc, 1);
    /*for (int k=0; k<len; k++){
        //_bn_print("acc",&acc);
        if (signers[k]==i) continue;
        // (k-i)^-1 mod r
        if (signers[k]>i) {
            bn_set_dig(&base, signers[k]-i);
        }
        else if (signers[k]<i) {
            bn_set_dig(&base, i-signers[k]);
            bn_sub(&base, &r, &base);
        } 
        //_bn_print("base",&base);
        bn_mxp_slide(&inv, &base, &r_2, &r);
        //_bn_print("inv",&inv);
        // fp_inv_exgcd_bn
        bn_mul(&acc, &inv, &acc);
        bn_mul_dig(&acc, &acc, signers[k]+1);
        bn_mod_barrt(&acc, &acc, &r, &u);
    }*/
    // the sign of the accumulator acc, equal to 1 if positive, 0 if negative
    int sign = 1;
    for (int k=0; k<len; k+=32){
        bn_set_dig(&base, 1);
        bn_set_dig(&numerator, 1);
        for (int j=k; j< MIN(len, k+32); j++){
            if (signers[j]==i) continue;
            // (k-i)^-1 mod r
            if (signers[j]<i) sign ^= 1;
            bn_mul_dig(&base, &base, abs((int)signers[j]-i));
            bn_mul_dig(&numerator, &numerator, signers[j]+1);
        }
        bn_mxp_slide(&inv, &base, &r_2, &r);
        bn_mul(&acc, &acc, &inv);
        bn_mod_barrt(&acc, &acc, &r, &u);
        bn_mul(&acc, &acc, &numerator);
        bn_mod_barrt(&acc, &acc, &r, &u);
    }
    if (sign) bn_copy(res, &acc);
    else bn_sub(res, &r, &acc);
    bn_free(&r);bn_free(&r_1);
    bn_free(&u);bn_free(&acc);
    bn_free(&inv);bn_free(&base);
}


// Computes the Langrange interpolation at zero LI(0) with regards to the points [signers(1)+1..signers(t+1)+1] 
// and their images [shares(1)..shares(t+1)]
// len is the polynomial degree 
// the result is stored at dest
void G1_lagrangeInterpolateAtZero(byte* dest, const byte* shares, const uint32_t* signers, const int len) {
    // computes Q(x) = A_0 + A_1*x + ... +  A_n*x^n  in G2
    // powers of x
    bn_st bn_lagr_coef;
    bn_new(&bn_lagr_coef);
    bn_new_size(&bn_lagr_coef, BITS_TO_BYTES(Fr_BITS));
    
    // temp variables
    ep_st mult, acc, share;
    ep_new(&mult);         
    ep_new(&acc);
    ep_new(&share);
    ep_set_infty(&acc);

    for (int i=0; i < len; i++) {
        _ep_read_bin_compact(&share, &shares[SIGNATURE_LEN*i], SIGNATURE_LEN);
        Zr_lagrangeCoefficientAtZero(&bn_lagr_coef, signers[i], signers, len);
        ep_mul_lwnaf(&mult, &share, &bn_lagr_coef);
        ep_add_projc(&acc, &acc, &mult);
    }
    // export the result
    _ep_write_bin_compact(dest, &acc, SIGNATURE_LEN);

    ep2_free(&acc);
    ep2_free(&mult);
    ep2_free(&share);
    bn_free(&bn_lagr_coef);
    return;
}