/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007, 2008, 2009 RELIC Authors
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
 * @ingroup fb
 */

#include <stdlib.h>
#include <stdio.h>
#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_dv.h"
#include "relic_fp.h"
#include "relic_fp_low.h"
#include "relic_error.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp_param_set(int param) {
	bn_t t0, t1, p;
	int f[10] = { 0 };

	TRY {
		bn_new(t0);
		bn_new(t1);
		bn_new(p);

		switch (param) {
			case SECG_160:
				/* p = 2^160 - 2^31 + 1. */
				f[0] = -1;
				f[1] = -31;
				fp_prime_set_spars(f, 2);
				break;
			case NIST_192:
				/* p = 2^192 - 2^64 - 1. */
				f[0] = -1;
				f[1] = -64;
				fp_prime_set_spars(f, 2);
				break;
			case NIST_224:
				/* p = 2^224 - 2^96 + 1. */
				f[0] = 1;
				f[1] = -96;
				fp_prime_set_spars(f, 2);
				break;
			case NIST_256:
				/* p = 2^256 - 2^224 + 2^192 + 2^96 - 1. */
				f[0] = -1;
				f[1] = 96;
				f[2] = 192;
				f[3] = -224;
				fp_prime_set_spars(f, 4);
				break;
			case BNN_256:
				/* x = -4080000000000001. */
				bn_set_2b(t0, 62);
				bn_set_2b(t1, 55);
				bn_add(t0, t0, t1);
				bn_add_dig(t0, t0, 1);
				bn_neg(t0, t0);
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
			case BNP_256:
				/* x = 6000000000001F2D. */
				bn_set_2b(t0, 62);
				bn_set_2b(t1, 61);
				bn_add(t0, t0, t1);
				bn_set_dig(t1, 0x1F);
				bn_lsh(t1, t1, 8);
				bn_add(t0, t0, t1);
				bn_add_dig(t0, t0, 0x2D);
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
			case NIST_384:
				/* p = 2^384 - 2^128 - 2^96 + 2^32 - 1. */
				f[0] = -1;
				f[1] = 32;
				f[2] = -96;
				f[3] = -128;
				fp_prime_set_spars(f, 4);
				break;
			case NIST_521:
				/* p = 2^521 - 1. */
				f[0] = -1;
				fp_prime_set_spars(f, 1);
				break;
			default:
				THROW(ERR_NO_FIELD);
				break;
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(t0);
		bn_free(t1);
		bn_free(p);
	}
}

void fp_param_set_any(void) {
#if FP_PRIME == 160
	fp_param_set(SECG_160);
#elif FP_PRIME == 192
	fp_param_set(NIST_192);
#elif FP_PRIME == 224
	fp_param_set(NIST_224);
#elif FP_PRIME == 256
#ifdef FP_PMERS
	fp_param_set(NIST_256);
#else
	fp_param_set(BNN_256);
#endif
#elif FP_PRIME == 384
	fp_param_set(NIST_384);
#elif FP_PRIME == 521
	fp_param_set(NIST_521);
#else
	bn_t modulus;

	bn_null(modulus);

	TRY {
		bn_new(modulus);
		bn_gen_prime(modulus, FP_BITS);
		fp_prime_set_dense(modulus);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		bn_free(modulus);
	}
#endif
}

void fp_param_print(void) {
	util_print_banner("Prime modulus:", 0);
	util_print("   ");
	fp_print(fp_prime_get());
}
