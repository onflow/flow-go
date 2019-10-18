#include "misc.h"
#include "bls_include.h"


// DEBUG related functions
void _bytes_print(char* s, byte* data, int len) {
    printf("[%s]:\n", s);
    for (int i=0; i<len; i++) 
        printf("%02x,", data[i]);
    printf("\n");
}

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

// seeds relic PRG
void _seed_relic(byte* seed, int len) {
    rand_seed(seed, len);
}

// generates a random number less than the order r
void _bn_randZr(bn_t x) {
    bn_t r;
    bn_new(r); 
    g2_get_ord(r);

    bn_new_size(x, bn_size_raw(r));
    if (x)
        bn_rand_mod(x,r);
    bn_free(r);
    //bn_set_dig(x, 0);
}

// ep_write_bin_compact exports a point a in E(Fp) to a buffer bin in a compressed or uncompressed form.
// len is the allocated size of the buffer bin for sanity check
// The encoding is inspired from zkcrypto (https://github.com/zkcrypto/pairing/tree/master/src/bls12_381) with a small change to accomodate Relic lib
// The code is a modified version of Relic ep_write_bin
// The most significant bit of the buffer, when set, indicates that the point is in compressed form. 
// Otherwise, the point is in uncompressed form.
// The second-most significant bit indicates that the point is at infinity. 
// If this bit is set, the remaining bits of the group element's encoding should be set to zero.
// The third-most significant bit is set if (and only if) this point is in compressed form and it is not the point at infinity and its y-coordinate is odd.
void _ep_write_bin_compact(byte *bin, const ep_st *a, const int len) {
    ep_t t;
    ep_null(t);
    const int G1size = (G1_BYTES/(SERIALIZATION+1));

    if (len!=G1size) {
        THROW(ERR_NO_BUFFER);
        return;
    }
 
    if (ep_is_infty(a)) {
            bin[0] = (SERIALIZATION << 7) | 0x40;
            memset(bin+1, 0, G1size-1);
            return;
    }

    TRY {
        ep_new(t);
        ep_norm(t, a);
        fp_write_bin(bin, Fp_BYTES, t->x);

        if (SERIALIZATION == COMPRESSED) {
            bin[0] |= (fp_get_bit(t->y, 0) << 5);
        } else {
            fp_write_bin(bin + Fp_BYTES, Fp_BYTES, t->y);
        }
    } CATCH_ANY {
        THROW(ERR_CAUGHT);
    }

    bin[0] |= (SERIALIZATION << 7);
    ep_free(t);
 }


// ep_read_bin_compact imports a point from a buffer in a compressed or uncompressed form.
// len is the size of the input buffer
// The encoding is inspired from zkcrypto (https://github.com/zkcrypto/pairing/tree/master/src/bls12_381) with a small change to accomodate Relic lib
// The code is a modified version of Relic ep_write_bin
void _ep_read_bin_compact(ep_st* a, const byte *bin, const int len) {
    const int G1size = (G1_BYTES/(SERIALIZATION+1));
    if (len!=G1size) {
        THROW(ERR_NO_BUFFER);
        return;
    }
    if (bin[0] & 0x40) {
        if (bin[0] & 0x3F) {
            THROW(ERR_NO_VALID);
            return;
        }
        for (int i=1; i<G1size-1; i++) {
            if (bin[i]) {
                THROW(ERR_NO_VALID);
                return;
            } 
        }
		ep_set_infty(a);
		return;
	} 

    int compressed = bin[0] >> 7;
    int y_is_odd = (bin[0] >> 5) & 1;

    if (y_is_odd && (!compressed)) {
        THROW(ERR_NO_VALID);
        return;
    } 

	a->norm = 1;
	fp_set_dig(a->z, 1);
    byte* temp = (byte*)malloc(Fp_BYTES);
    if (!temp) {
        THROW(ERR_NO_MEMORY);
        return;
    }
    memcpy(temp, bin, Fp_BYTES);
    temp[0] &= 0x1F;
	fp_read_bin(a->x, temp, Fp_BYTES);
    free(temp);

    if (SERIALIZATION == UNCOMPRESSED) {
        fp_read_bin(a->y, bin + Fp_BYTES, Fp_BYTES);
    }
    else {
        fp_zero(a->y);
        fp_set_bit(a->y, 0, y_is_odd);
        ep_upk(a, a);
    }
}

// _ep2_write_bin_compact exports a point in E(Fp^2) to a buffer in a compressed or uncompressed form.
// The code is a modified version of Relic ep2_write_bin
// The most significant bit of the buffer, when set, indicates that the point is in compressed form. 
// Otherwise, the point is in uncompressed form.
// The second-most significant bit indicates that the point is at infinity. 
// If this bit is set, the remaining bits of the group element's encoding should be set to zero.
// The third-most significant bit is set if (and only if) this point is in compressed form and it is not the point at infinity and its y-coordinate is odd.
void _ep2_write_bin_compact(byte *bin, const ep2_st *a, const int len) {
    ep2_t t;
    ep2_null(t);
    const int G2size = (G2_BYTES/(SERIALIZATION+1));

    if (len!=G2size) {
        THROW(ERR_NO_BUFFER);
        return;
    }
 
    if (ep2_is_infty((ep2_st *)a)) {
            bin[0] = (SERIALIZATION << 7) | 0x40;
            memset(bin+1, 0, G2size-1);
            return;
    }

    TRY {
        ep2_new(t);
        ep2_norm(t, (ep2_st *)a);
        fp2_write_bin(bin, 2*Fp_BYTES, t->x, 0);

        if (SERIALIZATION == COMPRESSED) {
            bin[0] |= (fp_get_bit(t->y[0], 0) << 5);
        } else {
            fp2_write_bin(bin + 2*Fp_BYTES, 2*Fp_BYTES, t->y, 0);
        }
    } CATCH_ANY {
        THROW(ERR_CAUGHT);
    }

    bin[0] |= (SERIALIZATION << 7);
    ep_free(t);
}

// _ep2_read_bin_compact imports a point from a buffer in a compressed or uncompressed form.
// The code is a modified version of Relic ep_write_bin
void _ep2_read_bin_compact(ep2_st* a, const byte *bin, const int len) {
    const int G2size = (G2_BYTES/(SERIALIZATION+1));
    if (len!=G2size) {
        THROW(ERR_NO_BUFFER);
        return;
    }

    if (bin[0] & 0x40) {
        if (bin[0] & 0x3F) {
            THROW(ERR_NO_VALID);
            return;
        }
        for (int i=1; i<G2size-1; i++) {
            if (bin[i]) {
                THROW(ERR_NO_VALID);
                return;
            } 
        }
		ep2_set_infty(a);
		return;
	} 
    byte compressed = bin[0] >> 7;
    byte y_is_odd = (bin[0] >> 5) & 1;
    if (y_is_odd && (!compressed)) {
        THROW(ERR_NO_VALID);
        return;
    } 
	a->norm = 1;
	fp_set_dig(a->z[0], 1);
	fp_zero(a->z[1]);
    byte* temp = (byte*)malloc(2*Fp_BYTES);
    if (!temp) {
        THROW(ERR_NO_MEMORY);
        return;
    }
    memcpy(temp, bin, 2*Fp_BYTES);
    temp[0] &= 0x1F;
    fp2_read_bin(a->x, temp, 2*Fp_BYTES);
    free(temp);


    if (SERIALIZATION == UNCOMPRESSED) {
        fp2_read_bin(a->y, bin + 2*Fp_BYTES, 2*Fp_BYTES);
    }
    else {
        fp2_zero(a->y);
        fp_set_bit(a->y[0], 0, y_is_odd);
		fp_zero(a->y[1]);
        ep2_upk(a, a);
    }
}

// Simple hashing to G1 as described in the original BLS paper 
// https://www.iacr.org/archive/asiacrypt2001/22480516.pdf
// taken and modified from Relic library
void mapToG1_simple(ep_t p, const uint8_t *msg, int len) {
	bn_t k, pm1o2;
	fp_t t;
	uint8_t digest[RLC_MD_LEN];

	bn_null(k);
	bn_null(pm1o2);
	fp_null(t);
	ep_null(q);

	TRY {
		bn_new(k);
		bn_new(pm1o2);
		fp_new(t);
		ep_new(q);

		pm1o2->sign = RLC_POS;
		pm1o2->used = RLC_FP_DIGS;
		dv_copy(pm1o2->dp, fp_prime_get(), RLC_FP_DIGS);
		bn_hlv(pm1o2, pm1o2);
		md_map(digest, msg, len);
		bn_read_bin(k, digest, RLC_MIN(RLC_FP_BYTES, RLC_MD_LEN));
		fp_prime_conv(t, k);
		fp_prime_back(k, t);

        fp_prime_conv(p->x, k);
        fp_zero(p->y);
        fp_set_dig(p->z, 1);

        while (1) {
            ep_rhs(t, p);
            if (fp_srt(p->y, t)) {
                p->norm = 1;
                break;
            }
            fp_add_dig(p->x, p->x, 1);
        }

        // Now, multiply by cofactor to get the correct group. 
        ep_curve_get_cof(k);
        if (bn_bits(k) < RLC_DIG) {
            ep_mul_dig(p, p, k->dp[0]);
        } else {
            ep_mul_basic(p, p, k);
        }
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(k);
		bn_free(pm1o2);
		fp_free(t);
		ep_free(q);
	}
}

// computes hashing to G1 
// DEBUG/test function
ep_st* _hashToG1(const byte* data, const int len) {
    ep_st* h = (ep_st*) malloc(sizeof(ep_st));
    ep_new(h);
    // hash to G1 (construction 2 in https://eprint.iacr.org/2019/403.pdf)
    mapToG1_swu(h, data, len); 
    return h;
}