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
 * Implementation of the prime field inversion functions.
 *
 * @version $Id$
 * @ingroup fp
 */

#include "relic_core.h"
#include "relic_fp.h"
#include "relic_fp_low.h"
#include "relic_bn_low.h"
#include "relic_util.h"
#include "relic_rand.h"
#include "relic_error.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp_inv(fp_t c, fp_t a) {
	fp_t t;
	bn_t _a, _p, u, v, x1, x2;
	dig_t *p = NULL, carry;
	int i, k, flag = 0;

	fp_null(t);
	bn_null(_a);
	bn_null(_p);
	bn_null(u);
	bn_null(v);
	bn_null(x1);
	bn_null(x2);

	TRY {
		fp_new(t);
		bn_new(_a);
		bn_new(_p);
		bn_new(u);
		bn_new(v);
		bn_new(x1);
		bn_new(x2);

		p = fp_prime_get();
		fp_zero(t);

		/* u = a, v = p, x1 = 1, x2 = 0, k = 0. */
		k = 0;
		bn_set_dig(x1, 1);
		bn_zero(x2);

#if FP_RDC != MONTY
		_a->used = FP_DIGS;
		dv_copy(_a->dp, a, FP_DIGS);
		bn_trim(_a);
		_p->used = FP_DIGS;
		dv_copy(_p->dp, p, FP_DIGS);
		bn_trim(_p);
		bn_mod_monty_conv(u, _a, _p);
#else
		u->used = FP_DIGS;
		dv_copy(u->dp, a, FP_DIGS);
		bn_trim(u);
#endif
		v->used = FP_DIGS;
		dv_copy(v->dp, p, FP_DIGS);
		bn_trim(v);

		while (!bn_is_zero(v)) {
			/* If v is even then v = v/2, x1 = 2 * x1. */
			if (bn_is_even(v)) {
				bn_hlv(v, v);
				bn_dbl(x1, x1);
			} else {
				/* If u is even then u = u/2, x2 = 2 * x2. */
				if (bn_is_even(u)) {
					bn_hlv(u, u);
					bn_dbl(x2, x2);
					/* If v >= u,then v = (v - u)/2, x2 += x1, x1 = 2 * x1. */
				} else {
					if (bn_cmp(v, u) != CMP_LT) {
						bn_sub(v, v, u);
						bn_hlv(v, v);
						bn_add(x2, x2, x1);
						bn_dbl(x1, x1);
					} else {
						/* u = (u - v)/2, x1 += x2, x2 = 2 * x2. */
						bn_sub(u, u, v);
						bn_hlv(u, u);
						bn_add(x1, x1, x2);
						bn_dbl(x2, x2);
					}
				}
			}
			k++;
		}

		/* If x1 > p then x1 = x1 - p. */
		for (i = x1->used; i < FP_DIGS; i++) {
			x1->dp[i] = 0;
		}

		while (x1->used > FP_DIGS) {
			carry = bn_subn_low(x1->dp, x1->dp, fp_prime_get(), FP_DIGS);
			bn_sub1_low(x1->dp + FP_DIGS, x1->dp + FP_DIGS, carry,
					x1->used - FP_DIGS);
			bn_trim(x1);
		}
		if (fp_cmpn_low(x1->dp, fp_prime_get()) == CMP_GT) {
			fp_subn_low(x1->dp, x1->dp, fp_prime_get());
		}

		/* If k < Wt then x1 = x1 * R^2 * R^{-1} mod p. */
		if (k < FP_DIGS * FP_DIGIT) {
			flag = 1;
			fp_mul(x1->dp, x1->dp, fp_prime_get_conv());
			k = k + FP_DIGS * FP_DIGIT;
		}
		/* x1 = x1 * R^2 * R^{-1} mod p. */
		fp_mul(x1->dp, x1->dp, fp_prime_get_conv());
		fp_zero(t);
		t[0] = 1;
		if (k > FP_DIGS * FP_DIGIT) {
			fp_lsh(t, t, 2 * FP_DIGS * FP_DIGIT - k);
			/* c = x1 * 2^{2Wt - k} * R^{-1} mod p. */
			fp_mul(c, x1->dp, t);
		} else {
			fp_prime_conv_dig(c, 1);
		}
#if FP_RDC != MONTY
		/*is_one
		 * If we do not use Montgomery reduction, the result of inversion is
		 * a^{-1}R^3 mod p or a^{-1}R^4 mod p, depending on flag.
		 * Hence we must reduce the result three or four times.
		 */
		_a->used = FP_DIGS;
		dv_copy(_a->dp, c, FP_DIGS);
		bn_mod_monty_back(_a, _a, _p);
		bn_mod_monty_back(_a, _a, _p);
		bn_mod_monty_back(_a, _a, _p);

		if (flag) {
			bn_mod_monty_back(_a, _a, _p);
		}
		fp_zero(c);
		dv_copy(c, _a->dp, _a->used);
#endif
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp_free(t);
		bn_free(_a);
		bn_free(_p);
		bn_free(u);
		bn_free(v);
		bn_free(x1);
		bn_free(x2);
	}
}
