#include "include.h"

// DEBUG related functions
void _fp_print(char* s, fp_st* a) {
    char* str = malloc(sizeof(char) * fp_size_str(*a, 16));
    fp_write_str(str, 100, *a, 16);
    printf("[%s]:\n%s\n", s, str);
    free(str);
}

void _bn_print(char* s, bn_st *a) {
    char* str = malloc(sizeof(char) * bn_size_str(a, 16));
    bn_write_str(str, 100, a, 16);
    printf("[%s]:\n%s\n", s, str);
    free(str);
}

void _ep_print(char* s, ep_st* p) {
    printf("[%s]:\n", s);
    g1_print(p);
}

void _ep2_print(char* s, ep2_st* p) {
    printf("[%s]:\n", s);
    g2_print(p);
}

// Ignoring seed for now and relying on relic entropy
void _bn_randZr(bn_t x, byte* seed) {
    bn_t r;
    bn_new(r); 
    g2_get_ord(r);

    bn_new_size(x, bn_size_raw(r));
    if (x)
        bn_rand_mod(x,r);
    bn_free(r);
}

// ep_write_bin_compact exports a point to a buffer in a compressed or uncompressed form.
// The coding is inspired from zkcrypto (https://github.com/zkcrypto/pairing/tree/master/src/bls12_381) with a small change to accomodate Relic lib
// The code is a modified version of Relic ep_write_bin
// The most significant bit of the buffer, when set, indicates that the point is in compressed form. 
// Otherwise, the point is in uncompressed form.
// The second-most significant bit indicates that the point is at infinity. 
// If this bit is set, the remaining bits of the group element's encoding should be set to zero.
// The third-most significant bit is set if (and only if) this point is in compressed form and it is not the point at infinity and its y-coordinate is odd.
void ep_write_bin_compact(byte *bin, const ep_st *a) {
    ep_t t;
    ep_null(t);
 
    if (ep_is_infty(a)) {
            bin[0] = (SERIALIZATION << 7) | 0x40;
            memset(bin+1, 0, SIGNATURE_LEN-1);
            return;
    }

    TRY {
        ep_new(t);
        ep_norm(t, a);
 
        if (SERIALIZATION == COMPRESSED) {
            fp_write_bin(bin, SIGNATURE_LEN, t->x);
            bin[0] |= (fp_get_bit(t->y, 0) << 5);
        } else {
            fp_write_bin(bin, SIGNATURE_LEN, t->x);
            fp_write_bin(bin + SIGNATURE_LEN, SIGNATURE_LEN, t->y);
        }
    } CATCH_ANY {
        THROW(ERR_CAUGHT);
    }

    bin[0] |= (SERIALIZATION << 7);
    ep_free(t);
 }


// ep_read_bin_compact imports a point from a buffer in a compressed or uncompressed form.
// The coding is inspired from zkcrypto (https://github.com/zkcrypto/pairing/tree/master/src/bls12_381) with a small change to accomodate Relic lib
// The code is a modified version of Relic ep_write_bin
void ep_read_bin_compact(ep_st* a, const byte *bin) {
    if ((bin[0] & 0x40) == 0) {
        if (bin[0] & 0x3F) {
            THROW(ERR_NO_VALID);
            return;
        }
        for (int i=1; i<SIGNATURE_LEN; i++) {
            if (bin[i]) {
                THROW(ERR_NO_VALID);
                return;
            } 
        }
		ep_set_infty(a);
		return;
	} 

    int compressed = bin[0] >> 7;
    int y_is_odd = (bin[0] >> 6) & 1;

    if (y_is_odd && (!compressed)) {
        THROW(ERR_NO_VALID);
        return;
    } 

	a->norm = 1;
	fp_set_dig(a->z, 1);
	fp_read_bin(a->x, bin, SIGNATURE_LEN>>1);
    for (int i=0; i<3; i++){
        fp_set_bit(a->x, FP_PRIME+i, 0);
    }

    if (SERIALIZATION == UNCOMPRESSED) {
        fp_read_bin(a->y, bin + (SIGNATURE_LEN>>1), SIGNATURE_LEN>>1);
    }
    else {
        fp_set_bit(a->y, 0, y_is_odd);
        ep_upk(a, a);
    }
}