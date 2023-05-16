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

int get_mapToG1_input_len() {
    return MAP_TO_G1_INPUT_LEN;
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

// ------------------- Fr utilities

// Montgomery constant R related to the curve order r
// R mod r = (1<<256)%r 
const Fr BLS12_381_rR = {{  \
    TO_LIMB_T(0x1824b159acc5056f), TO_LIMB_T(0x998c4fefecbc4ff5), \
    TO_LIMB_T(0x5884b7fa00034802), TO_LIMB_T(0x00000001fffffffe), \
    }};

// returns true if a == 0 and false otherwise
bool_t Fr_is_zero(const Fr* a) {
    return bytes_are_zero((const byte*)a, sizeof(Fr));
}

// returns true if a == b and false otherwise
bool_t Fr_is_equal(const Fr* a, const Fr* b) {
    return vec_is_equal(a, b, sizeof(Fr));
}

// sets `a` to limb `l`
void Fr_set_limb(Fr* a, const limb_t l){
    vec_zero((byte*)a + sizeof(limb_t), sizeof(Fr) - sizeof(limb_t));
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
// TODO: clean up?
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
static void pow256_from_be_bytes(pow256 ret, const byte a[Fr_BYTES])
{
    byte* b = (byte*)a + Fr_BYTES - 1;
    if ((uptr_t)ret == (uptr_t)a) { // swap in place
        for (int i=0; i<Fr_BYTES/2; i++) {
            byte tmp = *ret;
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
// TODO: check endianness!!
static void pow256_from_Fr(pow256 ret, const Fr* in) {
    le_bytes_from_limbs(ret, (limb_t*)in, Fr_BYTES);
}

// reads a scalar in `a` and checks it is a valid Fr element (a < r).
// input is bytes-big-endian.
// returns:
//    - BLST_BAD_ENCODING if the length is invalid
//    - BLST_BAD_SCALAR if the scalar isn't in Fr
//    - BLST_SUCCESS if the scalar is valid 
BLST_ERROR Fr_read_bytes(Fr* a, const byte *bin, int len) {
    if (len != Fr_BYTES) {
        return BLST_BAD_ENCODING;
    }
    pow256 tmp;
    // compare to r using the provided tool from BLST 
    pow256_from_be_bytes(tmp, bin);  // TODO: check endianness!!
    if (!check_mod_256(tmp, BLS12_381_r)) {  // check_mod_256 compares pow256 against a vec256!
        return BLST_BAD_SCALAR;
    }
    vec_zero(tmp, sizeof(tmp));
    limbs_from_be_bytes((limb_t*)a, bin, Fr_BYTES); // TODO: check endianness!!
    return BLST_SUCCESS;
}

// reads a scalar in `a` and checks it is a valid Fr_star element (0 < a < r).
// input bytes are big endian.
// returns:
//    - BLST_BAD_ENCODING if the length is invalid
//    - BLST_BAD_SCALAR if the scalar isn't in Fr_star
//    - BLST_SUCCESS if the scalar is valid 
BLST_ERROR Fr_star_read_bytes(Fr* a, const byte *bin, int len) {
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
void Fr_write_bytes(byte *bin, const Fr* a) {
    // be_bytes_from_limbs works for both limb endiannesses
    be_bytes_from_limbs(bin, (limb_t*)a, Fr_BYTES);
}

// maps big-endian bytes into an Fr element using modular reduction
// Input is byte-big-endian, output is Fr (internally vec256)
// TODO: check redc_mont_256(vec256 ret, const vec512 a, const vec256 p, limb_t n0);
static void Fr_from_be_bytes(Fr* out, const byte *bytes, size_t n)
{
    Fr digit, radix;
    Fr_set_zero(out);
    Fr_copy(&radix, (Fr*)BLS12_381_rRR); // R^2

    byte* p = (byte*)bytes + n;
    while (n > Fr_BYTES) {
        // limbs_from_be_bytes works for both limb endiannesses
        limbs_from_be_bytes((limb_t*)&digit, p -= Fr_BYTES, Fr_BYTES); // l_i
        Fr_mul_montg(&digit, &digit, &radix); // l_i * R^i  (i is the loop number starting at 1)
        Fr_add(out, out, &digit);
        Fr_mul_montg(&radix, &radix, (Fr*)BLS12_381_rRR); // R^(i+1)
        n -= Fr_BYTES;
    }
    Fr_set_zero(&digit);
    limbs_from_be_bytes((limb_t*)&digit, p - n, n);
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
bool_t map_bytes_to_Fr(Fr* a, const byte* bin, int len) {
    Fr_from_be_bytes(a, bin, len);
    return Fr_is_zero(a);
}

// ------------------- Fp utilities

// Montgomery constants related to the prime p
const Fp BLS12_381_pR = { ONE_MONT_P };        /* R mod p = (1<<384)%p */

// sets `a` to 0
static void Fp_set_zero(Fp* a){
    vec_zero((byte*)a, sizeof(Fp));
}

// sets `a` to limb `l`
static void Fp_set_limb(Fp* a, const limb_t l){
    vec_zero((byte*)a + sizeof(limb_t), sizeof(Fp) - sizeof(limb_t));
    *((limb_t*)a) = l;
}

static void Fp_copy(Fp* res, const Fp* a) {
    vec_copy((byte*)res, (byte*)a, sizeof(Fp));
}

static void Fp_add(Fp *res, const Fp *a, const Fp *b) {
    add_mod_384((limb_t*)res, (limb_t*)a, (limb_t*)b, BLS12_381_P);
}

static void Fp_sub(Fp *res, const Fp *a, const Fp *b) {
    sub_mod_384((limb_t*)res, (limb_t*)a, (limb_t*)b, BLS12_381_P);
}

static void Fp_neg(Fp *res, const Fp *a) {
    cneg_mod_384((limb_t*)res, (limb_t*)a, 1, BLS12_381_P);
}

// checks if `a` is a quadratic residue in Fp. If yes, it computes 
// the square root in `res`.
// 
// The boolean output is valid whether `a` is in Montgomery form or not,
// since montgomery constant `R` is a quadratic residue.
// However, the square root is valid only if `a` is in montgomery form.
static bool_t Fp_sqrt_montg(Fp *res, const Fp* a) {
   return sqrt_fp((limb_t*)res, (limb_t*)a);
}

static bool Fp_check(const Fp* in) {
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
    if (!Fp_check(a)) {
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
static int fp_read_bin_safe(fp_t a, const byte *bin, int len) {
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
static byte Fp_get_sign(const Fp* y) {
    // BLST's sgn0_pty_mont_384 requires input to be in Montg form.
    // The needed sign bit is on position 1 !
    return (sgn0_pty_mont_384((const limb_t*)y, BLS12_381_P, p0)>>1) & 1;
}

// ------------------- Fp^2 utilities

// sets `a` to limb `l`
static void Fp2_set_limb(Fp2* a, const limb_t l){
    Fp_set_limb(&real(a), l);  
    Fp_set_zero(&imag(a));
}

static void Fp2_add(Fp2 *res, const Fp2 *a, const Fp2 *b) {
    add_mod_384x((vec384*)res, (vec384*)a, (vec384*)b, BLS12_381_P);
}

static void Fp2_sub(Fp2 *res, const Fp2 *a, const Fp2 *b) {
    sub_mod_384x((vec384*)res, (vec384*)a, (vec384*)b, BLS12_381_P);
}

static void Fp2_neg(Fp2 *res, const Fp2 *a) {
    cneg_mod_384(real(res), real(a), 1, BLS12_381_P);
    cneg_mod_384(imag(res), imag(a), 1, BLS12_381_P);
}

// res = a*b in montgomery form
static void Fp2_mul_montg(Fp2 *res, const Fp2 *a, const Fp2 *b) {
    mul_mont_384x((vec384*)res, (vec384*)a, (vec384*)b, BLS12_381_P, p0); 
}

// res = a^2 in montgomery form
static void Fp2_squ_montg(Fp2 *res, const Fp2 *a) {
    sqr_mont_384x((vec384*)res, (vec384*)a, BLS12_381_P, p0); 
}

// checks if `a` is a quadratic residue in Fp^2. If yes, it computes 
// the square root in `res`.
// 
// The boolean output is valid whether `a` is in Montgomery form or not,
// since montgomery constant `R` is a quadratic residue.
// However, the square root is valid only if `a` is in montgomery form.
static bool_t Fp2_sqrt_montg(Fp2 *res, const Fp2* a) {
   return sqrt_fp2((vec384*)res, (vec384*)a);
}

// returns the sign of y.
// sign(y_0) if y_1 = 0, else sign(y_1)
// y coordinates must be in montgomery form
static byte Fp2_get_sign(Fp2* y) {
    // BLST's sgn0_pty_mont_384x requires input to be in Montg form.
    // The needed sign bit is on position 1 !
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

// ------------------- E1 utilities

// TODO: to delete, only used by temporary E2_blst_to_relic
static int ep_read_bin_compact(ep_t a, const byte *bin, const int len) {
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

// TODO: temp utility function to delete
ep_st* E1_blst_to_relic(const E1* x) {
    ep_st* out = (ep_st*)malloc(sizeof(ep_st)); 
    byte* data = (byte*)malloc(G1_SER_BYTES);
    E1_write_bytes(data, x);
    ep_read_bin_compact(out, data, G1_SER_BYTES);
    free(data);
    return out;
}

void E1_copy(E1* res, const E1* p) {
    vec_copy(res, p, sizeof(E1));
}

// checks p1 == p2
bool_t E1_is_equal(const E1* p1, const E1* p2) {
    // `POINTonE1_is_equal` includes the infinity case
    return POINTonE1_is_equal((const POINTonE1*)p1, (const POINTonE1*)p2);
}

// compare p to infinity
bool_t E1_is_infty(const E1* p) {
    // BLST infinity points are defined by Z=0
    return vec_is_zero(p->z, sizeof(p->z));  
}

// set p to infinity
void E1_set_infty(E1* p) {
    // BLST infinity points are defined by Z=0
    vec_zero(p->z, sizeof(p->z));  
}

// converts an E1 point from Jacobian into affine coordinates (z=1)
void E1_to_affine(E1* res, const E1* p) {
    // optimize in case coordinates are already affine
    if (vec_is_equal(p->z, BLS12_381_pR, Fp_BYTES)) {
        E1_copy(res, p);
        return;
    }
    // convert from Jacobian
    POINTonE1_from_Jacobian((POINTonE1*)res, (const POINTonE1*)p);   
}

// checks affine point `p` is in E1
bool_t E1_affine_on_curve(const E1* p) {
    // BLST's `POINTonE1_affine_on_curve` does not include the inifity case!
    return POINTonE1_affine_on_curve((POINTonE1_affine*)p) | E1_is_infty(p);
}

// checks if input E1 point is on the subgroup G1.
// It assumes input `p` is on E1.
bool_t E1_in_G1(const E1* p){
    // currently uses Scott method
    return POINTonE1_in_G1((const POINTonE1*)p);
}

// E1_read_bytes imports a E1(Fp) point from a buffer in a compressed or uncompressed form.
// The resulting point is guaranteed to be on curve E1 (no G1 check is included).
// Expected serialization follows:
// https://www.ietf.org/archive/id/draft-irtf-cfrg-pairing-friendly-curves-08.html#name-zcash-serialization-format-)
//
// returns:
//    - BLST_BAD_ENCODING if the length is invalid or serialization header bits are invalid
//    - BLST_BAD_SCALAR if Fp coordinates couldn't deserialize
//    - BLST_POINT_NOT_ON_CURVE if deserialized point isn't on E1
//    - BLST_SUCCESS if deserialization is valid 

// TODO: replace with POINTonE1_Deserialize_BE and POINTonE1_Uncompress_Z, 
//       and update logic with G2 subgroup check?
BLST_ERROR E1_read_bytes(E1* a, const byte *bin, const int len) {
    // check the length
    if (len != G1_SER_BYTES) {
        return BLST_BAD_ENCODING;
    }

    // check the compression bit
    int compressed = bin[0] >> 7;
    if ((compressed == 1) != (G1_SERIALIZATION == COMPRESSED)) {
        return BLST_BAD_ENCODING;
    } 

    // check if the point in infinity
    int is_infinity = bin[0] & 0x40;
    if (is_infinity) {
        // the remaining bits need to be cleared
        if (bin[0] & 0x3F) {
            return BLST_BAD_ENCODING;
        }
        for (int i=1; i<G1_SER_BYTES-1; i++) {
            if (bin[i]) {
                return BLST_BAD_ENCODING;
            } 
        }
		E1_set_infty(a);
		return BLST_SUCCESS;
	} 

    // read the sign bit and check for consistency
    int y_sign = (bin[0] >> 5) & 1;
    if (y_sign && (!compressed)) {
        return BLST_BAD_ENCODING;
    } 
    
    // use a temporary buffer to mask the header bits and read a.x
    byte temp[Fp_BYTES];
    memcpy(temp, bin, Fp_BYTES);
    temp[0] &= 0x1F;        // clear the header bits
    BLST_ERROR ret = Fp_read_bytes(&a->x, temp, sizeof(temp));
    if (ret != BLST_SUCCESS) {
        return ret;
    }

    // set a.z to 1
    Fp_copy(&a->z, &BLS12_381_pR);

    if (G1_SERIALIZATION == UNCOMPRESSED) {
        ret = Fp_read_bytes(&a->y, bin + Fp_BYTES, sizeof(a->y));
        if (ret != BLST_SUCCESS){ 
            return ret;
        }
        // check read point is on curve
        if (!E1_affine_on_curve(a)) { 
            return BLST_POINT_NOT_ON_CURVE;
        }
        return BLST_SUCCESS;
    }
    
    // compute the possible square root
    Fp_to_montg(&a->x, &a->x);
    Fp_squ_montg(&a->y, &a->x);
    Fp_mul_montg(&a->y, &a->y, &a->x);    // x^3
    Fp_add(&a->y, &a->y, &B_E1);          // B_E1 is already in Montg form             
    if (!Fp_sqrt_montg(&a->y, &a->y))     // check whether x^3+b is a quadratic residue
        return BLST_POINT_NOT_ON_CURVE; 

    // resulting (x,y) is guaranteed to be on curve (y is already in Montg form)
    if (Fp_get_sign(&a->y) != y_sign) {
        Fp_neg(&a->y, &a->y); // flip y sign if needed
    }
    return BLST_SUCCESS;
}

// E1_write_bytes exports a point in E1(Fp) to a buffer in a compressed or uncompressed form.
// It assumes buffer is of length G1_SER_BYTES
// The serialization follows:
// https://www.ietf.org/archive/id/draft-irtf-cfrg-pairing-friendly-curves-08.html#name-zcash-serialization-format-)
void E1_write_bytes(byte *bin, const E1* a) {
    if (E1_is_infty(a)) {
            // set the infinity bit
            bin[0] = (G1_SERIALIZATION << 7) | (1 << 6);
            memset(bin+1, 0, G1_SER_BYTES-1);
            return;
    }
    E1 tmp;
    E1_to_affine(&tmp, a);

    Fp_from_montg(&tmp.x, &tmp.x);
    Fp_write_bytes(bin, &tmp.x);

    if (G1_SERIALIZATION == COMPRESSED) {
        bin[0] |= (Fp_get_sign(&tmp.y) << 5);
    } else {
        Fp_from_montg(&tmp.y, &tmp.y);
        Fp_write_bytes(bin + Fp_BYTES, &tmp.y);
    }
    // compression bit
    bin[0] |= (G1_SERIALIZATION << 7);
}

// generic point addition that must handle doubling and points at infinity
void E1_add(E1* res, const E1* a, const E1* b) {
    POINTonE1_dadd((POINTonE1*)res, (POINTonE1*)a, (POINTonE1*)b, NULL); 
}

// Exponentiation of a generic point `a` in E1, res = expo.a
void E1_mult(E1* res, const E1* p, const Fr* expo) {
    pow256 tmp;
    pow256_from_Fr(tmp, expo);
    POINTonE1_mult_glv((POINTonE1*)res, (POINTonE1*)p, tmp);
    vec_zero(&tmp, sizeof(tmp));
}

// computes the sum of the E1 array elements `y[i]` and writes it in `sum`.
void E1_sum_vector(E1* sum, const E1* y, const int len){
    E1_set_infty(sum);
    for (int i=0; i<len; i++){
        E1_add(sum, sum, &y[i]);
    }
}

// Computes the sum of input signatures (E1 elements) flattened in a single byte array
// `sigs_bytes` of `sigs_len` bytes.
// and writes the sum (E1 element) as bytes in `dest`.
// The function does not check membership of E1 inputs in G1 subgroup.
// The header is using byte pointers to minimize Cgo calls from the Go layer.
int E1_sum_vector_byte(byte* dest, const byte* sigs_bytes, const int sigs_len) {
    int error = UNDEFINED;
    // sanity check that `len` is multiple of `G1_SER_BYTES`
    if (sigs_len % G1_SER_BYTES) {
        error =  INVALID; 
        goto mem_error;
    }
    int n = sigs_len/G1_SER_BYTES; // number of signatures
    
    E1* sigs = (E1*) malloc(n * sizeof(E1));
    if (!sigs) goto mem_error;

    // import the points from the array
    for (int i=0; i < n; i++) {
        // deserialize each point from the input array
        if  (E1_read_bytes(&sigs[i], &sigs_bytes[G1_SER_BYTES*i], G1_SER_BYTES) != BLST_SUCCESS) {
            error = INVALID; 
            goto out;
        }
    }
    // sum the points
    E1 acc;        
    E1_sum_vector(&acc, sigs, n);
    // export the result
    E1_write_bytes(dest, &acc);
    error = VALID;
out:
    free(sigs);
mem_error:
    return error;
}

// Exponentiation of generator g1 of G1, res = expo.g1
void G1_mult_gen(E1* res, const Fr* expo) {
    pow256 tmp;
    pow256_from_Fr(tmp, expo);
    POINTonE1_mult_glv((POINTonE1*)res, &BLS12_381_G1, tmp);
    vec_zero(&tmp, sizeof(tmp));
}

 
// Reads a scalar bytes and maps it to Fp using modular reduction.
// output is in Montgomery form. 
// `len` must be less or equal to 96 bytes and must be a multiple of 8.
// This function is only used by `map_to_G1` where input is 64 bytes.
// input `len` is not checked to satisfy the conditions above.
static void map_96_bytes_to_Fp(Fp* a, const byte* bin, int len) {
    vec768 tmp ;
    vec_zero(&tmp, sizeof(tmp));
    limbs_from_be_bytes((limb_t*)tmp, bin, len);
    redc_mont_384((limb_t*)a, tmp, BLS12_381_P, p0); // aR^(-2)
    Fp_mul_montg(a, a, (Fp*)BLS12_381_RRRR); // aR
}

// maps bytes input `hash` to G1.
// `hash` must be `MAP_TO_G1_INPUT_LEN` (128 bytes)
// It uses construction 2 from section 5 in https://eprint.iacr.org/2019/403.pdf
int map_to_G1(E1* h, const byte* hash, const int len) {
    // sanity check of length
    if (len != MAP_TO_G1_INPUT_LEN) {
        return INVALID;
    }
    // map to field elements
    Fp u[2];
    map_96_bytes_to_Fp(&u[0], hash, MAP_TO_G1_INPUT_LEN/2);
    map_96_bytes_to_Fp(&u[1], hash + MAP_TO_G1_INPUT_LEN/2, MAP_TO_G1_INPUT_LEN/2);
    // map field elements to G1
    // inputs must be in Montgomery form
    map_to_g1((POINTonE1 *)h, (limb_t *)&u[0], (limb_t *)&u[1]);
    return VALID;
}

// ------------------- E2 utilities

// TODO: to delete
static int fp2_read_bin_safe(fp2_t a, const byte *bin, int len) {
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
ep2_st* E2_blst_to_relic(const E2* x) {
    ep2_st* out = (ep2_st*)malloc(sizeof(ep2_st)); 
    byte* data = (byte*)malloc(G2_SER_BYTES);
    E2_write_bytes(data, x);
    ep2_read_bin_compact(out, data, G2_SER_BYTES);
    free(data);
    return out;
}

// E2_read_bytes imports a E2(Fp^2) point from a buffer in a compressed or uncompressed form.
// The resulting point is guaranteed to be on curve E2 (no G2 check is included).
//
// returns:
//    - BLST_BAD_ENCODING if the length is invalid or serialization header bits are invalid
//    - BLST_BAD_SCALAR if Fp^2 coordinates couldn't deserialize
//    - BLST_POINT_NOT_ON_CURVE if deserialized point isn't on E2
//    - BLST_SUCCESS if deserialization is valid 

// TODO: replace with POINTonE2_Deserialize_BE and POINTonE2_Uncompress_Z, 
//       and update logic with G2 subgroup check?
BLST_ERROR E2_read_bytes(E2* a, const byte *bin, const int len) {
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
		return BLST_SUCCESS;
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
    if (!Fp2_sqrt_montg(a_y, a_y))    // check whether x^3+b is a quadratic residue
        return BLST_POINT_NOT_ON_CURVE; 

    // resulting (x,y) is guaranteed to be on curve (y is already in Montg form)
    if (Fp2_get_sign(a_y) != y_sign) {
        Fp2_neg(a_y, a_y); // flip y sign if needed
    }
    return BLST_SUCCESS;
}

// E2_write_bytes exports a point in E2(Fp^2) to a buffer in a compressed or uncompressed form.
// It assumes buffer is of length G2_SER_BYTES
// The serialization follows:
// https://www.ietf.org/archive/id/draft-irtf-cfrg-pairing-friendly-curves-08.html#name-zcash-serialization-format-) 
void E2_write_bytes(byte *bin, const E2* a) {
    if (E2_is_infty(a)) {
            // set the infinity bit
            bin[0] = (G2_SERIALIZATION << 7) | (1 << 6);
            memset(bin+1, 0, G2_SER_BYTES-1);
            return;
    }
    E2 tmp;
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
void E2_set_infty(E2* p) {
    // BLST infinity points are defined by Z=0
    vec_zero(p->z, sizeof(p->z));  
}

// check if `p` is infinity
bool_t E2_is_infty(const E2* p) {
    // BLST infinity points are defined by Z=0
    return vec_is_zero(p->z, sizeof(p->z));
}

// checks affine point `p` is in E2
bool_t E2_affine_on_curve(const E2* p) {
    // BLST's `POINTonE2_affine_on_curve` does not include the infinity case!
    return POINTonE2_affine_on_curve((POINTonE2_affine*)p) | E2_is_infty(p);
}

// checks p1 == p2
bool_t E2_is_equal(const E2* p1, const E2* p2) {
    // `POINTonE2_is_equal` includes the infinity case
    return POINTonE2_is_equal((const POINTonE2*)p1, (const POINTonE2*)p2);
}

// res = p
void  E2_copy(E2* res, const E2* p) {
    vec_copy(res, p, sizeof(E2));
}

// converts an E2 point from Jacobian into affine coordinates (z=1)
void E2_to_affine(E2* res, const E2* p) {
    // optimize in case coordinates are already affine
    if (vec_is_equal(p->z, BLS12_381_Rx.p2, Fp2_BYTES)) {
        E2_copy(res, p);
        return;
    }
    // convert from Jacobian
    POINTonE2_from_Jacobian((POINTonE2*)res, (const POINTonE2*)p);   
}

// generic point addition that must handle doubling and points at infinity
void E2_add(E2* res, const E2* a, const E2* b) {
    POINTonE2_dadd((POINTonE2*)res, (POINTonE2*)a, (POINTonE2*)b, NULL); 
}

// Point negation in place.
// no need for an api of the form E2_neg(E2* res, const E2* a) for now
static void E2_neg(E2* a) {
    POINTonE2_cneg((POINTonE2*)a, 1);
}

// Exponentiation of a generic point `a` in E2, res = expo.a
void E2_mult(E2* res, const E2* p, const Fr* expo) {
    pow256 tmp;
    pow256_from_Fr(tmp, expo);
    POINTonE2_mult_gls((POINTonE2*)res, (POINTonE2*)p, tmp);
    vec_zero(&tmp, sizeof(tmp)); 
}

// Exponentiation of a generic point `a` in E2 by a byte exponent.
void  E2_mult_small_expo(E2* res, const E2* p, const byte expo) {
    pow256 pow_expo; 
    vec_zero(&pow_expo, sizeof(pow256)); 
    pow_expo[0] = expo; // `pow256` uses bytes little endian.
    // TODO: to bench against a specific version of mult with 8 bits expo
    POINTonE2_mult_gls((POINTonE2*)res, (POINTonE2*)p, pow_expo);
    pow_expo[0] = 0;
}

// Exponentiation of generator g2 of G2, res = expo.g2
void G2_mult_gen(E2* res, const Fr* expo) {
    pow256 tmp;
    pow256_from_Fr(tmp, expo);
    POINTonE2_mult_gls((POINTonE2*)res, &BLS12_381_G2, tmp);
    vec_zero(&tmp, sizeof(tmp));
}

// checks if input E2 point is on the subgroup G2.
// It assumes input `p` is on E2.
bool_t E2_in_G2(const E2* p){
    // currently uses Scott method
    return POINTonE2_in_G2((const POINTonE2*)p);
}

// computes the sum of the E2 array elements `y[i]` and writes it in `sum`
void E2_sum_vector(E2* sum, const E2* y, const int len){
    E2_set_infty(sum);
    for (int i=0; i<len; i++){
        E2_add(sum, sum, &y[i]);
    }
}

// ------------------- other


// Verifies the validity of 2 SPoCK proofs and 2 public keys.
// Membership check in G1 of both proofs is verified in this function.
// Membership check in G2 of both keys is not verified in this function.
// the membership check in G2 is separated to allow optimizing multiple verifications 
// using the same public keys.
int bls_spock_verify(const E2* pk1, const byte* sig1, const E2* pk2, const byte* sig2) {  
    ep_t elemsG1[2];
    ep2_t elemsG2[2];
    ep_new(elemsG1[0]);
    ep_new(elemsG1[1]);
    ep2_new(elemsG2[1]);
    ep2_new(elemsG2[0]);
    int ret = UNDEFINED;

    // elemsG1[0] = s1
    E1 s;
    if (E1_read_bytes(&s, sig1, SIGNATURE_LEN) != BLST_SUCCESS) {
        ret = INVALID;
        goto out;
    };
    // check s1 is in G1
    if (!E1_in_G1(&s))  {
        ret = INVALID;
        goto out;
    }
    ep_st* s_tmp = E1_blst_to_relic(&s);
    ep_copy(elemsG1[0], s_tmp);

    // elemsG1[1] = s2
    if (E1_read_bytes(&s, sig2, SIGNATURE_LEN) != BLST_SUCCESS) {
        ret = INVALID;
        goto out;
    };
    // check s2 is in G1
    if (!E1_in_G1(&s))  {
        ret = INVALID;
        goto out;
    }
    s_tmp = E1_blst_to_relic(&s);
    ep_copy(elemsG1[1], s_tmp); 

    // elemsG2[1] = pk1
    ep2_st* pk_tmp = E2_blst_to_relic(pk1);
    ep2_copy(elemsG2[1], pk_tmp);

    // elemsG2[0] = pk2
    pk_tmp = E2_blst_to_relic(pk2);
    ep2_copy(elemsG2[0], pk_tmp);
    free(pk_tmp);
    free(s_tmp);

#if DOUBLE_PAIRING  
    // elemsG2[0] = -pk2
    ep2_neg(elemsG2[0], elemsG2[0]);

    fp12_t pair;
    fp12_new(&pair);
    // double pairing with Optimal Ate 
    pp_map_sim_oatep_k12(pair, (ep_t*)(elemsG1) , (ep2_t*)(elemsG2), 2);

    // compare the result to 1
    int res = fp12_cmp_dig(pair, 1);
    fp12_free(pair);

#elif SINGLE_PAIRING   
    fp12_t pair1, pair2;
    fp12_new(&pair1); fp12_new(&pair2);
    pp_map_oatep_k12(pair1, elemsG1[0], elemsG2[0]);
    pp_map_oatep_k12(pair2, elemsG1[1], elemsG2[1]);

    int res = fp12_cmp(pair1, pair2);
    fp12_free(pair1); fp12_free(pair2);
#endif

    if (core_get()->code == RLC_OK) {
        if (res == RLC_EQ) { 
            ret = VALID; 
        }
        else { 
            ret = INVALID; 
        }
        goto out; 
    }

out:
    ep_free(elemsG1[0]);
    ep_free(elemsG1[1]);
    ep2_free(elemsG2[0]);
    ep2_free(elemsG2[1]);
    return ret;
}

// Subtracts all G2 array elements `y` from an element `x` and writes the 
// result in res
void E2_subtract_vector(E2* res, const E2* x, const E2* y, const int len){
    E2_sum_vector(res, y, len);
    E2_neg(res);
    E2_add(res, x, res);
}


// maps the bytes to a point in G1.
// `len` should be at least Fr_BYTES.
// this is a testing file only, should not be used in any protocol!
void unsafe_map_bytes_to_G1(E1* p, const byte* bytes, int len) {
    assert(len >= Fr_BYTES);
    // map to Fr
    Fr log;
    map_bytes_to_Fr(&log, bytes, len);
    // multiplies G1 generator by a random scalar
    G1_mult_gen(p, &log);
}

// generates a point in E1\G1 and stores it in p
// this is a testing file only, should not be used in any protocol!
BLST_ERROR unsafe_map_bytes_to_G1complement(E1* p, const byte* bytes, int len) {
    assert(G1_SERIALIZATION == COMPRESSED);
    assert(len >= G1_SER_BYTES);

    // attempt to deserilize a compressed E1 point from input bytes
    // after fixing the header 2 bits
    byte copy[G1_SER_BYTES];
    memcpy(copy, bytes, sizeof(copy));
    copy[0] |= 1<<7;        // set compression bit
    copy[0] &= ~(1<<6);     // clear infinity bit - point is not infinity

    BLST_ERROR ser = E1_read_bytes(p, copy, G1_SER_BYTES);
    if (ser != BLST_SUCCESS) {
        return ser;
    }

    // map the point to E2\G2 by clearing G2 order
    E1_mult(p, p, (const Fr*)BLS12_381_r);
    E1_to_affine(p, p);

    assert(E1_affine_on_curve(p));  // sanity check to make sure p is in E2
    return BLST_SUCCESS;
}

// maps the bytes to a point in G2.
// `len` should be at least Fr_BYTES.
// this is a testing tool only, it should not be used in any protocol!
void unsafe_map_bytes_to_G2(E2* p, const byte* bytes, int len) {
    assert(len >= Fr_BYTES);
    // map to Fr
    Fr log;
    map_bytes_to_Fr(&log, bytes, len);
    // multiplies G2 generator by a random scalar
    G2_mult_gen(p, &log);
}

// attempts to map `bytes` to a point in E2\G2 and stores it in p.
// `len` should be at least G2_SER_BYTES. It returns BLST_SUCCESS only if mapping 
// succeeds.
// For now, function only works when E2 serialization is compressed.
// this is a testing tool only, it should not be used in any protocol!
BLST_ERROR unsafe_map_bytes_to_G2complement(E2* p, const byte* bytes, int len) {
    assert(G2_SERIALIZATION == COMPRESSED);
    assert(len >= G2_SER_BYTES);

    // attempt to deserilize a compressed E2 point from input bytes
    // after fixing the header 2 bits
    byte copy[G2_SER_BYTES];
    memcpy(copy, bytes, sizeof(copy));
    copy[0] |= 1<<7;        // set compression bit
    copy[0] &= ~(1<<6);     // clear infinity bit - point is not infinity

    BLST_ERROR ser = E2_read_bytes(p, copy, G2_SER_BYTES);
    if (ser != BLST_SUCCESS) {
        return ser;
    }

    // map the point to E2\G2 by clearing G2 order
    E2_mult(p, p, (const Fr*)BLS12_381_r);
    E2_to_affine(p, p);

    assert(E2_affine_on_curve(p));  // sanity check to make sure p is in E2
    return BLST_SUCCESS;
}

// This is a testing function.
// It wraps a call to a Relic macro since cgo can't call macros.
void xmd_sha256(byte *hash, int len_hash, byte *msg, int len_msg, byte *dst, int len_dst){
    md_xmd_sh256(hash, len_hash, msg, len_msg, dst, len_dst);
}


// DEBUG printing functions 
#define DEBUG 1
#if DEBUG==1
void bytes_print_(char* s, byte* data, int len) {
    printf("[%s]:\n", s);
    for (int i=0; i<len; i++) 
        printf("%02X,", data[i]);
    printf("\n");
}

void Fr_print_(char* s, Fr* a) {
    printf("[%s]:\n", s);
    limb_t* p = (limb_t*)(a) + Fr_LIMBS;
    for (int i=0; i<Fr_LIMBS; i++) 
        printf("%016llX", *(--p));
    printf("\n");
}

void Fp_print_(char* s, const Fp* a) {
    Fp tmp;
    Fp_from_montg(&tmp, a);
    printf("[%s]:\n", s);
    limb_t* p = (limb_t*)(&tmp) + Fp_LIMBS;
    for (int i=0; i<Fp_LIMBS; i++) 
        printf("%016llX ", *(--p));
    printf("\n");
}

void Fp2_print_(char* s, const Fp2* a) {
    printf("[%s]:\n", s);
    Fp tmp;
    Fp_from_montg(&tmp, &real(a));
    limb_t* p = (limb_t*)(&tmp) + Fp_LIMBS;
    for (int i=0; i<Fp_LIMBS; i++) 
        printf("%016llX", *(--p));
    printf("\n");
    Fp_from_montg(&tmp, &imag(a));
    p = (limb_t*)(&tmp) + Fp_LIMBS;
    for (int i=0; i<Fp_LIMBS; i++) 
        printf("%016llX", *(--p));
    printf("\n");
}

void E1_print_(char* s, const E1* p, const int jacob) {
    E1 a; E1_copy(&a, p);
    if (!jacob) E1_to_affine(&a, &a);
    printf("[%s]:\n", s);
    Fp_print_(".x", &(a.x));
    Fp_print_(".y", &(a.y));
    if (jacob) Fp_print_(".z", &(a.z));
}

void E2_print_(char* s, const E2* p, const int jacob) {
    E2 a; E2_copy(&a, p);
    if (!jacob) E2_to_affine(&a, &a);
    printf("[%s]:\n", s);
    Fp2_print_(".x", &(a.x));
    Fp2_print_(".y", &(a.y));
    if (jacob) Fp2_print_(".z", &(a.z));
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
#endif
