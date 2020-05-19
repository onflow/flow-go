// +build relic

// this file contains utility functions for the curve BLS 12-381
// these tools are shared by the BLS signature scheme, the BLS based threshold signature
// and the BLS distributed key generation protcols

#include "bls12381_utils.h"
#include "bls_include.h"

// The functions are tested for ALLOC=AUTO (not for ALLOC=DYNAMIC)

// return macro values to the upper Go Layer
int get_valid() {
    return VALID;
}

int get_invalid() {
    return INVALID;
}

// global variable of the pre-computed data
prec_st bls_prec_st;
prec_st* bls_prec = NULL;

#if (hashToPoint == OPSWU)
extern const uint64_t a1_data[6];
extern const uint64_t b1_data[6];
extern const uint64_t iso_Nx_data[ELLP_Nx_LEN][6];
extern const uint64_t iso_Dx_data[ELLP_Dx_LEN][6];
extern const uint64_t iso_Ny_data[ELLP_Ny_LEN][6];
extern const uint64_t iso_Dy_data[ELLP_Dy_LEN][6];
#endif

// sets the global variable to input
void precomputed_data_set(prec_st* p) {
    bls_prec = p;
}

// Reads a prime field element from a digit vector in big endian format.
static void fp_read_raw(fp_t a, const dig_t *raw, int len) {
     bn_t t;
     bn_null(t); 
     if (len != Fp_DIGITS) {
         THROW(ERR_NO_BUFFER);
     }
      TRY {
         bn_new(t);
         bn_read_raw(t, raw, len);
         if (bn_is_zero(t)) {
             fp_zero(a);
         } else {
             if (t->used == 1) {
                 fp_prime_conv_dig(a, t->dp[0]);
             } else {
                 fp_prime_conv(a, t);
             }
         }
     }
     CATCH_ANY {
         THROW(ERR_CAUGHT);
     }
     FINALLY {
         bn_free(t);
     }
 }

// pre-compute some data required for curve BLS12-381
prec_st* init_precomputed_data_BLS12_381() {
    bls_prec = &bls_prec_st;
    #if (hashToPoint == OPSWU)
        fp_read_raw(bls_prec->a1, a1_data, 6);
        fp_read_raw(bls_prec->b1, b1_data, 6);
        for (int i=0; i<ELLP_Dx_LEN; i++)  
            fp_read_raw(bls_prec->iso_Dx[i], iso_Dx_data[i], 6);
        for (int i=0; i<ELLP_Nx_LEN; i++)  
            fp_read_raw(bls_prec->iso_Nx[i], iso_Nx_data[i], 6);
        for (int i=0; i<ELLP_Dy_LEN; i++)  
            fp_read_raw(bls_prec->iso_Dy[i], iso_Dy_data[i], 6);
        for (int i=0; i<ELLP_Ny_LEN; i++)  
            fp_read_raw(bls_prec->iso_Ny[i], iso_Ny_data[i], 6);
    #endif
    // (p-3)/4
    bn_read_raw(&bls_prec->p_3div4, fp_prime_get(), Fp_DIGITS);
    bn_sub_dig(&bls_prec->p_3div4, &bls_prec->p_3div4, 3);
    bn_rsh(&bls_prec->p_3div4, &bls_prec->p_3div4, 2);
    // (p-1)/2
    fp_sub_dig(bls_prec->p_1div2, fp_prime_get(), 1);
    fp_rsh(bls_prec->p_1div2, bls_prec->p_1div2, 1);
    return bls_prec;
}

// Initializes Relic context with BLS12-381 parameters
ctx_t* relic_init_BLS12_381() { 
    // check Relic was compiled with the right conf 
    if (ALLOC != AUTO) return NULL;

    // initialize relic core with a new context
    ctx_t* bls_ctx = (ctx_t*) malloc(sizeof(ctx_t));
    core_set(bls_ctx);
    if (core_init() != RLC_OK) return NULL;

    // init BLS curve
    int ret = RLC_OK;
    #if (FP_PRIME == 381)
    ret = ep_param_set_any_pairf(); // sets B12_P381 if FP_PRIME = 381 in relic config
    #else
    ep_param_set(B12_P381);
    ep2_curve_set_twist(EP_MTYPE);  // Multiplicative twist 
    #endif 

    if (ret != RLC_OK) return NULL;
    return core_get();
}

// seeds relic PRG
void seed_relic(byte* seed, int len) {
    #if RAND == HASHD
    // instantiate a new DRBG
    ctx_t *ctx = core_get();
    ctx->seeded = 0;
    #endif
    rand_seed(seed, len);
}

// Exponentiation of a generic point p in G1
void ep_mult(ep_st* res, const ep_st* p, const bn_st *expo) {
    // Using window NAF of size 2
    #if (EP_MUL	== LWNAF)
        g1_mul(res, p, expo);
    #else 
        ep_mul_lwnaf(res, p, expo);
    #endif
}

// Exponentiation of generator g1 in G1
// This function is not called by BLS but is here for DEBUG/TESTs purposes
void ep_mult_gen(ep_st* res, const bn_st *expo) {
#define GENERIC_POINT 0
#define FIXED_MULT    (GENERIC_POINT^1)

#if GENERIC_POINT
    ep_mult(res, &core_get()->ep_g, expo);
#elif FIXED_MULT
    // Using precomputed table of size 4
    g1_mul_gen(res, expo);
#endif
}

// Exponentiation of a generic point p in G2
void ep2_mult(ep2_st* res, ep2_st* p, bn_st *expo) {
    // Using window NAF of size 2
    #if (EP_MUL	== LWNAF)
        g2_mul(res, p, expo);
    #else 
        ep2_mul_lwnaf(res, p, expo);
    #endif
}

// Exponentiation of fixed g2 in G2
void ep2_mult_gen(ep2_st* res, const bn_st *expo) {
    // Using precomputed table of size 4
    g2_mul_gen(res, (bn_st*)expo);
}



// DEBUG printing functions 
void bytes_print_(char* s, byte* data, int len) {
    printf("[%s]:\n", s);
    for (int i=0; i<len; i++) 
        printf("%02x,", data[i]);
    printf("\n");
}

void fp_print_(char* s, fp_st a) {
    char* str = malloc(sizeof(char) * fp_size_str(a, 16));
    fp_write_str(str, 100, a, 16);
    printf("[%s]:\n%s\n", s, str);
    free(str);
}

void bn_print_(char* s, bn_st *a) {
    char* str = malloc(sizeof(char) * bn_size_str(a, 16));
    bn_write_str(str, 128, a, 16);
    printf("[%s]:\n%s\n", s, str);
    free(str);
}

void ep_print_(char* s, ep_st* p) {
    printf("[%s]:\n", s);
    g1_print(p);
}

void ep2_print_(char* s, ep2_st* p) {
    printf("[%s]:\n", s);
    g2_print(p);
} 

// generates a random number less than the order r
void bn_randZr(bn_t x) {
    bn_st r;
    bn_new(&r); 
    g2_get_ord(&r);

    bn_new_size(x, bn_size_raw(&r));
    if (x)
        bn_rand_mod(x,&r);
    bn_free(&r);
}

// reads a scalar from an array and maps it to Zr
// the resulting scalar is in the range 0 < a < r
// len must be less than BITS_TO_BYTES(RLC_BN_BITS)
void bn_map_to_Zr(bn_st* a, const uint8_t* bin, int len) {
    bn_st tmp;
    bn_new(&tmp);
    bn_new_size(&tmp, BYTES_TO_DIGITS(len));
    bn_read_bin(&tmp, bin, len);
    bn_t r;
    bn_new(r); 
    g2_get_ord(r);
    bn_sub_dig(r,r,1);
    bn_mod_basic(a,&tmp,r);
    bn_add_dig(a,a,1);
    bn_free(r);
    bn_free(&tmp);
}


// reads a bit in a prime field element at a given index
// whether the field element is in Montgomery domain or not
static int fp_get_bit_generic(const fp_t a, int bit) {
#if (FP_RDC == MONTY)
    bn_st tmp;
    bn_new(&tmp);
    fp_prime_back(&tmp, a);
    int res = bn_get_bit(&tmp, bit);
    bn_free(&tmp);
    return res;
#else
    return fp_get_bit(a, bit);
#endif
}

// uncompress a G1 point p into r taken into account the coordinate x
// and the LS bit of the y coordinate.
// the (y) bit is the LS of (y*R mod p) if Montgomery domain is used, otherwise
// is the LS bit of y 
// (taken and modifed from Relic ep_upk function)
static int ep_upk_generic(ep_t r, const ep_t p) {
    fp_t t;
    int result = 0;
    fp_null(t);
    TRY {
        fp_new(t);
        ep_rhs(t, p);
        /* t0 = sqrt(x1^3 + a * x1 + b). */
        result = fp_srt(t, t);
        if (result) {
            /* Verify if least significant bit of the result matches the
            * compressed y-coordinate. */
            #if (FP_RDC == MONTY)
            bn_st tmp;
            bn_new(&tmp);
            fp_prime_back(&tmp, t);
            if (bn_get_bit(&tmp, 0) != fp_get_bit(p->y, 0)) {
                fp_neg(t, t);
            }
            bn_free(&tmp);
            #else
            if (fp_get_bit(t, 0) != fp_get_bit(p->y, 0)) {
                fp_neg(t, t);
            }
            #endif
            fp_copy(r->x, p->x);
            fp_copy(r->y, t);
            fp_set_dig(r->z, 1);
            r->norm = 1;
        }
    }
    CATCH_ANY {
        THROW(ERR_CAUGHT);
    }
    FINALLY {
        fp_free(t);
    }
    return result;
}


// ep_write_bin_compact exports a point a in E(Fp) to a buffer bin in a compressed or uncompressed form.
// len is the allocated size of the buffer bin for sanity check
// The encoding is inspired from zkcrypto (https://github.com/zkcrypto/pairing/tree/master/src/bls12_381) with a small change to accomodate Relic lib
// The code is a modified version of Relic ep_write_bin
// The most significant bit of the buffer, when set, indicates that the point is in compressed form. 
// Otherwise, the point is in uncompressed form.
// The second-most significant bit, when set, indicates that the point is at infinity. 
// If this bit is set, the remaining bits of the group element's encoding should be set to zero.
// The third-most significant bit is set if (and only if) this point is in compressed form and it is not the point at infinity and its y-coordinate is odd.
void ep_write_bin_compact(byte *bin, const ep_st *a, const int len) {
    ep_t t;
    ep_null(t);
    const int G1size = (G1_BYTES/(SERIALIZATION+1));

    if (len!=G1size) {
        THROW(ERR_NO_BUFFER);
        return;
    }
 
    if (ep_is_infty(a)) {
            // set the infinity bit
            bin[0] = (SERIALIZATION << 7) | 0x40;
            memset(bin+1, 0, G1size-1);
            return;
    }

    TRY {
        ep_new(t);
        ep_norm(t, a);
        fp_write_bin(bin, Fp_BYTES, t->x);

        if (SERIALIZATION == COMPRESSED) {
            bin[0] |= (fp_get_bit_generic(t->y, 0) << 5);
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
// look at the comments of ep_write_bin_compact for a detailed description
// The code is a modified version of Relic ep_write_bin
int ep_read_bin_compact(ep_st* a, const byte *bin, const int len) {
    const int G1size = (G1_BYTES/(SERIALIZATION+1));
    if (len!=G1size) {
        THROW(ERR_NO_BUFFER);
        return RLC_ERR;
    }
    // check if the point is infinity
    if (bin[0] & 0x40) {
        // check if the remaining bits are cleared
        if (bin[0] & 0x3F) {
            THROW(ERR_NO_VALID);
            return RLC_ERR;
        }
        for (int i=1; i<G1size-1; i++) {
            if (bin[i]) {
                THROW(ERR_NO_VALID);
                return RLC_ERR;
            } 
        }
		ep_set_infty(a);
		return RLC_OK;
	} 

    int compressed = bin[0] >> 7;
    int y_is_odd = (bin[0] >> 5) & 1;

    if (y_is_odd && (!compressed)) {
        THROW(ERR_NO_VALID);
        return RLC_ERR;
    } 

	a->norm = 1;
	fp_set_dig(a->z, 1);
    byte* temp = (byte*)malloc(Fp_BYTES);
    if (!temp) {
        THROW(ERR_NO_MEMORY);
        return RLC_ERR;
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
        ep_upk_generic(a, a);
    }
    return RLC_OK;
}

// uncompress a G2 point p into r taking into account the coordinate x
// and the LS bit of the y lower coordinate.
// the (y0) bit is the LS of (y0*R mod p) if Montgomery domain is used, otherwise
// is the LS bit of y0 
// (taken and modifed from Relic ep_upk function)
static  int ep2_upk_generic(ep2_t r, ep2_t p) {
    fp2_t t;
    int result = 0;
    fp2_null(t);
    TRY {
        fp2_new(t);
        ep2_rhs(t, p);
        /* t0 = sqrt(x1^3 + a * x1 + b). */
        result = fp2_srt(t, t);
        if (result) {
            /* Verify if least significant bit of the result matches the
             * compressed y-coordinate. */
            #if (FP_RDC == MONTY)
            bn_st tmp;
            bn_new(&tmp);
            fp_prime_back(&tmp, t[0]);
            if (bn_get_bit(&tmp, 0) != fp_get_bit(p->y[0], 0)) {
                fp2_neg(t, t);
            }
            bn_free(&tmp);
            #else
            if (fp_get_bit(t[0], 0) != fp_get_bit(p->y[0], 0)) {
                fp2_neg(t, t);
            }
            #endif
            fp2_copy(r->x, p->x);
            fp2_copy(r->y, t);
            fp_set_dig(r->z[0], 1);
            fp_zero(r->z[1]);
            r->norm = 1;
        }
    }
    CATCH_ANY {
        THROW(ERR_CAUGHT);
    }
    FINALLY {
        fp2_free(t);
    }
    return result;
}

// ep2_write_bin_compact exports a point in E(Fp^2) to a buffer in a compressed or uncompressed form.
// The code is a modified version of Relic ep2_write_bin
// The most significant bit of the buffer, when set, indicates that the point is in compressed form. 
// Otherwise, the point is in uncompressed form.
// The second-most significant bit indicates that the point is at infinity. 
// If this bit is set, the remaining bits of the group element's encoding should be set to zero.
// The third-most significant bit is set if (and only if) this point is in compressed form and it is not the point at infinity and its y-coordinate is odd.
void ep2_write_bin_compact(byte *bin, const ep2_st *a, const int len) {
    ep2_t t;
    ep2_null(t);
    const int G2size = (G2_BYTES/(SERIALIZATION+1));

    if (len!=G2size) {
        THROW(ERR_NO_BUFFER);
        return;
    }
 
    if (ep2_is_infty((ep2_st *)a)) {
            // set the infinity bit
            bin[0] = (SERIALIZATION << 7) | 0x40;
            memset(bin+1, 0, G2size-1);
            return;
    }

    TRY {
        ep2_new(t);
        ep2_norm(t, (ep2_st *)a);
        fp2_write_bin(bin, 2*Fp_BYTES, t->x, 0);

        if (SERIALIZATION == COMPRESSED) {
            bin[0] |= (fp_get_bit_generic(t->y[0], 0) << 5);
        } else {
            fp2_write_bin(bin + 2*Fp_BYTES, 2*Fp_BYTES, t->y, 0);
        }
    } CATCH_ANY {
        THROW(ERR_CAUGHT);
    }

    bin[0] |= (SERIALIZATION << 7);
    ep_free(t);
}

// ep2_read_bin_compact imports a point from a buffer in a compressed or uncompressed form.
// The code is a modified version of Relic ep_write_bin
int ep2_read_bin_compact(ep2_st* a, const byte *bin, const int len) {
    const int G2size = (G2_BYTES/(SERIALIZATION+1));
    if (len!=G2size) {
        THROW(ERR_NO_VALID);
        return RLC_ERR;
    }

    // check if the point in infinity
    if (bin[0] & 0x40) {
        // the remaining bits need to be cleared
        if (bin[0] & 0x3F) {
            THROW(ERR_NO_VALID);
            return RLC_ERR;
        }
        for (int i=1; i<G2size-1; i++) {
            if (bin[i]) {
                THROW(ERR_NO_VALID);
                return RLC_ERR;
            } 
        }
		ep2_set_infty(a);
		return RLC_OK;
	} 
    byte compressed = bin[0] >> 7;
    byte y_is_odd = (bin[0] >> 5) & 1;
    if (y_is_odd && (!compressed)) {
        THROW(ERR_NO_VALID);
        return RLC_ERR;
    } 
	a->norm = 1;
	fp_set_dig(a->z[0], 1);
	fp_zero(a->z[1]);
    byte* temp = (byte*)malloc(2*Fp_BYTES);
    if (!temp) {
        THROW(ERR_NO_MEMORY);
        return RLC_ERR;
    }
    memcpy(temp, bin, 2*Fp_BYTES);
    // clear the header bits
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
        ep2_upk_generic(a, a);
    }
    return RLC_OK;
}
