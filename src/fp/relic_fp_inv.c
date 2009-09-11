/*
 * Copyright 2007 Project RELIC
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file.
 *
 * RELIC is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
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
	fp_t t = NULL;
	bn_t _a = NULL, _p = NULL, u = NULL, v = NULL, x1 = NULL, x2 = NULL;
	dig_t *p = NULL, carry;
	int i, k, flag = 0;

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

#if FP_RDC != MONTY
		_a->used = FP_DIGS;
		_p->used = FP_DIGS;
		dv_copy(_a->dp, a, FP_DIGS);
		dv_copy(_p->dp, p, FP_DIGS);
		bn_mod_monty_conv(u, _a, _p);
#else
		u->used = FP_DIGS;
		dv_copy(u->dp, a, FP_DIGS);
#endif
		v->used = FP_DIGS;
		dv_copy(v->dp, p, FP_DIGS);

		k = 0;
		bn_set_dig(x1, 1);
		bn_zero(x2);
		while (!bn_is_zero(v)) {
			if (bn_is_even(v)) {
				bn_rsh(v, v, 1);
				bn_lsh(x1, x1, 1);
			} else {
				if (bn_is_even(u)) {
					bn_rsh(u, u, 1);
					bn_lsh(x2, x2, 1);
				} else {
					if (bn_cmp(v, u) != CMP_LT) {
						bn_sub(v, v, u);
						bn_rsh(v, v, 1);
						bn_add(x2, x2, x1);
						bn_lsh(x1, x1, 1);
					} else {
						bn_sub(u, u, v);
						bn_rsh(u, u, 1);
						bn_add(x1, x1, x2);
						bn_lsh(x2, x2, 1);
					}
				}
			}
			k++;
		}

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
			fp_sub(x1->dp, x1->dp, fp_prime_get());
		}

		if (k < FP_DIGS * FP_DIGIT) {
			flag = 1;
			printf("dentro\n");
			fp_mul(x1->dp, x1->dp, fp_prime_get_conv());
			k = k + FP_DIGS * FP_DIGIT;
		}

		fp_mul(x1->dp, x1->dp, fp_prime_get_conv());
		fp_zero(t);
		t[0] = 1;
		fp_lsh(t, t, 2 * FP_DIGS * FP_DIGIT - k);
		fp_mul(c, x1->dp, t);

#if FP_RDC != MONTY
		/* If we do not use Montgomery reduction, the result of inversion is
		 * a^{-1}R^3 mod p or a^{-1}R^4 mod p, depending on flag.
		 * Hence we must reduce the result three times.
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
