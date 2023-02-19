// +build relic

// extra tools to use BLST low level that are needed by the Flow crypto library

#include "blst_include.h"
#include "bls12381_utils.h"

// internal type of BLST `pow256` uses bytes little endian.
// input is bytes big endian as used by Flow crypto lib external scalars.
void pow256_from_be_bytes(pow256 ret, const unsigned char a[Fr_BYTES])
{
    unsigned char* b = (unsigned char*)a + Fr_BYTES - 1;
    if ((uptr_t)ret == (uptr_t)a) { // swap in place
        for (int i=0; i<Fr_BYTES/2; i++) {
            unsigned char tmp = *ret;
            *(ret++) = *b;
            *(b--) = tmp;
        }
        return;
    }
    for (int i=0; i<Fr_BYTES; i++) {
        *(ret++) = *(b--);
    }
}

// maps big-endian bytes into an Fr element using modular reduction
// output is vec256 (also used as Fr)
void vec256_from_be_bytes(vec256 out, const unsigned char *bytes, size_t n)
{
    // TODO: optimize once working
    vec256 digit, radix;
    vec_zero(out, Fr_BYTES);
    vec_copy(radix, BLS12_381_rRR, sizeof(radix));

    bytes += n;
    while (n > 32) {
        limbs_from_be_bytes(digit, bytes -= 32, 32);
        from_mont_256(digit, digit, BLS12_381_r, r0);
        mul_mont_sparse_256(digit, digit, radix, BLS12_381_r, r0);
        add_mod_256(out, out, digit, BLS12_381_r);
        mul_mont_sparse_256(radix, radix, BLS12_381_rRR, BLS12_381_r, r0);
        n -= 32;
    }
    limbs_from_be_bytes(digit, bytes -= n, n);
    from_mont_256(digit, digit, BLS12_381_r, r0);
    mul_mont_sparse_256(digit, digit, radix, BLS12_381_r, r0);
    add_mod_256(out, out, digit, BLS12_381_r);

    vec_zero(digit, sizeof(digit));
}