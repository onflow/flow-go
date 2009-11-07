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
 * Implementation of the low-level prime field modular reduction functions.
 *
 * @version $Id$
 * @ingroup fp
 */

#include <gmp.h>

#include "relic_core.h"
#include "relic_fp.h"
#include "relic_fp_low.h"
#include "relic_bn_low.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp_rdcs_low(dig_t *c, dig_t *a, dig_t *m) {
	align dig_t q[2 * FP_DIGS], _q[2 * FP_DIGS];
	align dig_t _r[2 * FP_DIGS], r[2 * FP_DIGS], t[2 * FP_DIGS];
	int *sform, len;
	int first, i, j, b0, d0, b1, d1;
	dig_t carry;

	sform = fp_prime_get_spars(&len);

	SPLIT(b0, d0, FP_BITS, FP_DIG_LOG);
	first = (d0) + (b0 == 0 ? 0 : 1);

	/* q = floor(a/b^k) */
	dv_zero(q, 2 * FP_DIGS);
	bn_rshd_low(q, a, 2 * FP_DIGS, d0);
	bn_rshb_low(q, q, 2 * FP_DIGS, b0);

	/* r = a - qb^k. */
	dv_copy(r, a, first);
	if (b0 > 0) {
		r[first - 1] &= MASK(b0);
	}

	carry = 0;
	while (!fp_is_zero(q)) {
		dv_zero(_q, 2 * FP_DIGS);
		for (i = len - 1; i > 0; i--) {
			j = (sform[i] < 0 ? -sform[i] : sform[i]);
			SPLIT(b1, d1, j, FP_DIG_LOG);
			dv_zero(t, 2 * FP_DIGS);
			bn_lshd_low(t, q, FP_DIGS, d1);
			bn_lshb_low(t, t, 2 * FP_DIGS, b1);
			if (sform[i] > 0) {
				bn_subn_low(_q, _q, t, 2 * FP_DIGS);
			} else {
				bn_addn_low(_q, _q, t, 2 * FP_DIGS);
			}
		}
		if (sform[0] > 0) {
			bn_subn_low(_q, _q, q, 2 * FP_DIGS);
		} else {
			bn_addn_low(_q, _q, q, 2 * FP_DIGS);
		}
		bn_rshd_low(q, _q, 2 * FP_DIGS, d0);
		bn_rshb_low(q, q, 2 * FP_DIGS, b0);

		dv_copy(_r, _q, first);
		if (b0 > 0) {
			_r[first - 1] &= MASK(b0);
		}
		fp_addn_low(r, r, _r);
	}
	while (fp_cmpn_low(r, m) != CMP_LT) {
		fp_subn_low(r, r, m);
	}
	fp_copy(c, r);
}

dig_t fp_rdcn_low(dig_t *c, dig_t *a, dig_t *m, dig_t u) {
	int i;
	dig_t r, carry, *tmp;

	tmp = a;

	for (i = 0; i < FP_DIGS; i++, tmp++) {
		r = (dig_t)(*tmp * u);
		carry = mpn_addmul_1(tmp, m, FP_DIGS, r);
		carry = mpn_add_1(tmp + FP_DIGS, tmp + FP_DIGS, FP_DIGS - i, carry);
	}
	for (i = 0; i < FP_DIGS; i++, c++, tmp++) {
		*c = *tmp;
	}
	return carry;
}
