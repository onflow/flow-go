
#include "include.h"

#define DOUBADD 0 
#define SLIDW   1 
#define MONTG   2 
#define PRECOM  3 
#define WREG    4 
#define WNAF    5 
#define RELIC   6 

#define G1MULT RELIC
#define G2MULT RELIC

// Initializes Relic context with BLS12-381 parameters
ctx_t* _relic_init_BLS12_381() { 
    // check Relic was compiled with the right conf 
    if (ALLOC != AUTO) return NULL;

    // init relic core
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

int _getSignatureLengthBLS_BLS12381() {
    return SIGNATURE_LEN;
}

int _getPubKeyLengthBLS_BLS12381() {
    return PK_LEN;
}

int _getPrKeyLengthBLS_BLS12381() {
    return SK_LEN;
}


static void ep_swu_b12(ep_t p, const fp_t t, int u, int negate) {
	fp_t t0, t1, t2, t3;

	fp_null(t0);
	fp_null(t1);
	fp_null(t2);
	fp_null(t3);

	TRY {
		fp_new(t0);
		fp_new(t1);
		fp_new(t2);
		fp_new(t3);

		/* t0 = t^2. */
		fp_sqr(t0, t);
		/* Compute f(u) such that u^3 + b is a square. */
		fp_set_dig(p->x, -u);
		fp_neg(p->x, p->x);
		ep_rhs(t1, p);
		/* Compute t1 = (-f(u) + t^2), t2 = t1 * t^2 and invert if non-zero. */
		fp_add(t1, t1, t0);
		fp_mul(t2, t1, t0);
		if (!fp_is_zero(t2)) {
			/* Compute inverse of u^3 * t2 and fix later. */
			fp_mul(t2, t2, p->x);
			fp_mul(t2, t2, p->x);
			fp_mul(t2, t2, p->x);
			fp_inv(t2, t2);
		}
		/* Compute t0 = t^4 * u * sqrt(-3)/t2. */
		fp_sqr(t0, t0);
		fp_mul(t0, t0, t2);
		fp_mul(t0, t0, p->x);
		fp_mul(t0, t0, p->x);
		fp_mul(t0, t0, p->x);
		/* Compute constant u * sqrt(-3). */
		fp_copy(t3, core_get()->srm3);
		for (int i = 1; i < -u; i++) {
			fp_add(t3, t3, core_get()->srm3);
		}
		fp_mul(t0, t0, t3);
		/* Compute (u * sqrt(-3) + u)/2 - t0. */
		fp_add_dig(p->x, t3, -u);
		fp_hlv(p->y, p->x);
		fp_sub(p->x, p->y, t0);
		ep_rhs(p->y, p);
		if (!fp_srt(p->y, p->y)) {
			/* Now try t0 - (u * sqrt(-3) - u)/2. */
			fp_sub_dig(p->x, t3, -u);
			fp_hlv(p->y, p->x);
			fp_sub(p->x, t0, p->y);
			ep_rhs(p->y, p);
			if (!fp_srt(p->y, p->y)) {
				/* Finally, try (u - t1^2 / t2). */
				fp_sqr(p->x, t1);
				fp_mul(p->x, p->x, t1);
				fp_mul(p->x, p->x, t2);
				fp_sub_dig(p->x, p->x, -u);
				ep_rhs(p->y, p);
				fp_srt(p->y, p->y);
			}
		}
		if (negate) {
			fp_neg(p->y, p->y);
		}
		fp_set_dig(p->z, 1);
		p->norm = 1;
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp_free(t0);
		fp_free(t1);
		fp_free(t2);
		fp_free(t3);
	}
}

// Maps a 384 bits number to G1
// Optimized Shallue–van de Woestijne encoding from Section 3 of
// "Fast and simple constant-time hashing to the BLS12-381 elliptic curve".
// taken and modified from Relic library
void mapToG1_swu(ep_t p, const uint8_t *digest, int len) {
	bn_t k, pm1o2;
	fp_t t;
	ep_t q;
	uint8_t sec_digest[RLC_MD_LEN_SH384];
	int neg;

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
		bn_read_bin(k, digest, RLC_MIN(RLC_FP_BYTES, len));
		fp_prime_conv(t, k);
		fp_prime_back(k, t);
		neg = (bn_cmp(k, pm1o2) == RLC_LT ? 0 : 1);

        ep_swu_b12(p, t, -3, neg);
        md_map_sh384(sec_digest, digest, len);
        bn_read_bin(k, sec_digest, RLC_MIN(RLC_FP_BYTES, RLC_MD_LEN_SH384));
        fp_prime_conv(t, k);
        neg = (bn_cmp(k, pm1o2) == RLC_LT ? 0 : 1);
        ep_swu_b12(q, t, -3, neg);
        ep_add(p, p, q);
        ep_norm(p, p);
        /* Now, multiply by cofactor to get the correct group. */
        fp_prime_get_par(k);
        bn_neg(k, k);
        bn_add_dig(k, k, 1);
        if (bn_bits(k) < RLC_DIG) {
            ep_mul_dig(p, p, k->dp[0]);
        } else {
            ep_mul(p, p, k);
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


// Exponentiation of random p in G1
void _G1scalarPointMult(ep_st* res, ep_st* p, bn_st *expo) {
    // Using window NAF of size 2
    #if (EP_MUL	== LWNAF)
        g1_mul(res, p, expo);
    #else 
        ep_mul_lwnaf(res, p, expo);
    #endif
}

// Exponentiation of fixed g1 in G1
// This function is not called by BLS but is here for DEBUG/TESTs purposes
void _G1scalarGenMult(ep_st* res, bn_st *expo) {
#define GENERIC_POINT 1
#define FIXED_MULT    (GENERIC_POINT^1)

#if GENERIC_POINT
    _G1scalarPointMult(res, &core_get()->ep_g, expo);
#elif FIXED_MULT
    // Using precomputed table of size 4
    g1_mul_gen(res, expo);
#endif
}

// Exponentiation of random p in G2
void _G2scalarPointMult(ep2_st* res, ep2_st* p, bn_st *expo) {
    // Using window NAF of size 2
    #if (EP_MUL	== LWNAF)
        g2_mul(res, p, expo);
    #else 
        ep2_mul_lwnaf(res, p, expo);
    #endif
    
}

// Exponentiation of fixed g2 in G2
void _G2scalarGenMult(ep2_st* res, bn_st *expo) {
    // Using precomputed table of size 4
    g2_mul_gen(res, expo);
}


// Computes BLS signature
void _blsSign(byte* s, bn_st *sk, byte* data, int len) {
    ep_st h;
    ep_new(&h);
    // hash to G1
    mapToG1_swu(&h, data, len); 
    // s = p^sk
	_G1scalarPointMult(&h, &h, sk);  
    _ep_write_bin_compact(s, &h);

    ep_free(&p);
}

// Verifies the validity of a BLS signature
int _blsVerify(ep2_st *pk, byte* sig, byte* data, int len) { 
    // TODO : check pk is on curve  (should be done offline)
	// TODO : check pk is in G2 (should be done offline) 
    ep_t elemsG1[2];
    ep2_t elemsG2[2];

    // elemsG1[0] = s
    ep_new(elemsG1[0]);
    _ep_read_bin_compact(elemsG1[0], sig);

 #if MEMBERSHIP_CHECK
    // check s is on curve
	// check s is in G1
    if (!ep_is_valid(elemsG1[0]))
        return SIG_INVALID;
    ep_st inf;
    ep_new(&inf);
    // check s^order == infinity
    // use basic double & add as lwnaf reduces the expo modulo r
    // TODO : write a simple lwnaf without reduction
    ep_mul_basic(&inf, elemsG1[0], &core_get()->ep_r);
    if (!ep_is_infty(&inf)){
        ep_free(&inf);
        return SIG_INVALID;
    }
    ep_free(&inf);
 #endif

    // elemsG1[1] = h
    ep_new(elemsG1[1]);
    // hash to G1 
    mapToG1_swu(elemsG1[1], data, len); 

    // elemsG2[1] = pk
    ep2_new(elemsG2[1]);
    ep2_copy(elemsG2[1], pk);

#if DOUBLE_PAIRING  
    // elemsG2[0] = -g2
    ep2_new(&elemsG2[0]);
    ep2_neg(elemsG2[0], &core_get()->ep2_g); // could be hardcoded 

    fp12_t pair;
    fp12_new(&pair);
    // double pairing with Optimal Ate 
    pp_map_sim_oatep_k12(pair, (ep_t*)(elemsG1) , (ep2_t*)(elemsG2), 2);

    // compare the result to 1
    int res = fp12_cmp_dig(pair, 1);

#elif SINGLE_PAIRING   
    fp12_t pair1, pair2;
    fp12_new(&pair1); fp12_new(&pair2);
    pp_map_oatep_k12(pair1, elemsG1[0], &core_get()->ep2_g);
    pp_map_oatep_k12(pair2, elemsG1[1], elemsG2[1]);

    int res = fp12_cmp(pair1, pair2);
#endif
    fp12_free(&one);
    ep_new(elemsG1[0]);
    ep_new(elemsG1[1]);
    ep2_new(elemsG2[0]);
    ep2_new(elemsG2[1]);
    
    if (res == RLC_EQ && core_get()->code == RLC_OK) 
        return SIG_VALID;
    else 
        return SIG_INVALID;
}
