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
 * Implementation of the prime elliptic curve utilities.
 *
 * @version $Id$
 * @ingroup ep
 */

#include "relic_core.h"
#include "relic_ep2.h"
#include "relic_error.h"
#include "relic_conf.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

int ep2_is_infinity(ep2_t p) {
	return (fp2_is_zero(p->z) == 1);
}

void ep2_set_infinity(ep2_t p) {
	fp2_zero(p->x);
	fp2_zero(p->y);
	fp2_zero(p->z);
}

void ep2_copy(ep2_t r, ep2_t p) {
	fp2_copy(r->x, p->x);
	fp2_copy(r->y, p->y);
	fp2_copy(r->z, p->z);
	r->norm = p->norm;
}

int ep2_cmp(ep2_t p, ep2_t q) {
	if (fp2_cmp(p->x, q->x) != RLC_EQ) {
		return RLC_NE;
	}

	if (fp2_cmp(p->y, q->y) != RLC_EQ) {
		return RLC_NE;
	}

	if (fp2_cmp(p->z, q->z) != RLC_EQ) {
		return RLC_NE;
	}

	return RLC_EQ;
}

void ep2_rand(ep2_t p) {
	bn_t n, k;

	bn_new(k);

	n = ep2_curve_get_ord();

	bn_rand(k, BN_POS, bn_bits(n));
	bn_mod_basic(k, k, n);

	//ep2_mul(p, ep2_curve_get_gen(), k);

	bn_free(k);
}

void ep2_print(ep2_t p) {
	fp2_print(p->x);
	fp2_print(p->y);
	fp2_print(p->z);
}
