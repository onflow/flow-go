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
#include "relic_ep.h"
#include "relic_error.h"
#include "relic_conf.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

int ep_is_infty(ep_t p) {
	return (fp_is_zero(p->z) == 1);
}

void ep_set_infty(ep_t p) {
	fp_zero(p->x);
	fp_zero(p->y);
	fp_zero(p->z);
}

void ep_copy(ep_t r, ep_t p) {
	fp_copy(r->x, p->x);
	fp_copy(r->y, p->y);
	fp_copy(r->z, p->z);
	r->norm = p->norm;
}

int ep_cmp(ep_t p, ep_t q) {
	if (fp_cmp(p->x, q->x) != RLC_EQ) {
		return RLC_NE;
	}

	if (fp_cmp(p->y, q->y) != RLC_EQ) {
		return RLC_NE;
	}

	if (fp_cmp(p->z, q->z) != RLC_EQ) {
		return RLC_NE;
	}

	return RLC_EQ;
}

void ep_rand(ep_t p) {
	bn_t n = NULL, k = NULL;

	TRY {
		bn_new(k);

		n = ep_curve_get_ord();

		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod_basic(k, k, n);

		ep_mul(p, ep_curve_get_gen(), k);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		bn_free(k);
	}
}

void ep_print(ep_t p) {
	fp_print(p->x);
	fp_print(p->y);
	fp_print(p->z);
}
