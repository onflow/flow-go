/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2012 RELIC Authors
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file
 * for contact information.
 *
 * RELIC is free software; you can redistribute it and/or
 * modify it under the terms of the GNU Lesser General Public
 * License as published by the Free Software Foundation; either
 * version 2.1 of the License, or (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @file
 *
 * Implementation of the prime field modulus manipulation.
 *
 * @version $Id$
 * @ingroup fp
 */

#include "relic_core.h"
#include "relic_fpx.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

int fp_param_get(void) {
	return core_get()->fp_id;
}

void fp_param_get_var(bn_t x) {
	bn_t a;

	bn_null(a);

	TRY {
		bn_new(a);

		switch (fp_param_get()) {
			case BN_158:
				/* x = 4000000031. */
				bn_set_2b(x, 38);
				bn_add_dig(x, x, 0x31);
				break;
			case BN_254:
				/* x = -4080000000000001. */
				bn_set_2b(x, 62);
				bn_set_2b(a, 55);
				bn_add(x, x, a);
				bn_add_dig(x, x, 1);
				bn_neg(x, x);
				break;
			case BN_256:
				/* x = -600000000000219B. */
				bn_set_2b(x, 62);
				bn_set_2b(a, 61);
				bn_add(x, x, a);
				bn_set_dig(a, 0x21);
				bn_lsh(a, a, 8);
				bn_add(x, x, a);
				bn_add_dig(x, x, 0x9B);
				bn_neg(x, x);
				break;
			case KSS_508:
				/* x = -(2^64 + 2^51 - 2^46 - 2^12). */
				bn_set_2b(x, 64);
				bn_set_2b(a, 51);
				bn_add(x, x, a);
				bn_set_2b(a, 46);
				bn_sub(x, x, a);
				bn_set_2b(a, 12);
				bn_sub(x, x, a);
				bn_neg(x, x);
				break;
			case BN_638:
				/* x = 2^158 - 2^128 - 2^68 + 1. */
				bn_set_2b(x, 158);
				bn_set_2b(a, 128);
				bn_sub(x, x, a);
				bn_set_2b(a, 68);
				bn_sub(x, x, a);
				bn_add_dig(x, x, 1);
				break;
			case BLS12_638:
				/* x = -2^107 + 2^105 + 2^93 + 2^5. */
				bn_set_2b(x, 107);
				bn_set_2b(a, 105);
				bn_sub(x, x, a);
				bn_set_2b(a, 93);
				bn_sub(x, x, a);
				bn_set_2b(a, 5);
				bn_sub(x, x, a);
				bn_neg(x, x);
				break;
			default:
				THROW(ERR_NO_VALID);
				break;
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(a);
	}
}

int *fp_param_get_sps(int *len) {
	ctx_t *ctx = core_get();
	int *ptr = NULL, *var = ctx->var;
	bn_t a;

	bn_null(a);

	TRY {
		bn_new(a);

		*len = 0;

		switch (fp_param_get()) {
			case BN_158:
			case BN_254:
			case BN_256:
				fp_param_get_var(a);
				if (bn_sign(a) == BN_NEG) {
					bn_neg(a, a);
				}
				*len = bn_ham(a);
				for (int i = 0, j = 0; j < bn_bits(a); j++) {
					if (bn_test_bit(a, j)) {
						var[i++] = j;
					}
				}
				break;
			case KSS_508:
				var[0] = -12;
				var[1] = -46;
				var[2] = 51;
				var[3] = 64;
				*len = 4;
				break;
			case BN_638:
				var[0] = 0;
				var[1] = -68;
				var[2] = -128;
				var[3] = 158;
				*len = 4;
				break;
			case BLS12_638:
				var[0] = -5;
				var[1] = -93;
				var[2] = -105;
				var[3] = 107;
				*len = 4;
				break;
			default:
				THROW(ERR_NO_VALID);
				break;
		}

		if (*len > 0 && *len < MAX_TERMS ) {
			ptr = var;
		} else {
			ptr = NULL;
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(a);
	}

	return ptr;
}

void fp_param_get_map(int *s, int *len) {
	for (int i = 0; i < FP_BITS; i++) {
		s[i] = 0;
	}

	switch (fp_param_get()) {
		case BN_158:
			s[3] = s[5] = s[8] = s[39] = s[40] = 1;
			*len = 41;
			break;
		case BN_254:
			s[2] = s[56] = s[57] = s[63] = s[64] = 1;
			*len = 65;
			break;
		case BN_256:
			s[5] = s[7] = s[8] = s[11] = s[14] = s[15] = s[62] = s[65] = 1;
			*len = 66;
			break;
		case BN_638:
			s[3] = s[159] = s[160] = 1;
			s[69] = s[70] = s[129] = s[130] = -1;
			*len = 161;
			break;
		case KSS_508:
			s[64] = s[51] = 1;
			s[12] = s[46] = -1;
			*len = 65;
			break;
		case BLS12_638:
			s[5] = s[93] = s[105] = -1;
			s[107] = 1;
			*len = 108;
			break;
		default:
			THROW(ERR_NO_VALID);
			break;
	}
}

void fp_param_set(int param) {
	bn_t t0, t1, t2, p;
	int f[10] = { 0 };

	/* Suppress possible unused parameter warning. */
	(void) f;

	TRY {
		bn_new(t0);
		bn_new(t1);
		bn_new(t2);
		bn_new(p);

		core_get()->fp_id = param;

		switch (param) {
#if FP_PRIME == 158
			case BN_158:
				/* x = 4000000031. */
				fp_param_get_var(t0);
				/* p = 36 * x^4 + 36 * x^3 + 24 * x^2 + 6 * x + 1. */
				bn_set_dig(p, 1);
				bn_mul_dig(t1, t0, 6);
				bn_add(p, p, t1);
				bn_mul(t1, t0, t0);
				bn_mul_dig(t1, t1, 24);
				bn_add(p, p, t1);
				bn_mul(t1, t0, t0);
				bn_mul(t1, t1, t0);
				bn_mul_dig(t1, t1, 36);
				bn_add(p, p, t1);
				bn_mul(t0, t0, t0);
				bn_mul(t1, t0, t0);
				bn_mul_dig(t1, t1, 36);
				bn_add(p, p, t1);
				fp_prime_set_dense(p);
				break;
#elif FP_PRIME == 160
			case SECG_160:
				/* p = 2^160 - 2^31 + 1. */
				f[0] = -1;
				f[1] = -31;
				fp_prime_set_pmers(f, 2);
				break;
			case SECG_160D:
				/* p = 2^160 - 2^32 - 2^14 - 2^12 - 2^9 - 2^8 - 2^7 - 2^3 - 2^2 - 1.*/
				f[0] = -1;
				f[1] = -2;
				f[2] = -3;
				f[3] = -7;
				f[4] = -8;
				f[5] = -9;
				f[6] = -12;
				f[7] = -14;
				f[8] = -32;
				fp_prime_set_pmers(f, 9);
				break;
#elif FP_PRIME == 192
			case NIST_192:
				/* p = 2^192 - 2^64 - 1. */
				f[0] = -1;
				f[1] = -64;
				fp_prime_set_pmers(f, 2);
				break;
			case SECG_192:
				/* p = 2^192 - 2^32 - 2^12 - 2^8 - 2^7 - 2^6 - 2^3 - 1.*/
				f[0] = -1;
				f[1] = -3;
				f[2] = -6;
				f[3] = -7;
				f[4] = -8;
				f[5] = -12;
				f[6] = -32;
				fp_prime_set_pmers(f, 7);
				break;
#elif FP_PRIME == 224
			case NIST_224:
				/* p = 2^224 - 2^96 + 1. */
				f[0] = 1;
				f[1] = -96;
				fp_prime_set_pmers(f, 2);
				break;
			case SECG_224:
				/* p = 2^224 - 2^32 - 2^12 - 2^11 - 2^9 - 2^7 - 2^4 - 2 - 1.*/
				f[0] = -1;
				f[1] = -1;
				f[2] = -4;
				f[3] = -7;
				f[4] = -9;
				f[5] = -11;
				f[6] = -12;
				f[7] = -32;
				fp_prime_set_pmers(f, 8);
				break;
#elif FP_PRIME == 254
			case BN_254:
				/* x = -4080000000000001. */
				fp_param_get_var(t0);
				/* p = 36 * x^4 + 36 * x^3 + 24 * x^2 + 6 * x + 1. */
				bn_set_dig(p, 1);
				bn_mul_dig(t1, t0, 6);
				bn_add(p, p, t1);
				bn_mul(t1, t0, t0);
				bn_mul_dig(t1, t1, 24);
				bn_add(p, p, t1);
				bn_mul(t1, t0, t0);
				bn_mul(t1, t1, t0);
				bn_mul_dig(t1, t1, 36);
				bn_add(p, p, t1);
				bn_mul(t0, t0, t0);
				bn_mul(t1, t0, t0);
				bn_mul_dig(t1, t1, 36);
				bn_add(p, p, t1);
				fp_prime_set_dense(p);
				break;
#elif FP_PRIME == 256
			case NIST_256:
				/* p = 2^256 - 2^224 + 2^192 + 2^96 - 1. */
				f[0] = -1;
				f[1] = 96;
				f[2] = 192;
				f[3] = -224;
				fp_prime_set_pmers(f, 4);
				break;
			case SECG_256:
				/* p = 2^256 - 2^32 - 2^9 - 2^8 - 2^7 - 2^6 - 2^4 - 1. */
				f[0] = -1;
				f[1] = -4;
				f[2] = -6;
				f[3] = -7;
				f[4] = -8;
				f[5] = -9;
				f[6] = -32;
				fp_prime_set_pmers(f, 7);
				break;
			case BN_256:
				/* x = 6000000000001F2D. */
				fp_param_get_var(t0);
				/* p = 36 * x^4 + 36 * x^3 + 24 * x^2 + 6 * x + 1. */
				bn_set_dig(p, 1);
				bn_mul_dig(t1, t0, 6);
				bn_add(p, p, t1);
				bn_mul(t1, t0, t0);
				bn_mul_dig(t1, t1, 24);
				bn_add(p, p, t1);
				bn_mul(t1, t0, t0);
				bn_mul(t1, t1, t0);
				bn_mul_dig(t1, t1, 36);
				bn_add(p, p, t1);
				bn_mul(t0, t0, t0);
				bn_mul(t1, t0, t0);
				bn_mul_dig(t1, t1, 36);
				bn_add(p, p, t1);
				fp_prime_set_dense(p);
				break;
#elif FP_PRIME == 384
			case NIST_384:
				/* p = 2^384 - 2^128 - 2^96 + 2^32 - 1. */
				f[0] = -1;
				f[1] = 32;
				f[2] = -96;
				f[3] = -128;
				fp_prime_set_pmers(f, 4);
#elif FP_PRIME == 508
			case KSS_508:
				fp_param_get_var(t0);
				/* h = (49*u^2 + 245 * u + 343)/3 */
				bn_mul_dig(p, t0, 245);
				bn_add_dig(p, p, 200);
				bn_add_dig(p, p, 143);
				bn_sqr(t1, t0);
				bn_mul_dig(t2, t1, 49);
				bn_add(p, p, t2);
				bn_div_dig(p, p, 3);
				/* n = (u^6 + 37 * u^3 + 343)/343. */
				bn_mul(t1, t1, t0);
				bn_mul_dig(t2, t1, 37);
				bn_sqr(t1, t1);
				bn_add(t2, t2, t1);
				bn_add_dig(t2, t2, 200);
				bn_add_dig(t2, t2, 143);
				bn_div_dig(t2, t2, 49);
				bn_div_dig(t2, t2, 7);
				bn_mul(p, p, t2);
				/* t = (u^4 + 16 * u + 7)/7. */
				bn_mul_dig(t1, t0, 16);
				bn_add_dig(t1, t1, 7);
				bn_sqr(t2, t0);
				bn_sqr(t2, t2);
				bn_add(t2, t2, t1);
				bn_div_dig(t2, t2, 7);
				bn_add(p, p, t2);
				bn_sub_dig(p, p, 1);
				fp_prime_set_dense(p);
				break;
#elif FP_PRIME == 521
			case NIST_521:
				/* p = 2^521 - 1. */
				f[0] = -1;
				fp_prime_set_pmers(f, 1);
				break;
#elif FP_PRIME == 638
			case BN_638:
				fp_param_get_var(t0);
				/* p = 36 * x^4 + 36 * x^3 + 24 * x^2 + 6 * x + 1. */
				bn_set_dig(p, 1);
				bn_mul_dig(t1, t0, 6);
				bn_add(p, p, t1);
				bn_mul(t1, t0, t0);
				bn_mul_dig(t1, t1, 24);
				bn_add(p, p, t1);
				bn_mul(t1, t0, t0);
				bn_mul(t1, t1, t0);
				bn_mul_dig(t1, t1, 36);
				bn_add(p, p, t1);
				bn_mul(t0, t0, t0);
				bn_mul(t1, t0, t0);
				bn_mul_dig(t1, t1, 36);
				bn_add(p, p, t1);
				fp_prime_set_dense(p);
				break;
			case BLS12_638:
				fp_param_get_var(t0);
				/* p = (x^2 - 2x + 1) * (x^4 - x^2 + 1)/3 + x. */
				bn_sqr(t1, t0);
				bn_sqr(p, t1);
				bn_sub(p, p, t1);
				bn_add_dig(p, p, 1);
				bn_sub(t1, t1, t0);
				bn_sub(t1, t1, t0);
				bn_add_dig(t1, t1, 1);
				bn_mul(p, p, t1);
				bn_div_dig(p, p, 3);
				bn_add(p, p, t0);
				fp_prime_set_dense(p);
				break;
#else
			default:
				bn_gen_prime(p, FP_BITS);
				fp_prime_set_dense(p);
				core_get()->fp_id = 0;
				break;
#endif
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(t0);
		bn_free(t1);
		bn_free(t2);
		bn_free(p);
	}
}

int fp_param_set_any(void) {
#if FP_PRIME == 158
	fp_param_set(BN_158);
#elif FP_PRIME == 160
	fp_param_set(SECG_160);
#elif FP_PRIME == 192
	fp_param_set(NIST_192);
#elif FP_PRIME == 224
	fp_param_set(NIST_224);
#elif FP_PRIME == 254
	fp_param_set(BN_254);
#elif FP_PRIME == 256
#ifdef FP_PMERS
	fp_param_set(SECG_256);
#else
	fp_param_set(BN_256);
#endif
#elif FP_PRIME == 384
	fp_param_set(NIST_384);
#elif FP_PRIME == 508
	fp_param_set(KSS_508);
#elif FP_PRIME == 521
	fp_param_set(NIST_521);
#elif FP_PRIME == 638
	fp_param_set(BN_638);
#else
	return fp_param_set_any_dense();
#endif
	return STS_OK;
}

int fp_param_set_any_dense() {
	bn_t modulus;
	int result = STS_OK;

	bn_null(modulus);

	TRY {
		bn_new(modulus);
		bn_gen_prime(modulus, FP_BITS);
		if (!bn_is_prime(modulus)) {
			result = STS_ERR;
		} else {
			fp_prime_set_dense(modulus);
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(modulus);
	}
	return result;
}

int fp_param_set_any_pmers(void) {
#if FP_PRIME == 160
	fp_param_set(SECG_160);
#elif FP_PRIME == 192
	fp_param_set(NIST_192);
#elif FP_PRIME == 224
	fp_param_set(NIST_224);
#elif FP_PRIME == 256
	fp_param_set(NIST_256);
#elif FP_PRIME == 384
	fp_param_set(NIST_384);
#elif FP_PRIME == 521
	fp_param_set(NIST_521);
#else
	return STS_ERR;
#endif
	return STS_OK;
}

int fp_param_set_any_tower() {
#if FP_PRIME == 158
	fp_param_set(BN_158);
#elif FP_PRIME == 254
	fp_param_set(BN_254);
#elif FP_PRIME == 256
	fp_param_set(BN_256);
#elif FP_PRIME == 508
	fp_param_set(KSS_508);
#elif FP_PRIME == 638
	fp_param_set(BN_638);
#else
	do {
		/* Since we have to generate a prime number, pick a nice towering. */
		fp_param_set_any_dense();
	} while (fp_prime_get_mod8() == 1 || fp_prime_get_mod8() == 5);
#endif

	return STS_OK;
}

void fp_param_print(void) {
	util_banner("Prime modulus:", 0);
	util_print("   ");
	fp_print(fp_prime_get());
}
