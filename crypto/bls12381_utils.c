// +build relic

// this file contains utility functions for the curve BLS 12-381
// these tools are shared by the BLS signature scheme, the BLS based threshold signature
// and the BLS distributed key generation protocols

#include "bls12381_utils.h"
#include "bls_include.h"
#include "assert.h"

#include "blst_src.c"

// The functions are tested for ALLOC=AUTO (not for ALLOC=DYNAMIC)

// return macro values to the upper Go Layer
int get_valid() {
    return VALID;
}

int get_invalid() {
    return INVALID;
}

int get_Fr_BYTES() {
    return Fr_BYTES;
}


// Initializes Relic context with BLS12-381 parameters
ctx_t* relic_init_BLS12_381() { 
    // check Relic was compiled with the right conf 
    assert(ALLOC == AUTO);

    // sanity check of Relic constants the package is relying on
    assert(RLC_OK == RLC_EQ);

    // initialize relic core with a new context
    ctx_t* bls_ctx = (ctx_t*) calloc(1, sizeof(ctx_t));
    if (!bls_ctx) return NULL;
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

// global variable of the pre-computed data
prec_st bls_prec_st;
prec_st* bls_prec = NULL;

// required constants for the optimized SWU hash to curve
#if (hashToPoint == LOCAL_SSWU)
extern const uint64_t iso_Nx_data[ELLP_Nx_LEN][Fp_LIMBS];
extern const uint64_t iso_Ny_data[ELLP_Ny_LEN][Fp_LIMBS];
#endif

#if (MEMBERSHIP_CHECK_G1 == BOWE)
extern const uint64_t beta_data[Fp_LIMBS];
extern const uint64_t z2_1_by3_data[2];
#endif

// sets the global variable to input
void precomputed_data_set(const prec_st* p) {
    bls_prec = (prec_st*)p;
}

// Reads a prime field element from a digit vector in big endian format.
// There is no conversion to Montgomery domain in this function.
#define fp_read_raw(a, data_pointer) dv_copy((a), (data_pointer), Fp_LIMBS)

// pre-compute some data required for curve BLS12-381
prec_st* init_precomputed_data_BLS12_381() {

    bls_prec = &bls_prec_st;
    ctx_t* ctx = core_get();

    // (p-1)/2
    bn_div_dig(&bls_prec->p_1div2, &ctx->prime, 2);
    #if (hashToPoint == LOCAL_SSWU)
    // (p-3)/4
    bn_div_dig(&bls_prec->p_3div4, &bls_prec->p_1div2, 2);
    // sqrt(-z)
    fp_neg(bls_prec->sqrt_z, ctx->ep_map_u);
    fp_srt(bls_prec->sqrt_z, bls_prec->sqrt_z);
    // -a1 and a1*z
    fp_neg(bls_prec->minus_a1, ctx->ep_iso.a);
    fp_mul(bls_prec->a1z, ctx->ep_iso.a, ctx->ep_map_u);
    
    for (int i=0; i<ELLP_Nx_LEN; i++)  
        fp_read_raw(bls_prec->iso_Nx[i], iso_Nx_data[i]);
    for (int i=0; i<ELLP_Ny_LEN; i++)  
        fp_read_raw(bls_prec->iso_Ny[i], iso_Ny_data[i]);
    #endif

    #if (MEMBERSHIP_CHECK_G1 == BOWE)
    bn_new(&bls_prec->beta);
    bn_read_raw(&bls_prec->beta, beta_data, Fp_LIMBS);
    bn_new(&bls_prec->z2_1_by3);
    bn_read_raw(&bls_prec->z2_1_by3, z2_1_by3_data, 2);
    #endif

    // Montgomery constant R
    fp_set_dig(bls_prec->r, 1);
    return bls_prec;
}

// ------------------- Fr utilities

// Montgomery constant R related to the curve order r
const limb_t BLS12_381_rR[Fr_LIMBS] = {  /* (1<<256)%r */
    TO_LIMB_T(0x1824b159acc5056f), TO_LIMB_T(0x998c4fefecbc4ff5),
    TO_LIMB_T(0x5884b7fa00034802), TO_LIMB_T(0x00000001fffffffe)
};

// TODO: temp utility function to delete
bn_st* Fr_blst_to_relic(const Fr* x) {
    bn_st* out = (bn_st*)malloc(sizeof(bn_st)); 
    byte* data = (byte*)malloc(Fr_BYTES);
    be_bytes_from_limbs(data, (limb_t*)x, Fr_BYTES);
    out->alloc = RLC_DV_DIGS;
    bn_read_bin(out, data, Fr_BYTES);
    free(data);
    return out;
}

// TODO: temp utility function to delete
Fr* Fr_relic_to_blst(const bn_st* x){
    Fr* out = (Fr*)malloc(sizeof(Fr)); 
    byte* data = (byte*)malloc(Fr_BYTES);
    bn_write_bin(data, Fr_BYTES, x);   
    Fr_read_bytes(out, data, Fr_BYTES);
    free(data);
    return out;
}

// returns true if a == 0 and false otherwise
bool_t Fr_is_zero(const Fr* a) {
    return bytes_are_zero((const byte*)a, Fr_BYTES);
}

// returns true if a == b and false otherwise
bool_t Fr_is_equal(const Fr* a, const Fr* b) {
    return vec_is_equal(a, b, Fr_BYTES);
}

// sets `a` to limb `l`
void Fr_set_limb(Fr* a, const limb_t l){
    vec_zero((byte*)a + sizeof(limb_t), Fr_BYTES - sizeof(limb_t));
    *((limb_t*)a) = l;
}

void Fr_copy(Fr* res, const Fr* a) {
    vec_copy((byte*)res, (byte*)a, sizeof(Fr));
}

// sets `a` to 0
void Fr_set_zero(Fr* a){
    vec_zero((byte*)a, sizeof(Fr));
}

void Fr_add(Fr *res, const Fr *a, const Fr *b) {
    add_mod_256((limb_t*)res, (limb_t*)a, (limb_t*)b, BLS12_381_r);
}

void Fr_sub(Fr *res, const Fr *a, const Fr *b) {
    sub_mod_256((limb_t*)res, (limb_t*)a, (limb_t*)b, BLS12_381_r);
}

void Fr_neg(Fr *res, const Fr *a) {
    cneg_mod_256((limb_t*)res, (limb_t*)a, 1, BLS12_381_r);
}

// res = a*b*R^(-1)
void Fr_mul_montg(Fr *res, const Fr *a, const Fr *b) {
    mul_mont_sparse_256((limb_t*)res, (limb_t*)a, (limb_t*)b, BLS12_381_r, r0);
}

// res = a^2 * R^(-1)
void Fr_squ_montg(Fr *res, const Fr *a) {
    sqr_mont_sparse_256((limb_t*)res, (limb_t*)a, BLS12_381_r, r0);
}

// res = a*R
void Fr_to_montg(Fr *res, const Fr *a) {
    mul_mont_sparse_256((limb_t*)res, (limb_t*)a, BLS12_381_rRR, BLS12_381_r, r0);
}

// res = a*R^(-1)
void Fr_from_montg(Fr *res, const Fr *a) {
    from_mont_256((limb_t*)res, (limb_t*)a, BLS12_381_r, r0);
}

// res = a^(-1)*R
void Fr_inv_montg_eucl(Fr *res, const Fr *a) {
    // copied and modified from BLST code
    // Copyright Supranational LLC
    static const vec256 rx2 =   { /* left-aligned value of the modulus */
        TO_LIMB_T(0xfffffffe00000002), TO_LIMB_T(0xa77b4805fffcb7fd),
        TO_LIMB_T(0x6673b0101343b00a), TO_LIMB_T(0xe7db4ea6533afa90),
    };
    vec512 temp;
    ct_inverse_mod_256(temp, (limb_t*)a, BLS12_381_r, rx2);
    redc_mont_256((limb_t*)res, temp, BLS12_381_r, r0);
}

// result is in Montgomery form if base is in montgomery form
// if base = b*R, res = b^expo * R
// In general, res = base^expo * R^(-expo+1)
// `expo` is encoded as a little-endian limb_t table of length `expo_len`.
void Fr_exp_montg(Fr *res, const Fr* base, const limb_t* expo, const int expo_len) {
    // mask of the most significant bit
	const limb_t msb_mask =  (limb_t)1<<((sizeof(limb_t)<<3)-1);
	limb_t mask = msb_mask;
	int index = 0;

    expo += expo_len;
	// Treat most significant zero limbs
	while((index < expo_len) && (*(--expo) == 0)) {
		index++;
    }
	// Treat the most significant zero bits
	while((*expo & mask) == 0) {
		mask >>= 1;
    }
	// Treat the first `1` bit
	Fr_copy(res, base);
	mask >>= 1;
	// Scan all limbs of the exponent
	for ( ; index < expo_len; expo--) {
		// Scan all bits 
		for ( ; mask != 0 ; mask >>= 1 ) {
			// square
			Fr_squ_montg(res, res);
			// multiply
			if (*expo & mask) {
				Fr_mul_montg(res, res ,base);
			}
		}
		mask = msb_mask;
        index++;
	}
}

void Fr_inv_exp_montg(Fr *res, const Fr *a) {
    Fr r_2;
    Fr_copy(&r_2, (Fr*)BLS12_381_r);
    r_2.limbs[0] -= 2;
    Fr_exp_montg(res, a, (limb_t*)&r_2, 4);
}

// computes the sum of the array elements and writes the sum in jointx
void Fr_sum_vector(Fr* jointx, const Fr x[], const int len) {
    Fr_set_zero(jointx);
    for (int i=0; i<len; i++) {
        Fr_add(jointx, jointx, &x[i]);
    }
}

// internal type of BLST `pow256` uses bytes little endian.
// input is bytes big endian as used by Flow crypto lib external scalars.
static void pow256_from_be_bytes(pow256 ret, const unsigned char a[Fr_BYTES])
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

// internal type of BLST `pow256` uses bytes little endian.
static void pow256_from_Fr(pow256 ret, const Fr* in) {
    le_bytes_from_limbs(ret, (limb_t*)in, Fr_BYTES);
}

// reads a scalar in `a` and checks it is a valid Fr element (a < r).
// input is bytes-big-endian.
// returns:
//    - BLST_BAD_ENCODING if the length is invalid
//    - BLST_BAD_SCALAR if the scalar isn't in Fr
//    - BLST_SUCCESS if the scalar is valid 
BLST_ERROR Fr_read_bytes(Fr* a, const uint8_t *bin, int len) {
    if (len != Fr_BYTES) {
        return BLST_BAD_ENCODING;
    }
    pow256 tmp;
    // compare to r using the provided tool from BLST 
    pow256_from_be_bytes(tmp, bin);
    if (!check_mod_256(tmp, BLS12_381_r)) {  // check_mod_256 compares pow256 against a vec256!
        return BLST_BAD_SCALAR;
    }
    vec_zero(tmp, sizeof(tmp));
    limbs_from_be_bytes((limb_t*)a, bin, Fr_BYTES);
    return BLST_SUCCESS;
}

// reads a scalar in `a` and checks it is a valid Fr_star element (0 < a < r).
// input bytes are big endian.
// returns:
//    - BLST_BAD_ENCODING if the length is invalid
//    - BLST_BAD_SCALAR if the scalar isn't in Fr_star
//    - BLST_SUCCESS if the scalar is valid 
BLST_ERROR Fr_star_read_bytes(Fr* a, const uint8_t *bin, int len) {
    int ret = Fr_read_bytes(a, bin, len);
    if (ret != BLST_SUCCESS) {
        return ret;
    }
    // check if a=0
    if (Fr_is_zero(a)) {
        return BLST_BAD_SCALAR;
    }
    return BLST_SUCCESS;
}

// write Fr element `a` in big endian bytes.
void Fr_write_bytes(uint8_t *bin, const Fr* a) {
    be_bytes_from_limbs(bin, (limb_t*)a, Fr_BYTES);
}

// maps big-endian bytes into an Fr element using modular reduction
// Input is byte-big-endian, output is vec256 (also used as Fr)
static void vec256_from_be_bytes(Fr* out, const unsigned char *bytes, size_t n)
{
    Fr digit, radix;
    Fr_set_zero(out);
    Fr_copy(&radix, (Fr*)BLS12_381_rRR); // R^2

    bytes += n;
    while (n > Fr_BYTES) {
        limbs_from_be_bytes((limb_t*)&digit, bytes -= Fr_BYTES, Fr_BYTES); // l_i
        Fr_mul_montg(&digit, &digit, &radix); // l_i * R^i  (i is the loop number starting at 1)
        Fr_add(out, out, &digit);
        Fr_mul_montg(&radix, &radix, (Fr*)BLS12_381_rRR); // R^(i+1)
        n -= Fr_BYTES;
    }
    Fr_set_zero(&digit);
    limbs_from_be_bytes((limb_t*)&digit, bytes -= n, n);
    Fr_mul_montg(&digit, &digit, &radix);
    Fr_add(out, out, &digit);
    // at this point : out = l_1*R + L_2*R^2 .. + L_n*R^n
    // reduce the extra R
    Fr_from_montg(out, out);
    // clean up possible sensitive data
    Fr_set_zero(&digit);
}

// Reads a scalar from an array and maps it to Fr using modular reduction.
// Input is byte-big-endian as used by the external APIs.
// It returns true if scalar is zero and false otherwise.
bool map_bytes_to_Fr(Fr* a, const uint8_t* bin, int len) {
    vec256_from_be_bytes(a, bin, len);
    return Fr_is_zero(a);
}

// ------------------- Fp utilities

// Montgomery constant R related to the prime p
const limb_t BLS12_381_pR[Fp_LIMBS] = { ONE_MONT_P };  /* (1<<384)%p */

// sets `a` to 0
void Fp_set_zero(Fp* a){
    vec_zero((byte*)a, sizeof(Fp));
}

// sets `a` to limb `l`
void Fp_set_limb(Fp* a, const limb_t l){
    vec_zero((byte*)a + sizeof(limb_t), sizeof(Fp) - sizeof(limb_t));
    *((limb_t*)a) = l;
}

void Fp_copy(Fp* res, const Fp* a) {
    vec_copy((byte*)res, (byte*)a, sizeof(Fp));
}

static void Fp_add(Fp *res, const Fp *a, const Fp *b) {
    add_mod_384((limb_t*)res, (limb_t*)a, (limb_t*)b, BLS12_381_P);
}

void Fp_sub(Fp *res, const Fp *a, const Fp *b) {
    sub_mod_384((limb_t*)res, (limb_t*)a, (limb_t*)b, BLS12_381_P);
}

void Fp_neg(Fp *res, const Fp *a) {
    cneg_mod_384((limb_t*)res, (limb_t*)a, 1, BLS12_381_P);
}

static bool check_Fp(const Fp* in) {
    // use same method as in BLST internal function
    // which seems the most efficient. The method uses the assembly-based 
    // modular addition instead of limbs comparison
    Fp temp;
    Fp_add(&temp, in, &ZERO_384); 
    return vec_is_equal(&temp, in, Fp_BYTES);
    // no need to clear `tmp` as no use-case involves sensitive data being passed as `in`
}

// res = a*b*R^(-1)
void Fp_mul_montg(Fp *res, const Fp *a, const Fp *b) {
    mul_mont_384((limb_t*)res, (limb_t*)a, (limb_t*)b, BLS12_381_P, p0);
}

// res = a^2 * R^(-1)
void Fp_squ_montg(Fp *res, const Fp *a) {
    sqr_mont_384((limb_t*)res, (limb_t*)a, BLS12_381_P, p0);
}

// res = a*R
void Fp_to_montg(Fp *res, const Fp *a) {
    mul_mont_384((limb_t*)res, (limb_t*)a, BLS12_381_RR, BLS12_381_P, p0);
}

// res = a*R^(-1)
void Fp_from_montg(Fp *res, const Fp *a) {
    from_mont_384((limb_t*)res, (limb_t*)a, BLS12_381_P, p0);
}

// reads a scalar in `a` and checks it is a valid Fp element (a < p).
// input is bytes-big-endian.
// returns:
//    - BLST_BAD_ENCODING if the length is invalid
//    - BLST_BAD_SCALAR if the scalar isn't in Fp
//    - BLST_SUCCESS if the scalar is valid 
BLST_ERROR Fp_read_bytes(Fp* a, const byte *bin, int len) {
    if (len != Fp_BYTES) {
        return BLST_BAD_ENCODING;
    }
    limbs_from_be_bytes((limb_t*)a, bin, Fp_BYTES);
    // compare read scalar to p
    if (!check_Fp(a)) {
        return BLST_BAD_ENCODING;
    }       
    return BLST_SUCCESS;
}


// write Fp element to bin and assume `bin` has  `Fp_BYTES` allocated bytes.  
void Fp_write_bytes(byte *bin, const Fp* a) {
    be_bytes_from_limbs(bin, (limb_t*)a, Fp_BYTES);
}

// fp_read_bin_safe is a modified version of Relic's (void fp_read_bin).
// It reads a field element from a buffer and makes sure the big number read can be 
// written as a field element (is reduced modulo p). 
// Unlike Relic's versions, the function does not reduce the read integer modulo p and does
// not throw an exception for an integer larger than p. The function returns RLC_OK if the input
// corresponds to a field element, and returns RLC_ERR otherwise. 
static int fp_read_bin_safe(fp_t a, const uint8_t *bin, int len) {
    if (len != Fp_BYTES) {
        return RLC_ERR;
    }

    int ret = RLC_ERR; 
    bn_t t;
    bn_new(t);
    bn_read_bin(t, bin, Fp_BYTES);

    // make sure read bn is reduced modulo p
    // first check is sanity check, since current implementation of `bn_read_bin` insures
    // output bn is positive
    if (bn_sign(t) == RLC_NEG || bn_cmp(t, &core_get()->prime) != RLC_LT) {
        goto out;
    } 

    if (bn_is_zero(t)) {
        fp_zero(a);
    } else {
        if (t->used == 1) {
            fp_prime_conv_dig(a, t->dp[0]);
        } else {
            fp_prime_conv(a, t);
        }
    }	
    ret = RLC_OK;
out:
    bn_free(t);
    return ret;
}

// returns the sign of y.
// 1 if y > (p - 1)/2 and 0 otherwise.
// y is in montgomery form
static byte Fp_get_sign(const fp_t y) {
    return sgn0_pty_mont_384(y, BLS12_381_P, p0);
}

// ------------------- Fp^2 utilities

// sets `a` to limb `l`
void Fp2_set_limb(Fp2* a, const limb_t l){
    Fp_set_limb(&real(a), l);  
    Fp_set_zero(&imag(a));
}

void Fp2_add(Fp2 *res, const Fp2 *a, const Fp2 *b) {
    add_mod_384x((vec384*)res, (vec384*)a, (vec384*)b, BLS12_381_P);
}

void Fp2_sub(Fp2 *res, const Fp2 *a, const Fp2 *b) {
    sub_mod_384x((vec384*)res, (vec384*)a, (vec384*)b, BLS12_381_P);
}

void Fp2_neg(Fp2 *res, const Fp2 *a) {
    cneg_mod_384(real(res), real(a), 1, BLS12_381_P);
    cneg_mod_384(imag(res), imag(a), 1, BLS12_381_P);
}

// res = a*b in montgomery form
void Fp2_mul_montg(Fp2 *res, const Fp2 *a, const Fp2 *b) {
    mul_mont_384x((vec384*)res, (vec384*)a, (vec384*)b, BLS12_381_P, p0); 
}

// res = a^2 in montgomery form
void Fp2_squ_montg(Fp2 *res, const Fp2 *a) {
    sqr_mont_384x((vec384*)res, (vec384*)a, BLS12_381_P, p0); 
}

// checks if `a` is a quadratic residue in Fp^2. If yes, it computes 
// the square root in `res`.
// 
// The boolean output is valid whether `a` is in Montgomery form or not,
// since montgomery constant `R` is a quadratic residue.
// However, the square root is valid only if `a` is in montgomery form.
static bool_t Fp2_sqrt(Fp2 *res, const Fp2* a) {
   return sqrt_fp2((vec384*)res, (vec384*)a);
}

// returns the sign of y.
// sign(y_0) if y_1 = 0, else sign(y_1)
// y coordinates must be in montgomery form
static byte Fp2_get_sign(Fp2* y) {
    return (sgn0_pty_mont_384x((vec384*)y, BLS12_381_P, p0)>>1) & 1;
}

// reads an Fp^2 element in `a`.
// input is a serialization of real(a) concatenated to serializetion of imag(a).
// a[i] are both Fp elements.
// returns:
//    - BLST_BAD_ENCODING if the length is invalid
//    - BLST_BAD_SCALAR if the scalar isn't in Fp
//    - BLST_SUCCESS if the scalar is valid 
static BLST_ERROR Fp2_read_bytes(Fp2* a, const byte *bin, int len) {
    if (len != Fp2_BYTES) {
        return BLST_BAD_ENCODING;
    }
    BLST_ERROR ret = Fp_read_bytes(&real(a), bin, Fp_BYTES);
    if (ret != BLST_SUCCESS) {
        return ret;
    }
    ret = Fp_read_bytes(&imag(a), bin + Fp_BYTES, Fp_BYTES);
    if ( ret != BLST_SUCCESS) {
        return ret;
    }
    return BLST_SUCCESS;
}

// write Fp2 element to bin and assume `bin` has `Fp2_BYTES` allocated bytes.  
void Fp2_write_bytes(byte *bin, const Fp2* a) {
    Fp_write_bytes(bin, &real(a));
    Fp_write_bytes(bin + Fp_BYTES, &imag(a));
}

// ------------------- G1 utilities

// ep_read_bin_compact imports a point from a buffer in a compressed or uncompressed form.
// len is the size of the input buffer.
//
// The resulting point is guaranteed to be on the curve E1.
// The serialization follows:
// https://www.ietf.org/archive/id/draft-irtf-cfrg-pairing-friendly-curves-08.html#name-zcash-serialization-format-) 
// The code is a modified version of Relic ep_read_bin
//
// It returns RLC_OK if the inputs are valid (input buffer lengths are valid and coordinates correspond
// to a point on curve) and the execution completes, and RLC_ERR otherwise.
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
    int is_infinity = bin[0] & (1<<6);
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
    // use a temporary buffer to mask the header bits and read a.x 
    byte temp[Fp_BYTES];
    memcpy(temp, bin, Fp_BYTES);
    temp[0] &= 0x1F;
    if (fp_read_bin_safe(a->x, temp, sizeof(temp)) != RLC_OK) {
        return RLC_ERR;
    }

    if (G1_SERIALIZATION == UNCOMPRESSED) { 
        if (fp_read_bin_safe(a->y, bin + Fp_BYTES, Fp_BYTES) != RLC_OK) {
            return RLC_ERR;
        }
        // check read point is on curve
        if (!ep_on_curve(a)) {
            return RLC_ERR;
        }
        return RLC_OK;
    }
    fp_zero(a->y);
    fp_set_bit(a->y, 0, y_sign);
    if (ep_upk(a, a) == 1) {
        // resulting point is guaranteed to be on curve
        return RLC_OK;
    }
    return RLC_ERR;
}


// TODO: delete aftet deleting ep_write_bin_compact
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
void ep_write_bin_compact(byte *bin, const ep_t a, const int len) {
    const int G1_size = (G1_BYTES/(G1_SERIALIZATION+1));

    if (len!=G1_size) {
        RLC_THROW(ERR_NO_BUFFER);
        return;
    }
 
    if (ep_is_infty(a)) {
            // set the infinity bit
            bin[0] = (G1_SERIALIZATION << 7) | (1<<6);
            memset(bin+1, 0, G1_size-1);
            return;
    }

    RLC_TRY {
        ep_t t;
        ep_null(t);
        ep_new(t); 
        ep_norm(t, a);
        fp_write_bin(bin, Fp_BYTES, t->x);

        if (G1_SERIALIZATION == COMPRESSED) {
            bin[0] |= (fp_get_sign(t->y) << 5);
        } else {
            fp_write_bin(bin + Fp_BYTES, Fp_BYTES, t->y);
        }
        ep_free(t);
    } RLC_CATCH_ANY {
        RLC_THROW(ERR_CAUGHT);
    }

    bin[0] |= (G1_SERIALIZATION << 7);
 }

// Exponentiation of a generic point p in G1
void ep_mult(ep_t res, const ep_t p, const Fr *expo) {
    bn_st* tmp_expo = Fr_blst_to_relic(expo);
    // Using window NAF of size 2 
    ep_mul_lwnaf(res, p, tmp_expo);
    free(tmp_expo);
}

// Exponentiation of generator g1 in G1
// These two function are here for bench purposes only
void ep_mult_gen_bench(ep_t res, const Fr* expo) {
    bn_st* tmp_expo = Fr_blst_to_relic(expo);
    // Using precomputed table of size 4
    ep_mul_gen(res, tmp_expo);
    free(tmp_expo);
}

void ep_mult_generic_bench(ep_t res, const Fr* expo) {
    // generic point multiplication
    ep_mult(res, &core_get()->ep_g, expo);
}

// ------------------- E2 utilities

// TODO: to delete
static int fp2_read_bin_safe(fp2_t a, const uint8_t *bin, int len) {
    if (len != Fp2_BYTES) {
        return RLC_ERR;
    }
    if (fp_read_bin_safe(a[0], bin, Fp_BYTES) != RLC_OK) {
        return RLC_ERR;
    }
    if (fp_read_bin_safe(a[1], bin + Fp_BYTES, Fp_BYTES) != RLC_OK) {
        return RLC_ERR;
    }
    return RLC_OK;
}

// TODO: to delete, only used by temporary E2_blst_to_relic
static int ep2_read_bin_compact(ep2_t a, const byte *bin, const int len) {
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
    fp2_set_dig(a->z, 1);   // a.z
    // use a temporary buffer to mask the header bits and read a.x
    byte temp[Fp2_BYTES];
    memcpy(temp, bin, Fp2_BYTES);
    temp[0] &= 0x1F;        // clear the header bits
    if (fp2_read_bin_safe(a->x, temp, sizeof(temp)) != RLC_OK) {
        return RLC_ERR;
    }

    if (G2_SERIALIZATION == UNCOMPRESSED) {
        if (fp2_read_bin_safe(a->y, bin + Fp2_BYTES, Fp2_BYTES) != RLC_OK){ 
            return RLC_ERR;
        }
        // check read point is on curve
        if (!ep2_on_curve(a)) {
            return RLC_ERR;
        }
        return RLC_OK;
    }
    
    fp2_zero(a->y);
    fp_set_bit(a->y[0], 0, y_sign);
    fp_zero(a->y[1]);
    if (ep2_upk(a, a) == 1) {
        // resulting point is guaranteed to be on curve
        return RLC_OK;
    }
    return RLC_ERR;
}

// TODO: temp utility function to delete
ep2_st* E2_blst_to_relic(const G2* x) {
    ep2_st* out = (ep2_st*)malloc(sizeof(ep2_st)); 
    byte* data = (byte*)malloc(G2_SER_BYTES);
    E2_write_bytes(data, x);
    ep2_read_bin_compact(out, data, G2_SER_BYTES);
    free(data);
    return out;
}

// E2_read_bytes imports a point from a buffer in a compressed or uncompressed form.
// The resulting point is guaranteed to be on curve E2 (no G2 check is included)
//
// reads a scalar in `a` and checks it is a valid Fp element (a < p).
// input is bytes-big-endian.
// returns:
//    - BLST_BAD_ENCODING if the length is invalid or serialization header bits are invalid
//    - BLST_BAD_SCALAR if Fp^2 coordinates couldn't deserialize
//    - BLST_POINT_NOT_ON_CURVE if deserialized point isn't on E2
//    - BLST_SUCCESS if deserialization is valid 

// TODO: replace with POINTonE2_Deserialize_BE and POINTonE2_Uncompress_Z, 
//       and update logic with G2 subgroup check?
BLST_ERROR E2_read_bytes(G2* a, const byte *bin, const int len) {
    // check the length
    if (len != G2_SER_BYTES) {
        return BLST_BAD_ENCODING;
    }

    // check the compression bit
    int compressed = bin[0] >> 7;
    if ((compressed == 1) != (G2_SERIALIZATION == COMPRESSED)) {
        return BLST_BAD_ENCODING;
    } 

    // check if the point in infinity
    int is_infinity = bin[0] & 0x40;
    if (is_infinity) {
        // the remaining bits need to be cleared
        if (bin[0] & 0x3F) {
            return BLST_BAD_ENCODING;
        }
        for (int i=1; i<G2_SER_BYTES-1; i++) {
            if (bin[i]) {
                return BLST_BAD_ENCODING;
            } 
        }
		E2_set_infty(a);
		return RLC_OK;
	} 

    // read the sign bit and check for consistency
    int y_sign = (bin[0] >> 5) & 1;
    if (y_sign && (!compressed)) {
        return BLST_BAD_ENCODING;
    } 
    
    // use a temporary buffer to mask the header bits and read a.x
    byte temp[Fp2_BYTES];
    memcpy(temp, bin, Fp2_BYTES);
    temp[0] &= 0x1F;        // clear the header bits
    BLST_ERROR ret = Fp2_read_bytes(&a->x, temp, sizeof(temp));
    if (ret != BLST_SUCCESS) {
        return ret;
    }

    // set a.z to 1
    Fp2* a_z = &(a->z);
    Fp_copy(&real(a_z), &BLS12_381_pR);
    Fp_set_zero(&imag(a_z));   

    if (G2_SERIALIZATION == UNCOMPRESSED) {
        ret = Fp2_read_bytes(&(a->y), bin + Fp2_BYTES, sizeof(a->y));
        if (ret != BLST_SUCCESS){ 
            return ret;
        }
        // check read point is on curve
        if (!E2_affine_on_curve(a)) { 
            return BLST_POINT_NOT_ON_CURVE;
        }
        return BLST_SUCCESS;
    }
    
    // compute the possible square root
    Fp2* a_x = &(a->x);
    Fp_to_montg(&real(a_x), &real(a_x));
    Fp_to_montg(&imag(a_x), &imag(a_x));

    Fp2* a_y = &(a->y);
    Fp2_squ_montg(a_y, a_x);
    Fp2_mul_montg(a_y, a_y, a_x);
    Fp2_add(a_y, a_y, &B_E2);          // B_E2 is already in Montg form             
    if (!Fp2_sqrt(a_y, a_y))    // check whether x^3+b is a quadratic residue
        return BLST_POINT_NOT_ON_CURVE; 

    // resulting (x,y) is guaranteed to be on curve (y is already in Montg form)
    if (Fp2_get_sign(a_y) != y_sign) {
        Fp2_neg(a_y, a_y); // flip y sign if needed
    }
    return BLST_SUCCESS;
}

// E2_write_bytes exports a point in E(Fp^2) to a buffer in a compressed or uncompressed form.
// It assumes buffer is of length G2_SER_BYTES
// The serialization follows:
// https://www.ietf.org/archive/id/draft-irtf-cfrg-pairing-friendly-curves-08.html#name-zcash-serialization-format-)
// The code is a modified version of Relic ep2_write_bin 
void E2_write_bytes(byte *bin, const G2* a) {
    if (E2_is_infty(a)) {
            // set the infinity bit
            bin[0] = (G2_SERIALIZATION << 7) | (1 << 6);
            memset(bin+1, 0, G2_SER_BYTES-1);
            return;
    }
    G2 tmp;
    E2_to_affine(&tmp, a);

    Fp2* t_x = &(tmp.x);
    Fp_from_montg(&real(t_x), &real(t_x));
    Fp_from_montg(&imag(t_x), &imag(t_x));
    Fp2_write_bytes(bin, t_x);

    Fp2* t_y = &(tmp.y);
    if (G2_SERIALIZATION == COMPRESSED) {
        bin[0] |= (Fp2_get_sign(t_y) << 5);
    } else {
        Fp_from_montg(&real(t_y), &real(t_y));
        Fp_from_montg(&imag(t_y), &imag(t_y));
        Fp2_write_bytes(bin + Fp2_BYTES, t_y);
    }

    bin[0] |= (G2_SERIALIZATION << 7);
}

// set p to infinity
void E2_set_infty(G2* p) {
    vec_zero(p, sizeof(G2));
}

// check if `p` is infinity
bool_t E2_is_infty(const G2* p) {
    return vec_is_zero(p, sizeof(G2));
}

// checks affine point `p` is in E2
bool_t E2_affine_on_curve(const G2* p) {
    // BLST's `POINTonE2_affine_on_curve` does not include the inifity case, 
    // unlike what the function name means.
    return POINTonE2_affine_on_curve((POINTonE2_affine*)p) | E2_is_infty(p);
}

// checks p1 == p2
bool_t E2_is_equal(const G2* p1, const G2* p2) {
    // `POINTonE2_is_equal` includes the infinity case
    return POINTonE2_is_equal((const POINTonE2*)p1, (const POINTonE2*)p2);
}

// res = p
void  E2_copy(G2* res, const G2* p) {
    vec_copy(res, p, sizeof(G2));
}

// converts an E2 point from Jacobian into affine coordinates (z=1)
void E2_to_affine(G2* res, const G2* p) {
    // minor optimization in case coordinates are already affine
    if (vec_is_equal(p->z, BLS12_381_Rx.p2, Fp2_BYTES)) {
        E2_copy(res, p);
        return;
    }
    // convert from Jacobian
    POINTonE2_from_Jacobian((POINTonE2*)res, (const POINTonE2*)p);   
}

// generic point addition that must handle doubling and points at infinity
void E2_add(G2* res, const G2* a, const G2* b) {
    POINTonE2_dadd((POINTonE2*)res, (POINTonE2*)a, (POINTonE2*)b, NULL); 
}

// Point negation in place.
// no need for an api of the form E2_neg(G2* res, const G2* a) for now
static void E2_neg(G2* a) {
    POINTonE2_cneg((POINTonE2*)a, 1);
}

// Exponentiation of a generic point `a` in E2, res = expo.a
void E2_mult(G2* res, const G2* p, const Fr* expo) {
    pow256 tmp;
    pow256_from_Fr(tmp, expo);
    POINTonE2_sign((POINTonE2*)res, (POINTonE2*)p, tmp);
}

// Exponentiation of a generic point `a` in E2 by a byte exponent.
void  E2_mult_small_expo(G2* res, const G2* p, const byte expo) {
    pow256 pow_expo; // `pow256` uses bytes little endian.
    pow_expo[0] = expo;
    vec_zero(&pow_expo[1], 32-1);
    // TODO: to bench against a specific version of mult with 8 bits expo
    POINTonE2_sign((POINTonE2*)res, (POINTonE2*)p, pow_expo);
}

// Exponentiation of generator g2 of G2, res = expo.g2
void G2_mult_gen(G2* res, const Fr* expo) {
    pow256 tmp;
    pow256_from_Fr(tmp, expo);
    POINTonE2_sign((POINTonE2*)res, &BLS12_381_G2, tmp);
}

// computes the sum of the G2 array elements y and writes the sum in jointy
void E2_sum_vector(G2* jointy, const G2* y, const int len){
    E2_set_infty(jointy);
    for (int i=0; i<len; i++){
        E2_add(jointy, jointy, &y[i]);
    }
}

// ------------------- other


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

    // check s1 is in G1
    if (check_membership_G1(elemsG1[0]) != VALID) // only enabled if MEMBERSHIP_CHECK==1
        return INVALID;

    // elemsG1[1] = s2
    ep_new(elemsG1[1]);
    read_ret = ep_read_bin_compact(elemsG1[1], sig2, SIGNATURE_LEN);
    if (read_ret != RLC_OK) 
        return read_ret;

    // check s2 in G1
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
    
    if (core_get()->code == RLC_OK) {
        if (res == RLC_EQ) return VALID;
        return INVALID;
    }
    return UNDEFINED;
}

// Subtracts all G2 array elements `y` from an element `x` and writes the 
// result in res
void E2_subtract_vector(G2* res, const G2* x, const G2* y, const int len){
    E2_sum_vector(res, y, len);
    E2_neg(res);
    E2_add(res, x, res);
}

// computes the sum of the G1 array elements y and writes the sum in jointy
void ep_sum_vector(ep_t jointx, ep_st* x, const int len) {
    ep_set_infty(jointx);
    for (int i=0; i<len; i++){
        ep_add_jacob(jointx, jointx, &x[i]);
    }
}

// Computes the sum of the signatures (G1 elements) flattened in a single sigs array
// and writes the sum (G1 element) as bytes in dest.
// The function assumes sigs is correctly allocated with regards to len.
int ep_sum_vector_byte(byte* dest, const byte* sigs_bytes, const int len) {
    int error = UNDEFINED;

    // temp variables
    ep_t acc;        
    ep_new(acc);
    ep_set_infty(acc);
    ep_st* sigs = (ep_st*) malloc(len * sizeof(ep_st));
    if (!sigs) goto mem_error;
    for (int i=0; i < len; i++) ep_new(sigs[i]);

    // import the points from the array
    for (int i=0; i < len; i++) {
        // deserialize each point from the input array
        error = ep_read_bin_compact(&sigs[i], &sigs_bytes[SIGNATURE_LEN*i], SIGNATURE_LEN);
        if (error != RLC_OK) {
            goto out;
        }
    }
    // sum the points
    ep_sum_vector(acc, sigs, len);
    // export the result
    ep_write_bin_compact(dest, acc, SIGNATURE_LEN);

    error = VALID;
out:
    // free the temp memory
    ep_free(acc);
    for (int i=0; i < len; i++) ep_free(sigs[i]);
    free(sigs);
mem_error:
    return error;
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
const uint64_t beta_data[Fp_LIMBS] = { 
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
    dv_copy(b, beta_data, Fp_LIMBS); 
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
    ep_mul_basic(p, p, &core_get()->ep_r);

    assert(ep_on_curve(p));  // sanity check to make sure p is in E1
}

// generates a random point in G2 and stores it in p
void ep2_rand_G2(ep2_t p) {
    // multiplies G2 generator by a random scalar
    ep2_rand(p);
}

// generates a random point in E2\G2 and stores it in p
void ep2_rand_G2complement(ep2_t p) {
    // generate a random point in E2
    p->coord = BASIC;
    fp_set_dig(p->z[0], 1);
	fp_zero(p->z[1]);
    do {
        fp2_rand(p->x); // set x to a random field element
        byte r;
        rand_bytes(&r, 1);
        fp2_zero(p->y);
        fp_set_bit(p->y[0], 0, r&1); // set y randomly to 0 or 1
    }
    while (ep2_upk(p, p) == 0); // make sure p is in E1

    // map the point to E1\G1 by clearing G1 order
    ep2_mul_basic(p, p, &core_get()->ep_r);

    assert(ep2_on_curve(p));  // sanity check to make sure p is in E1
}

// This is a testing function.
// It wraps a call to a Relic macro since cgo can't call macros.
void xmd_sha256(uint8_t *hash, int len_hash, uint8_t *msg, int len_msg, uint8_t *dst, int len_dst){
    md_xmd_sh256(hash, len_hash, msg, len_msg, dst, len_dst);
}


// DEBUG printing functions 
void bytes_print_(char* s, byte* data, int len) {
    printf("[%s]:\n", s);
    for (int i=0; i<len; i++) 
        printf("%02x,", data[i]);
    printf("\n");
}

void Fr_print_(char* s, Fr* a) {
    printf("[%s]:\n", s);
    limb_t* p = (limb_t*)(a) + Fr_LIMBS;
    for (int i=0; i<Fr_LIMBS; i++) 
        printf("%16llx", *(--p));
    printf("\n");
}

void Fp_print_(char* s, Fp* a) {
    printf("[%s]:\n", s);
    limb_t* p = (limb_t*)(a) + Fp_LIMBS;
    for (int i=0; i<Fp_LIMBS; i++) 
        printf("%16llx", *(--p));
    printf("\n");
}

void Fp2_print_(char* s, const Fp2* a) {
    printf("[%s]:\n", s);
    Fp tmp;
    Fp_from_montg(&tmp, &real(a));
    limb_t* p = (limb_t*)(&tmp) + Fp_LIMBS;
    for (int i=0; i<Fp_LIMBS; i++) 
        printf("%16llx", *(--p));
    printf("\n");
    Fp_from_montg(&tmp, &imag(a));
    p = (limb_t*)(&tmp) + Fp_LIMBS;
    for (int i=0; i<Fp_LIMBS; i++) 
        printf("%16llx", *(--p));
    printf("\n");
}

void E2_print_(char* s, const G2* a) {
      printf("[%s]:\n", s);
      Fp2_print_(".x", &(a->x));
      Fp2_print_(".y", &(a->y));
      Fp2_print_(".z", &(a->z));
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
