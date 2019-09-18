
#include "dkg_include.h"
#include "bls_include.h"

// computes P(x) = a_0 + a_1*x + .. + a_n x^n (mod r)
// r being the order of G1
// x being a small integer
void _polynomialImage(byte* out, bn_st* a, int a_size, int x) {
    // order of G1 and G2
    // could ve hardcoded as get_ord() is doing a copy instead of returning a pointer
    printf("POLY\n");
    bn_st r;
    bn_new(&r); 
    g2_get_ord(&r);
    // Barett reduction constant
    // TODO: hardcode u
    bn_st u;
    bn_new(&u)
    bn_mod_pre_barrt(&u, &r);
    // powers of x
    bn_st bn_x;
    ep_new(&bn_x);
    bn_new_size(&bn_x, 2*bn_size_raw(&bn_x));
    bn_set_dig(&bn_x, 1);
    
    // temp variables
    bn_st mult, acc;
    bn_new(&mult);
    bn_new_size(&mult, 3*bn_size_raw(&r));
    bn_new(&acc);
    bn_new_size(&acc, 3*bn_size_raw(&r)+1);
    bn_set_dig(&acc, 0);

    /*_bn_print("bn_x", &bn_x);
    _bn_print("r", &r);
    _bn_print("acc", &acc);
    _bn_print("u", &u);*/

    printf("LOOP\n");

    for (int i=0; i<a_size; i++) {
        bn_mul(&mult, &a[i], &bn_x);
        bn_add(&acc, &acc, &mult);
        bn_mod_barrt(&acc, &acc, &r, &u);
        bn_mul_dig(&bn_x, &bn_x, x);
    }

    // export the result
    const int out_size = SK_LEN;
    _bn_print("acc", &acc);
    bn_write_bin(out, out_size, &acc);
    _bytes_print("bytes", out, out_size);
    /*for (int i=0; i<out_size; i++) {
        out[i] = 0 ;       
    }
    out[0] = x;*/

    bn_free(&acc)
    bn_free(&mult);
    bn_free(&r);
    bn_free(&bn_x);
    bn_free(&u);
}