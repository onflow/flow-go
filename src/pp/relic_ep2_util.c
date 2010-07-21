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
 * Implementation of utilities for prime elliptic curves over quadratic
 * extensions.
 *
 * @version $Id: relic_pp_ep2.c 463 2010-07-13 21:12:13Z conradoplg $
 * @ingroup pp
 */

#include "relic_core.h"
#include "relic_md.h"
#include "relic_pp.h"
#include "relic_error.h"
#include "relic_conf.h"
#include "relic_fp_low.h"

/*============================================================================*/
	/* Public definitions                                                         */
/*============================================================================*/

int ep2_is_infty(ep2_t p) {
	return (fp2_is_zero(p->z) == 1);
}

void ep2_set_infty(ep2_t p) {
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
	if (fp2_cmp(p->x, q->x) != CMP_EQ) {
		return CMP_NE;
	}

	if (fp2_cmp(p->y, q->y) != CMP_EQ) {
		return CMP_NE;
	}

	if (fp2_cmp(p->z, q->z) != CMP_EQ) {
		return CMP_NE;
	}

	return CMP_EQ;
}

void ep2_rand(ep2_t p) {
	bn_t n, k;
	ep2_t gen;

	bn_null(k);
	bn_null(n);
	ep2_null(gen);

	TRY {
		bn_new(k);
		bn_new(n);
		ep2_new(gen);

		ep2_curve_get_ord(n);

		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);

		ep2_curve_get_gen(gen);
		ep2_mul(p, gen, k);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(k);
		bn_free(n);
		ep2_free(gen);
	}
}

void ep2_print(ep2_t p) {
	fp2_print(p->x);
	fp2_print(p->y);
	fp2_print(p->z);
}
