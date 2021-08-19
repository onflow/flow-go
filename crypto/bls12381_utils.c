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

void bn_new_wrapper(bn_t a) {
    bn_new(a);
}

// global variable of the pre-computed data
prec_st bls_prec_st;
prec_st* bls_prec = NULL;

// required constants for the optimized SWU hash to curve
#if (hashToPoint == OPSWU)
extern const uint64_t p_3div4_data[Fp_DIGITS];
extern const uint64_t fp_p_1div2_data[Fp_DIGITS];
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

const uint64_t p_1div2_data[Fp_DIGITS] = {
   0xdcff7fffffffd555, 0x0f55ffff58a9ffff, 0xb39869507b587b12, 
   0xb23ba5c279c2895f, 0x258dd3db21a5d66b, 0x0d0088f51cbff34d,
};


// sets the global variable to input
void precomputed_data_set(const prec_st* p) {
    bls_prec = (prec_st*)p;
}

// Reads a prime field element from a digit vector in big endian format.
// There is no conversion to Montgomery domain in this function.
 #define fp_read_raw(a, data_pointer) dv_copy((a), (data_pointer), Fp_DIGITS)

// pre-compute some data required for curve BLS12-381
prec_st* init_precomputed_data_BLS12_381() {
    bls_prec = &bls_prec_st;

    #if (hashToPoint == OPSWU)
    // isogenous curve constants used in optimized SWU
    fp_read_raw(bls_prec->a1, a1_data);
    fp_read_raw(bls_prec->b1, b1_data);
    // (p-3)/4
    bn_new(&bls_prec->p_3div4);
    bn_read_raw(&bls_prec->p_3div4, p_3div4_data, Fp_DIGITS);
    // (p-1)/2
    fp_read_raw(bls_prec->fp_p_1div2, fp_p_1div2_data);
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
    bn_new(&bls_prec->beta);
    bn_read_raw(&bls_prec->beta, beta_data, Fp_DIGITS);
    bn_new(&bls_prec->z2_1_by3);
    bn_read_raw(&bls_prec->z2_1_by3, z2_1_by3_data, 2);
    #endif

    bn_new(&bls_prec->p_1div2);
    bn_read_raw(&bls_prec->p_1div2, p_1div2_data, Fp_DIGITS);
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
    ep_mul_lwnaf(res, p, expo);
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
    ep_mul_gen(res, (bn_st *)expo);
#endif
}

// Exponentiation of a generic point p in G2
void ep2_mult(ep2_t res, ep2_t p, bn_t expo) {
    // Using window NAF of size 2
    ep2_mul_lwnaf(res, p, expo);
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
    // reduce the modular reduction bias
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
    // reduce the modular reduction bias
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

// returns the sign of y.
// 1 if y > (p - 1)/2 and 0 otherwise.
static int fp_get_sign(const fp_t y) {
    bn_t bn_y;
    bn_new(bn_y);
    fp_prime_back(bn_y, y);
    return bn_cmp(bn_y, &bls_prec->p_1div2) == RLC_GT;		
}

// ep_write_bin_compact exports a point a in E(Fp) to a buffer bin in a compressed or uncompressed form.
// len is the allocated size of the buffer bin.
// The serialization is following:
// https://www.ietf.org/archive/id/draft-irtf-cfrg-pairing-friendly-curves-08.html#name-zcash-serialization-format-) 
// The code is a modified version of Relic ep_write_bin
// It returns RLC_OK if the inputs are valid and the execution completes, RLC_ERR if any of
// the inputs is invalid, and UNDEFINED if an unexpected execution error happens.
void ep_write_bin_compact(byte *bin, const ep_t a, const int len) {
    ep_t t;
    ep_null(t);
    const int G1_size = (G1_BYTES/(G1_SERIALIZATION+1));

    if (len!=G1_size) {
        RLC_THROW(ERR_NO_BUFFER);
        return;
    }
 
    if (ep_is_infty(a)) {
            // set the infinity bit
            bin[0] = (G1_SERIALIZATION << 7) | 0x40;
            memset(bin+1, 0, G1_size-1);
            return;
    }

    RLC_TRY {
        ep_new(t); 
        ep_norm(t, a);
        fp_write_bin(bin, Fp_BYTES, t->x);

        if (G1_SERIALIZATION == COMPRESSED) {
            bin[0] |= (fp_get_sign(t->y) << 5);
        } else {
            fp_write_bin(bin + Fp_BYTES, Fp_BYTES, t->y);
        }
    } RLC_CATCH_ANY {
        RLC_THROW(ERR_CAUGHT);
    }

    bin[0] |= (G1_SERIALIZATION << 7);
    ep_free(t);
 }


// ep_read_bin_compact imports a point from a buffer in a compressed or uncompressed form.
// len is the size of the input buffer.
// The serialization is following:
// https://www.ietf.org/archive/id/draft-irtf-cfrg-pairing-friendly-curves-08.html#name-zcash-serialization-format-) 
// The code is a modified version of Relic ep_read_bin
// It returns RLC_OK if the inputs are valid and the execution completes, RLC_ERR if any of
// the inputs is invalid, and UNDEFINED if an unexpected execution error happens.
int ep_read_bin_compact(ep_t a, const byte *bin, const int len) {
    // check the length
    const int G1_size = (G1_BYTES/(G1_SERIALIZATION+1));
    if (len!=G1_size) {
        return RLC_ERR;
    }

    // check the compression bit
    int compressed = bin[0] >> 7;
    if ((compressed == 1) != (G1_SERIALIZATION == COMPRESSED)) {
        return RLC_ERR;
    } 

    // check if the point is infinity
    int is_infinity = bin[0] & 0x40;
    if (is_infinity) {
        // check if the remaining bits are cleared
        if (bin[0] & 0x3F) {
            return RLC_ERR;
        }
        for (int i=1; i<G1_size-1; i++) {
            if (bin[i]) {
                return RLC_ERR;
            } 
        }
		ep_set_infty(a);
		return RLC_OK;
	} 

    // read the sign bit and check for consistency
    int y_sign = (bin[0] >> 5) & 1;
    if (y_sign && (!compressed)) {
        return RLC_ERR;
    } 

	a->coord = BASIC;
	fp_set_dig(a->z, 1);
    byte* temp = (byte*)malloc(Fp_BYTES);
    if (!temp) {
        RLC_THROW(ERR_NO_MEMORY);
        return UNDEFINED;
    }
    memcpy(temp, bin, Fp_BYTES);
    temp[0] &= 0x1F;
	fp_read_bin(a->x, temp, Fp_BYTES);
    free(temp);

    if (G1_SERIALIZATION == UNCOMPRESSED) {
        fp_read_bin(a->y, bin + Fp_BYTES, Fp_BYTES);
        return RLC_OK;
    }
    fp_zero(a->y);
    fp_set_bit(a->y, 0, y_sign);
    if (ep_upk(a, a) == 1) {
        return RLC_OK;
    }
    return RLC_ERR;
}


// returns the sign of y.
// sign(y_0) if y_1 = 0, else sign(y_1)
static int fp2_get_sign(fp2_t y) {
    if (fp_is_zero(y[1])) { // no need to convert back as the montgomery form of 0 is 0
        return fp_get_sign(y[0]);
    }
    return fp_get_sign(y[1]);
}

// ep2_write_bin_compact exports a point in E(Fp^2) to a buffer in a compressed or uncompressed form.
// len is the allocated size of the buffer bin.
// The serialization is following:
// https://www.ietf.org/archive/id/draft-irtf-cfrg-pairing-friendly-curves-08.html#name-zcash-serialization-format-)
// The code is a modified version of Relic ep2_write_bin 
void ep2_write_bin_compact(byte *bin, const ep2_t a, const int len) {
    ep2_t t;
    ep2_null(t);
    const int G2_size = (G2_BYTES/(G2_SERIALIZATION+1));

    if (len!=G2_size) {
        RLC_THROW(ERR_NO_BUFFER);
        return;
    }
 
    if (ep2_is_infty((ep2_st *)a)) {
            // set the infinity bit
            bin[0] = (G2_SERIALIZATION << 7) | 0x40;
            memset(bin+1, 0, G2_size-1);
            return;
    }

    RLC_TRY {
        ep2_new(t);
        ep2_norm(t, (ep2_st *)a);
        fp2_write_bin(bin, 2*Fp_BYTES, t->x, 0);

        if (G2_SERIALIZATION == COMPRESSED) {
            bin[0] |= (fp2_get_sign(t->y) << 5);
        } else {
            fp2_write_bin(bin + 2*Fp_BYTES, 2*Fp_BYTES, t->y, 0);
        }
    } RLC_CATCH_ANY {
        RLC_THROW(ERR_CAUGHT);
    }

    bin[0] |= (G2_SERIALIZATION << 7);
    ep_free(t);
}

// ep2_read_bin_compact imports a point from a buffer in a compressed or uncompressed form.
// It returns RLC_OK if the inputs are valid and the execution completes, RLC_ERR if any of
// the inputs is invalid, and UNDEFINED if an unexpected execution error happens.
// The code is a modified version of Relic ep2_read_bin
int ep2_read_bin_compact(ep2_t a, const byte *bin, const int len) {
    // check the length
    const int G2size = (G2_BYTES/(G2_SERIALIZATION+1));
    if (len!=G2size) {
        return RLC_ERR;
    }

    // check the compression bit
    int compressed = bin[0] >> 7;
    if ((compressed == 1) != (G2_SERIALIZATION == COMPRESSED)) {
        return RLC_ERR;
    } 

    // check if the point in infinity
    int is_infinity = bin[0] & 0x40;
    if (is_infinity) {
        // the remaining bits need to be cleared
        if (bin[0] & 0x3F) {
            return RLC_ERR;
        }
        for (int i=1; i<G2size-1; i++) {
            if (bin[i]) {
                return RLC_ERR;
            } 
        }
		ep2_set_infty(a);
		return RLC_OK;
	} 

    // read the sign bit and check for consistency
    int y_sign = (bin[0] >> 5) & 1;
    if (y_sign && (!compressed)) {
        return RLC_ERR;
    } 
    
	a->coord = BASIC;
	fp_set_dig(a->z[0], 1);
	fp_zero(a->z[1]);
    byte* temp = (byte*)malloc(2*Fp_BYTES);
    if (!temp) {
        RLC_THROW(ERR_NO_MEMORY);
        return UNDEFINED;
    }
    memcpy(temp, bin, 2*Fp_BYTES);
    // clear the header bits
    temp[0] &= 0x1F;
    fp2_read_bin(a->x, temp, 2*Fp_BYTES);
    free(temp);

    if (G2_SERIALIZATION == UNCOMPRESSED) {
        fp2_read_bin(a->y, bin + 2*Fp_BYTES, 2*Fp_BYTES);
        return RLC_OK;
    }
    
    fp2_zero(a->y);
    fp_set_bit(a->y[0], 0, y_sign);
    fp_zero(a->y[1]);
    if (ep2_upk(a, a) == 1) {
        return RLC_OK;
    }
    return RLC_ERR;
}

// reads a scalar in a and checks it is a valid Zr element (a < r)
// returns RLC_OK if the scalar is valid and RLC_ERR otherwise.
int bn_read_Zr_bin(bn_t a, const uint8_t *bin, int len) {
    if (len!=Fr_BYTES) {
        return RLC_ERR;
    }
    bn_read_bin(a, bin, Fr_BYTES);
    bn_t r;
    bn_new(r); 
    g2_get_ord(r);
    if (bn_cmp(a, r) == RLC_LT) {
        return RLC_OK;
    }
    return RLC_ERR;
}

// computes the sum of the array elements x and writes the sum in jointx
// the sum is computed in Zr
void bn_sum_vector(bn_t jointx, const bn_st* x, const int len) {
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
void ep2_sum_vector(ep2_t jointy, ep2_st* y, const int len){
    ep2_set_infty(jointy);
    for (int i=0; i<len; i++){
        ep2_add_projc(jointy, jointy, &y[i]);
    }
    ep2_norm(jointy, jointy); // not necessary but left here to optimize the 
                            // multiple pairing computations with the same 
                            // public key
}

// Verifies the validity of 2 SPoCK proofs and 2 public keys.
// Membership check in G1 of both proofs is verified in this function.
// Membership check in G2 of both keys is not verified in this function.
// the membership check in G2 is separated to allow optimizing multiple verifications 
// using the same public keys.
int bls_spock_verify(const ep2_t pk1, const byte* sig1, const ep2_t pk2, const byte* sig2) {  
    ep_t elemsG1[2];
    ep2_t elemsG2[2];

    // elemsG1[0] = s1
    ep_new(elemsG1[0]);
    int read_ret = ep_read_bin_compact(elemsG1[0], sig1, SIGNATURE_LEN);
    if (read_ret != RLC_OK) 
        return read_ret;

    // check s1 is on curve and in G1
    if (check_membership_G1(elemsG1[0]) != VALID) // only enabled if MEMBERSHIP_CHECK==1
        return INVALID;

    // elemsG1[1] = s2
    ep_new(elemsG1[1]);
    read_ret = ep_read_bin_compact(elemsG1[1], sig2, SIGNATURE_LEN);
    if (read_ret != RLC_OK) 
        return read_ret;

    // check s2 is on curve and in G1
    if (check_membership_G1(elemsG1[1]) != VALID) // only enabled if MEMBERSHIP_CHECK==1
        return INVALID; 

    // elemsG2[1] = pk1
    ep2_new(elemsG2[1]);
    ep2_copy(elemsG2[1], (ep2_st*)pk1);

    // elemsG2[0] = pk2
    ep2_new(elemsG2[0]);
    ep2_copy(elemsG2[0], (ep2_st*)pk2);

#if DOUBLE_PAIRING  
    // elemsG2[0] = -pk2
    ep2_neg(elemsG2[0], elemsG2[0]);

    fp12_t pair;
    fp12_new(&pair);
    // double pairing with Optimal Ate 
    pp_map_sim_oatep_k12(pair, (ep_t*)(elemsG1) , (ep2_t*)(elemsG2), 2);

    // compare the result to 1
    int res = fp12_cmp_dig(pair, 1);

#elif SINGLE_PAIRING   
    fp12_t pair1, pair2;
    fp12_new(&pair1); fp12_new(&pair2);
    pp_map_oatep_k12(pair1, elemsG1[0], elemsG2[0]);
    pp_map_oatep_k12(pair2, elemsG1[1], elemsG2[1]);

    int res = fp12_cmp(pair1, pair2);
#endif
    fp12_free(&one);
    ep_free(elemsG1[0]);
    ep_free(elemsG1[1]);
    ep2_free(elemsG2[0]);
    ep2_free(elemsG2[1]);
    
    if (res == RLC_EQ && core_get()->code == RLC_OK) 
        return VALID;
    else 
        return INVALID;
}

// Subtracts the sum of a G2 array elements y from an element x and writes the 
// result in res
void ep2_subtract_vector(ep2_t res, ep2_t x, ep2_st* y, const int len){
    ep2_sum_vector(res, y, len);
    ep2_neg(res, res);
    ep2_add_projc(res, x, res);
}

// computes the sum of the G1 array elements y and writes the sum in jointy
void ep_sum_vector(ep_t jointx, ep_st* x, const int len) {
    ep_set_infty(jointx);
    for (int i=0; i<len; i++){
        ep_add_jacob(jointx, jointx, &x[i]);
    }
}

// Computes the sum of the signatures (G1 elements) flattened in a single sigs array
// and store the sum bytes in dest.
// The function assumes sigs is correctly allocated with regards to len.
int ep_sum_vector_byte(byte* dest, const byte* sigs_bytes, const int len) {
    // temp variables
    ep_t acc;        
    ep_new(acc);
    ep_set_infty(acc);
    ep_st* sigs = (ep_st*) malloc(len * sizeof(ep_st));

    // import the points from the array
    for (int i=0; i < len; i++) {
        ep_new(sigs[i]);
        // deserialize each point from the input array
        int read_ret = ep_read_bin_compact(&sigs[i], &sigs_bytes[SIGNATURE_LEN*i], SIGNATURE_LEN);
        if (read_ret != RLC_OK)
            return read_ret;
    }
    // sum the points
    ep_sum_vector(acc, sigs, len);
    // export the result
    ep_write_bin_compact(dest, acc, SIGNATURE_LEN);

    // free the temp memory
    ep_free(acc);
    for (int i=0; i < len; i++) ep_free(sig[i]);
    free(sigs);
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
    p->coord = BASIC;
    fp_set_dig(p->z, 1);
    do {
        fp_rand(p->x); // set x to a random field element
        byte r;
        rand_bytes(&r, 1);
        fp_zero(p->y);
        fp_set_bit(p->y, 0, r&1); // set y randomly to 0 or 1
    }
    while (ep_upk(p, p) == 0); // make sure p is in E1

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

    if (!ep_on_curve(p)) { // sanity check to make sure p is in E1
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

// This is a testing function.
// It wraps a call to a Relic macro since cgo can't call macros.
void xmd_sha256(uint8_t *hash, int len_hash, uint8_t *msg, int len_msg, uint8_t *dst, int len_dst){
    md_xmd_sh256(hash, len_hash, msg, len_msg, dst, len_dst);
}
