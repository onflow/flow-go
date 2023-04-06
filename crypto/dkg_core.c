// +build relic

#include "dkg_include.h"


#define N_max 250
#define N_bits_max 8  // log(250)  
#define T_max  ((N_max-1)/2)

// computes P(x) = a_0 + a_1*x + .. + a_n x^n (mod r)
// r being the order of G1
// writes P(x) in out and P(x).g2 in y if y is non NULL
// x being a small integer
void Fr_polynomialImage_export(byte* out, ep2_t y, const Fr* a, const int a_size, const byte x){
    Fr image;
    Fr_polynomialImage(&image, y, a, a_size, x);
    // exports the result
    Fr_write_bytes(out, &image);
}

// computes P(x) = a_0 + a_1 * x + .. + a_n * x^n  where P is in Fr[X].
// a_i are all in Fr, `a_size` - 1 is P's degree, x is a small integer less than 255.
// The function writes P(x) in `image` and P(x).g2 in `y` if `y` is non NULL
void Fr_polynomialImage(Fr* image, ep2_t y, const Fr* a, const int a_size, const byte x){
    Fr_set_zero(image); 
    // convert `x` to Montgomery form
    Fr xR;
    Fr_set_limb(&xR, (limb_t)x);
    Fr_to_montg(&xR, &xR);

    for (int i = a_size-1; i >= 0; i--) {
        Fr_mul_montg(image, image, &xR); 
        Fr_add(image, image, &a[i]); // image is in normal form
    }
    // compute y = P(x).g2
    if (y) {
        bn_st* tmp = Fr_blst_to_relic(image);
        g2_mul_gen(y, tmp);
        free(tmp);
    }
}

// computes Q(x) = A_0 + A_1*x + ... +  A_n*x^n  in G2
// and stores the point in y
// r is the order of G2
static void G2_polynomialImage(ep2_t y, const ep2_st* A, const int len_A, const byte x){
    
    bn_t bn_x;        
    bn_new(bn_x);    
    ep2_set_infty(y);
    bn_set_dig(bn_x, x);
    for (int i = len_A-1; i >= 0 ; i--) {
        ep2_mul_lwnaf(y, y, bn_x);
        ep2_add_projc(y, y, (ep2_st*)&A[i]);
    }

    ep2_norm(y, y); // not necessary but called to optimize the 
                    // multiple pairing computations with the same public key
    bn_free(bn_x);
}

// computes y[i] = Q(i+1) for all participants i ( 0 <= i < len_y)
// where Q(x) = A_0 + A_1*x + ... +  A_n*x^n  in G2[X]
void G2_polynomialImages(ep2_st *y, const int len_y, const ep2_st* A, const int len_A) {
    for (byte i=0; i<len_y; i++) {
        //y[i] = Q(i+1)
        G2_polynomialImage(y+i , A, len_A, i+1);
    }
}

// export an array of ep2_st into an array of bytes
// the length matching is supposed to be checked
void ep2_vector_write_bin(byte* out, const ep2_st* A, const int len) {
    const int size = (G2_BYTES/(G2_SERIALIZATION+1));
    byte* p = out;
    for (int i=0; i<len; i++){
        ep2_write_bin_compact(p, &A[i], size);
        p += size;
    }
}

// The function imports an array of ep2_st from an array of bytes
// the length matching is supposed to be already done.
//
// It returns RLC_OK if reading all the vector succeeded and RLC_ERR 
// otherwise.
int ep2_vector_read_bin(ep2_st* A, const byte* src, const int len){
    const int size = (G2_BYTES/(G2_SERIALIZATION+1));
    byte* p = (byte*) src;
    for (int i=0; i<len; i++){
        int read_ret = G2_read_bytes(&A[i], p, size); // returns RLC_OK or RLC_ERR
        if (read_ret != RLC_OK)
            return read_ret;
        p += size;
    }
    return RLC_OK;
}

// returns 1 if g2^x = y, where g2 is the generator of G2
// returns 0 otherwise
int verifyshare(const Fr* x, const ep2_t y) {
    ep2_t res;
    ep2_new(res);
    bn_st* x_tmp = Fr_blst_to_relic(x);
    g2_mul_gen(res, x_tmp);
    free(x_tmp);
    return (ep2_cmp(res, (ep2_st*)y) == RLC_EQ);
}

