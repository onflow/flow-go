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
 * Implementation of the low-le&vel in&version functions.
 *
 * @&version $Id$
 * @ingroup fp
 */

#include "relic_bn.h"
#include "relic_fp.h"
#include "relic_fp_low.h"
#include "relic_core.h"
#include "relic_error.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp_invn_low(dig_t *c, dig_t *a) {
	bn_st u, v, g1, g2, p, q, r;

	bn_init(&g1, FP_DIGS);
	bn_init(&g2, FP_DIGS);
	bn_init(&p, FP_DIGS);
	bn_init(&p, FP_DIGS);
	bn_init(&r, FP_DIGS);

	/* u = a, vv = p, g1 = 1, g2 = 0. */
#if FP_RDC == MONTY
	fp_prime_back(&u, a);
#else
	u.used = FP_DIGS;
	dv_copy(u.dp, a, FP_DIGS);
#endif
	p.used = FP_DIGS;
	dv_copy(p.dp, fp_prime_get(), FP_DIGS);
	bn_copy(&v, &p);
	bn_set_dig(&g1, 1);
	bn_zero(&g2);

	/* While (u != 1. */
	while (bn_cmp_dig(&u, 1) != CMP_EQ) {
		/* q = [v/u], r = v mod u. */
		bn_div_rem(&q, &r, &v, &u);
		/* v = u, u = r. */
		bn_copy(&v, &u);
		bn_copy(&u, &r);
		/* r = g2 - q * g1. */
		bn_mul(&r, &q, &g1);
		bn_sub(&r, &g2, &r);
		/* g2 = g1, g1 = r. */
		bn_copy(&g2, &g1);
		bn_copy(&g1, &r);
	}

#if FP_RDC == MONTY
	fp_prime_conv(c, &g1);
#else
	if (bn_sign(&g1) == BN_NEG) {
		bn_add(&g1, &g1, &p);
	}
	dv_copy(c, g1.dp, FP_DIGS);
#endif
}
