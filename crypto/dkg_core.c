// +build relic

#include "dkg_include.h"


#define N_max 250
#define N_bits_max 8  // log(250)  
#define T_max  ((N_max-1)/2)

// computes P(x) = a_0 + a_1*x + .. + a_n x^n (mod r)
// r being the order of G1
// writes P(x) in out and P(x).g2 in y if y is non NULL
// x being a small integer
void Zr_polynomialImage_export(byte* out, ep2_t y, const bn_t a, const int a_size, const byte x){
    bn_t image;
    bn_new(image);
    Zr_polynomialImage(image, y, a, a_size, x);
    // exports the result
    const int out_size = Fr_BYTES;
    bn_write_bin(out, out_size, image);
    bn_free(image);
}

// computes P(x) = a_0 + a_1*x + .. + a_n x^n (mod r)
// r being the order of G1
// writes P(x) in out and P(x).g2 in y if y is non NULL
// x being a small integer
void Zr_polynomialImage(bn_t image, ep2_t y, const bn_st *a, const int a_size, const byte x){
    bn_t r;
    bn_new(r); 
    g2_get_ord(r);
    
    // temp variables
    bn_t acc;
    bn_new(acc); 
    bn_new_size(acc, BITS_TO_DIGITS(Fr_BITS+8+1));
    bn_set_dig(acc, 0);

    for (int i=a_size-1; i >= 0; i--) {
        bn_mul_dig(acc, acc, x);
        // Use basic reduction as it's an 9-bits reduction 
        // in the worst case (|acc|<|r|+9 )
        bn_mod_basic(acc, acc, r);
        bn_add(acc, acc, &a[i]);
    }
    // export the result
    bn_mod_basic(image, acc, r);

    // compute y = P(x).g2
    if (y) g2_mul_gen(y, acc);

    bn_free(acc)
    bn_free(r);
}

// computes Q(x) = A_0 + A_1*x + ... +  A_n*x^n  in G2
// and stores the point in y
// r is the order of G2
static void G2_polynomialImage(ep2_t y, const ep2_st* A, const int len_A,
         const byte x, const bn_t r){
    
    bn_t bn_x;        
    bn_new(bn_x);    
    ep2_set_infty(y);
    bn_set_dig(bn_x, x);
    for (int i = len_A-1; i >= 0 ; i--) {
        ep2_mul_lwnaf(y, y, bn_x);
        ep2_add_projc(y, y, (ep2_st*)&A[i]);
    }

    ep2_norm(y, y); // not necessary but left here to optimize the 
                    // multiple pairing computations with the same public key
    bn_free(bn_x);
}

// compute the participants public keys from the verification vector
// y[i] = Q(i+1) for all participants i, with:
// Q(x) = A_0 + A_1*x + ... +  A_n*x^n  in G2
void G2_polynomialImages(ep2_st *y, const int len_y, const ep2_st* A, const int len_A) {
    // order r
    bn_t r;
    bn_new(r); 
    g2_get_ord(r);
    for (byte i=0; i<len_y; i++) {
        //y[i] = Q(i+1)
        G2_polynomialImage(y+i , A, len_A, i+1, r);
    }
    bn_free(r);
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

// imports an array of ep2_st from an array of bytes
// the length matching is supposed to be already done
int ep2_vector_read_bin(ep2_st* A, const byte* src, const int len){
    const int size = (G2_BYTES/(G2_SERIALIZATION+1));
    byte* p = (byte*) src;
    for (int i=0; i<len; i++){
        int read_ret = ep2_read_bin_compact(&A[i], p, size);
        if (read_ret != RLC_OK)
            return read_ret;
        p += size;
    }
    return RLC_OK;
}

// returns 1 if g2^x = y, where g2 is the generator of G2
// returns 0 otherwise
int verifyshare(const bn_t x, const ep2_t y) {
    ep2_t res;
    ep2_new(res);
    g2_mul_gen(res, (bn_st*)x);
    return (ep2_cmp(res, (ep2_st*)y) == RLC_EQ);
}

