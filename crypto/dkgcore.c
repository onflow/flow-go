
#include "dkg_include.h"
#include "bls_include.h"

#define N_max 250
#define N_bits_max 8  // log(250)  
#define T_max  ((N_max-1)/2)
#define BITS_TO_DIGITS(x) ((x)/64)

// computes P(x) = a_0 + a_1*x + .. + a_n x^n (mod r)
// r being the order of G1
// writes P(x) in out and P(x).g2 in y
// x being a small integer
void Zr_polynomialImage(byte* out, bn_st* a, const int a_size, const int x, ep2_st* y){
    bn_st r;
    bn_new(&r); 
    g2_get_ord(&r);
    // Barett reduction constant
    // TODO: hardcode u
    bn_st u;
    bn_new(&u)
    bn_mod_pre_barrt(&u, &r);
    // powers of x
    bn_st bn_x;         // maximum is |n|+|r| --> 264 bits
    ep_new(&bn_x);
    bn_new_size(&bn_x, BITS_TO_DIGITS(Fr_BITS+N_bits_max));
    bn_set_dig(&bn_x, 1);
    
    // temp variables
    bn_st mult, acc;
    bn_new(&mult);         // maximum --> 256+256 = 512 bits
    bn_new_size(&mult, BITS_TO_DIGITS(2*Fr_BITS));
    bn_new(&acc);         // maximum --> 512+1 = 513 bits
    bn_new_size(&acc, BITS_TO_DIGITS(2*Fr_BITS+1));
    bn_set_dig(&acc, 0);

    for (int i=0; i<a_size; i++) {
        bn_mul(&mult, &a[i], &bn_x);
        bn_add(&acc, &acc, &mult);
        bn_mod_barrt(&acc, &acc, &r, &u);
        // TODO: hardcode x^i mod r ?
        bn_mul_dig(&bn_x, &bn_x, x);
        bn_mod_barrt(&bn_x, &bn_x, &r, &u);
    }
    // exports the result
    const int out_size = Fr_BYTES;
    bn_write_bin(out, out_size, &acc); 

    // compute y = P(x).g2
    g2_mul_gen(y, &acc);

    bn_free(&acc)
    bn_free(&mult);
    bn_free(&r);
    bn_free(&bn_x);
    bn_free(&u);
}

// computes Q(x) = A_0 + A_1*x + ... +  A_n*x^n  in G2
// and stores the point in y
// r is the order of G2, u is the barett constant associated to r
static void G2_polynomialImage(ep2_st* y, ep2_st* A, int len_A,
         int x, bn_st* r, bn_st* u){
    // powers of x
    bn_st bn_x;         // maximum is |n|+|r| --> 264 bits
    ep_new(&bn_x);
    bn_new_size(&bn_x, BITS_TO_DIGITS(Fr_BITS+N_bits_max));
    bn_set_dig(&bn_x, 1);
    
    // temp variables
    ep2_st mult, acc;
    ep2_new(&mult);         
    ep2_new(&acc);
    ep2_set_infty(&acc);

    for (int i=0; i < len_A; i++) {
        ep2_mul_lwnaf(&mult, &A[i], &bn_x);
        ep2_add_projc(&acc, &acc, &mult);
        // TODO: hardcode x^i mod r ?
        bn_mul_dig(&bn_x, &bn_x, x);
        bn_mod_barrt(&bn_x, &bn_x, r, u);
    }
    // export the result
    ep2_copy(y, &acc);
    ep2_norm(y, y);

    ep2_free(&acc)
    ep2_free(&mult);
    bn_free(&bn_x);
}

// compute the nodes public keys from the verification vector
// y[i] = Q(i+1) for all nodes i, with:
// Q(x) = A_0 + A_1*x + ... +  A_n*x^n  in G2
// for small x
void G2_polynomialImages(ep2_st* y, int len_y, ep2_st* A, int len_A) {
    // order r
    bn_st r;
    bn_new(&r); 
    g2_get_ord(&r);
    // Barett reduction constant
    // TODO: hardcode u
    bn_st u;
    bn_new(&u)
    bn_mod_pre_barrt(&u, &r);
    for (int i=0; i<len_y; i++) {
        //y[i] = Q(i+1)
        G2_polynomialImage(y+i , A, len_A, i+1, &r, &u);
    }
    bn_free(&u);
    bn_free(&r);
}

// export an array of ep2_st into an array of bytes
// the length matching is supposed to be done
void write_ep2st_vector(byte* out, ep2_st* A, const int len) {
    const int size = 2*Fp_BYTES+1;
    byte* p = out;
    for (int i=0; i<len; i++){
        ep2_write_bin(p, size, &A[i], 1);
        p+= size;
    }
}

// imports an array of ep2_st from an array of bytes
// the length matching is supposed to be done
void read_ep2st_vector(ep2_st* A, byte* src, const int len){
    const int size = 2*Fp_BYTES+1;
    byte* p = src;
    for (int i=0; i<len; i++){
        if (!*p) ep2_read_bin(&A[i], p, 1);
        else ep2_read_bin(&A[i], p, size);
        p+= size;
    }
}

int verifyshare(bn_st* x, ep2_st* y) {
    ep2_st res;
    ep2_new(res);
    g2_mul_gen(&res, x);
    return (ep2_cmp(&res, y) == RLC_EQ);
}