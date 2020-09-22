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
extern const uint64_t p_3div4_data[Fp_DIGITS];
extern const uint64_t p_1div2_data[Fp_DIGITS];
extern const uint64_t a1_data[Fp_DIGITS];
extern const uint64_t b1_data[Fp_DIGITS];
extern const uint64_t iso_Nx_data[ELLP_Nx_LEN][Fp_DIGITS];
extern const uint64_t iso_Dx_data[ELLP_Dx_LEN][Fp_DIGITS];
extern const uint64_t iso_Ny_data[ELLP_Ny_LEN][Fp_DIGITS];
extern const uint64_t iso_Dy_data[ELLP_Dy_LEN][Fp_DIGITS];
#endif

#if (MEMBERSHIP_CHECK_G1 == BOWE)
extern const uint64_t beta_data[Fp_DIGITS];
extern const uint64_t z2_1_by3_data[2];
#endif

// sets the global variable to input
void precomputed_data_set(prec_st* p) {
    bls_prec = p;
}

// Reads a prime field element from a digit vector in big endian format.
// There is no conversion to Montgomery domain in this function.
 #define fp_read_raw(a, data_pointer) dv_copy((a), (data_pointer), Fp_DIGITS)

// pre-compute some data required for curve BLS12-381
prec_st* init_precomputed_data_BLS12_381() {
    bls_prec = &bls_prec_st;
    #if (hashToPoint == OPSWU)
    fp_read_raw(bls_prec->a1, a1_data);
    fp_read_raw(bls_prec->b1, b1_data);
    // (p-3)/4
    bn_read_raw(&bls_prec->p_3div4, p_3div4_data, Fp_DIGITS);
    // (p-1)/2
    fp_read_raw(bls_prec->p_1div2, p_1div2_data);
    for (int i=0; i<ELLP_Dx_LEN; i++)  
        fp_read_raw(bls_prec->iso_Dx[i], iso_Dx_data[i]);
    for (int i=0; i<ELLP_Nx_LEN; i++)  
        fp_read_raw(bls_prec->iso_Nx[i], iso_Nx_data[i]);
    for (int i=0; i<ELLP_Dy_LEN; i++)  
        fp_read_raw(bls_prec->iso_Dy[i], iso_Dy_data[i]);
    for (int i=0; i<ELLP_Ny_LEN; i++)  
        fp_read_raw(bls_prec->iso_Ny[i], iso_Ny_data[i]);
    #endif
    #if (MEMBERSHIP_CHECK_G1 == BOWE)
    bn_read_raw(&bls_prec->beta, beta_data, Fp_DIGITS);
    bn_read_raw(&bls_prec->z2_1_by3, z2_1_by3_data, 2);
    #endif
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
void ep_mult(ep_t res, const ep_t p, const bn_t expo) {
    // Using window NAF of size 2
    #if (EP_MUL	== LWNAF)
        g1_mul(res, p, expo);
    #else 
        ep_mul_lwnaf(res, p, expo);
    #endif
}

// Exponentiation of generator g1 in G1
// This function is not called by BLS but is here for DEBUG/TESTs purposes
void ep_mult_gen(ep_t res, const bn_t expo) {
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
void ep2_mult(ep2_t res, ep2_t p, bn_t expo) {
    // Using window NAF of size 2
    #if (EP_MUL	== LWNAF)
        g2_mul(res, p, expo);
    #else 
        ep2_mul_lwnaf(res, p, expo);
    #endif
}

// Exponentiation of fixed g2 in G2
void ep2_mult_gen(ep2_t res, const bn_t expo) {
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

// DEBUG printing functions 
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
void bn_randZr_star(bn_t x) {
    int seed_len = BITS_TO_BYTES(Fr_BITS + SEC_BITS);
    byte* seed = (byte*) malloc(seed_len);
    rand_bytes(seed, seed_len);
    bn_map_to_Zr_star(x, seed, seed_len);
}

// generates a random number less than the order r
void bn_randZr(bn_t x) {
    bn_t r;
    bn_new(r); 
    g2_get_ord(r);

    bn_new_size(x, BITS_TO_DIGITS(Fr_BITS + SEC_BITS));
    bn_rand(x, RLC_POS, Fr_BITS + SEC_BITS);
    bn_mod(x, x, r);
    bn_free(r);
}

// reads a scalar from an array and maps it to Zr
// the resulting scalar is in the range 0 < a < r
// len must be less than BITS_TO_BYTES(RLC_BN_BITS)
void bn_map_to_Zr_star(bn_t a, const uint8_t* bin, int len) {
    bn_t tmp;
    bn_new(tmp);
    bn_new_size(tmp, BYTES_TO_DIGITS(len));
    bn_read_bin(tmp, bin, len);
    bn_t r;
    bn_new(r); 
    g2_get_ord(r);
    bn_sub_dig(r,r,1);
    bn_mod_basic(a,tmp,r);
    bn_add_dig(a,a,1);
    bn_free(r);
    bn_free(tmp);
}


// reads a bit in a prime field element at a given index
// whether the field element is in Montgomery domain or not
static int fp_get_bit_generic(const fp_t a, int bit) {
#if (FP_RDC == MONTY)
    bn_t tmp;
    bn_new(tmp);
    fp_prime_back(tmp, a);
    int res = bn_get_bit(tmp, bit);
    bn_free(tmp);
    return res;
#else
    return fp_get_bit(a, bit);
#endif
}

// uncompress a G1 point p into r taken into account the coordinate x
// and the LS bit of the y coordinate. 
//
// (taken and modifed from Relic ep_upk function)
// Change: the square root (y) is chosen regardless of the modular multiplication used.
// If montgomery multiplication is used, the square root is reduced to check the LS bit.
//
// Copyright 2019 D. F. Aranha and C. P. L. Gouvêa and T. Markmann and R. S. Wahby and K. Liao
// https://github.com/relic-toolkit/relic
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at

//        http://www.apache.org/licenses/LICENSE-2.0

//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.
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
            bn_t tmp;
            bn_new(tmp);
            fp_prime_back(tmp, t);
            if (bn_get_bit(tmp, 0) != fp_get_bit(p->y, 0)) {
                fp_neg(t, t);
            }
            bn_free(tmp);
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
//
// The most significant bit of the buffer, when set, indicates that the point is in compressed form. 
// Otherwise, the point is in uncompressed form.
// The second-most significant bit, when set, indicates that the point is at infinity. 
// If this bit is set, the remaining bits of the group element's encoding should be set to zero.
// The third-most significant bit is set if (and only if) this point is in compressed form and it is not the point at infinity and its y-coordinate is odd.
void ep_write_bin_compact(byte *bin, const ep_t a, const int len) {
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
// The code is a modified version of Relic ep_read_bin
int ep_read_bin_compact(ep_t a, const byte *bin, const int len) {
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
        return RLC_OK;
    }
    fp_zero(a->y);
    fp_set_bit(a->y, 0, y_is_odd);
    if (ep_upk_generic(a, a) == 1) {
        return RLC_OK;
    }
    return RLC_ERR;
}

// uncompress a G2 point p into r taking into account the coordinate x
// and the LS bit of the y lower coordinate.
//
// (taken and modifed from Relic ep2_upk function)
// Change: the square root (y) is chosen regardless of the modular multiplication used.
// If montgomery multiplication is used, the square root is reduced to check the LS bit.
//
// Copyright 2019 D. F. Aranha and C. P. L. Gouvêa and T. Markmann and R. S. Wahby and K. Liao
// https://github.com/relic-toolkit/relic
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
            bn_t tmp;
            bn_new(tmp);
            fp_prime_back(tmp, t[0]);
            if (bn_get_bit(tmp, 0) != fp_get_bit(p->y[0], 0)) {
                fp2_neg(t, t);
            }
            bn_free(tmp);
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
// The most significant bit of the buffer, when set, indicates that the point is in compressed form. 
// Otherwise, the point is in uncompressed form.
// The second-most significant bit indicates that the point is at infinity. 
// If this bit is set, the remaining bits of the group element's encoding should be set to zero.
// The third-most significant bit is set if (and only if) this point is in compressed form and it is not the point at infinity and its y-coordinate is odd.
void ep2_write_bin_compact(byte *bin, const ep2_t a, const int len) {
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
// The code is a modified version of Relic ep_read_bin
int ep2_read_bin_compact(ep2_t a, const byte *bin, const int len) {
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
        return RLC_OK;
    }
    
    fp2_zero(a->y);
    fp_set_bit(a->y[0], 0, y_is_odd);
    fp_zero(a->y[1]);
    if (ep2_upk_generic(a, a) == 1) {
        return RLC_OK;
    }
    return RLC_ERR;
}

// computes the sum of the array elements x and writes the sum in jointx
// the sum is computed in Zr
void bn_sum_vector(bn_t jointx, bn_st* x, int len) {
    bn_t r;
    bn_new(r); 
    g2_get_ord(r);
    bn_set_dig(jointx, 0);
    bn_new_size(jointx, BITS_TO_DIGITS(Fr_BITS+1));
    for (int i=0; i<len; i++) {
        bn_add(jointx, jointx, &x[i]);
        if (bn_cmp(jointx, r) == RLC_GT) 
            bn_sub(jointx, jointx, r);
    }
    bn_free(r);
}

// computes the sum of the G2 array elements y and writes the sum in jointy
void ep2_sum_vector(ep2_t jointy, ep2_st* y, int len){
    ep2_set_infty(jointy);
    for (int i=0; i<len; i++){
        ep2_add_projc(jointy, jointy, &y[i]);
    }
}

// Subtracts the sum of a G2 array elements y from an element x and writes the 
// result in res
void ep2_subtract_vector(ep2_t res, ep2_t x, ep2_st* y, int len){
    ep2_sum_vector(res, y, len);
    ep2_neg(res, res);
    ep2_add_projc(res, x, res);
}

// Computes the sum of the signatures (G1 elements) flattened in an single sigs array
// and store the sum bytes in dest
// The function assmues sigs is correctly allocated with regards to len.
int ep_sum_vector(byte* dest, const byte* sigs, const int len) {
    // temp variables
    ep_t acc, sig;        
    ep_new(acc);
    ep_new(sig);
    ep_set_infty(acc);

    // sum the points
    for (int i=0; i < len; i++) {
        if (ep_read_bin_compact(sig, &sigs[SIGNATURE_LEN*i], SIGNATURE_LEN) != RLC_OK)
            return INVALID;
        ep_add_projc(acc, acc, sig);
    }
    // export the result
    ep_write_bin_compact(dest, acc, SIGNATURE_LEN);

    // free the temp memory
    ep_free(acc);
    ep_free(sig);
    return VALID;
}

// uses a simple scalar multiplication by G1's order
// to check whether a point on the curve E1 is in G1.
int simple_subgroup_check_G1(const ep_t p){
    ep_t inf;
    ep_new(inf);
    // check p^order == infinity
    // use basic double & add as lwnaf reduces the expo modulo r
    ep_mul_basic(inf, p, &core_get()->ep_r);
    if (!ep_is_infty(inf)){
        ep_free(inf);
        return INVALID;
    }
    ep_free(inf);
    return VALID;
}

// uses a simple scalar multiplication by G1's order
// to check whether a point on the curve E2 is in G2.
int simple_subgroup_check_G2(const ep2_t p){
    ep2_t inf;
    ep2_new(inf);
    // check p^order == infinity
    // use basic double & add as lwnaf reduces the expo modulo r
    ep2_mul_basic(inf, (ep2_st*)p, &core_get()->ep_r);
    if (!ep2_is_infty(inf)){
        ep2_free(inf);
        return INVALID;
    }
    ep2_free(inf);
    return VALID;
}

#if (MEMBERSHIP_CHECK_G1 == BOWE)
// beta such that beta^3 == 1 mod p
// beta is in the Montgomery form
const uint64_t beta_data[Fp_DIGITS] = { 
    0xcd03c9e48671f071, 0x5dab22461fcda5d2, 0x587042afd3851b95,
    0x8eb60ebe01bacb9e, 0x03f97d6e83d050d2, 0x18f0206554638741,
};


// (z^2-1)/3 with z being the parameter of bls12-381
const uint64_t z2_1_by3_data[2] = { 
    0x0000000055555555, 0x396c8c005555e156  
};

// uses Bowe's check from section 3.2 from https://eprint.iacr.org/2019/814.pdf
// to check whether a point on the curve E1 is in G1.
int bowe_subgroup_check_G1(const ep_t p){
    if (ep_is_infty(p) == 1) 
        return VALID;
    fp_t b;
    fp_new(b);
    dv_copy(b, beta_data, Fp_DIGITS); 
    ep_t sigma, sigma2, p_inv;
    ep_new(sigma);
    ep_new(sigma2);
    ep_new(p_inv);

    // si(p) 
    ep_copy(sigma, p);
    fp_mul(sigma[0].x, sigma[0].x, b);
    // -si^2(p)
    ep_copy(sigma2, sigma);
    fp_mul(sigma2[0].x, sigma2[0].x, b);
    fp_neg(sigma2[0].y, sigma2[0].y);
    ep_dbl(sigma, sigma);
    // -p
    ep_copy(p_inv, p);
    fp_neg(p_inv[0].y, p_inv[0].y);
    // (z^2-1)/3 (2*si(p) - p - si^2(p)) - si^2(p)
    ep_add(sigma, sigma, p_inv);
    ep_add(sigma, sigma, sigma2);
    // TODO: multiplication using a chain?
    ep_mul_lwnaf(sigma, sigma, &bls_prec->z2_1_by3);
    ep_add(sigma, sigma, sigma2);
    
    ep_free(sigma2);
    ep_free(p_inv);
    // check result against infinity
    if (!ep_is_infty(sigma)){
        ep_free(sigma);
        return INVALID;
    }
    ep_free(sigma);
    return VALID;
}
#endif

// generates a random point in G1 and stores it in p
void ep_rand_G1(ep_t p) {
    // multiplies G1 generator by a random scalar
    ep_rand(p);
}

// generates a random point in E1\G1 and stores it in p
void ep_rand_G1complement(ep_t p) {
    // generate a random point in E1
    p->norm = 1;
    fp_set_dig(p->z, 1);
    do {
        fp_rand(p->x); // set x to a random field element
        byte r;
        rand_bytes(&r, 1);
        fp_zero(p->y);
        fp_set_bit(p->y, 0, r&1); // set y randomly to 0 or 1
    }
    while (ep_upk_generic(p, p) == 0); // make sure p is in E1

    // map the point to E1\G1 by clearing G1 order
    bn_t order;
    bn_new(order); 
    g1_get_ord(order);
    ep_mul_basic(p, p, order);
}

int subgroup_check_G1_test(int inG1, int method) {
    // generate a random p
    ep_t p;
	if (inG1) ep_rand_G1(p); // p in G1
	else ep_rand_G1complement(p); // p in E1\G1

    if (!ep_is_valid(p)) { // sanity check to make sure p is in E1
        return UNDEFINED; // this should not happen
    }
        
    switch (method) {
    case EXP_ORDER: 
        return simple_subgroup_check_G1(p);
    #if  (MEMBERSHIP_CHECK_G1 == BOWE)
    case BOWE:
        return bowe_subgroup_check_G1(p);
    #endif
    default:
        if (inG1) return VALID;
        else return INVALID;
    }
}

int subgroup_check_G1_bench() {
    ep_t p;
	ep_curve_get_gen(p);
    int res;
    // check p is in G1
    #if MEMBERSHIP_CHECK_G1 == EXP_ORDER
    res = simple_subgroup_check_G1(p);
    #elif MEMBERSHIP_CHECK_G1 == BOWE
    res = bowe_subgroup_check_G1(p);
    #endif
    return res;
}