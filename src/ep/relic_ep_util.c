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
 * Implementation of the prime elliptic curve utilities.
 *
 * @version $Id$
 * @ingroup ep
 */

#include "relic_core.h"
#include "relic_md.h"
#include "relic_ep.h"
#include "relic_error.h"
#include "relic_conf.h"
#include "relic_fp_low.h"

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
	p->norm = 1;
}

void ep_copy(ep_t r, ep_t p) {
	fp_copy(r->x, p->x);
	fp_copy(r->y, p->y);
	fp_copy(r->z, p->z);
	r->norm = p->norm;
}

int ep_cmp(ep_t p, ep_t q) {
	if (fp_cmp(p->x, q->x) != CMP_EQ) {
		return CMP_NE;
	}

	if (fp_cmp(p->y, q->y) != CMP_EQ) {
		return CMP_NE;
	}

	if (fp_cmp(p->z, q->z) != CMP_EQ) {
		return CMP_NE;
	}

	return CMP_EQ;
}

void ep_rand(ep_t p) {
	bn_t n, k;

	bn_null(k);
	bn_null(n);

	TRY {
		bn_new(k);
		bn_new(n);

		ep_curve_get_ord(n);

		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);

		ep_mul_gen(p, k);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		bn_free(k);
		bn_free(n);
	}
}

void ep_rhs(fp_t rhs, ep_t p) {
	fp_t t0;
	fp_t t1;

	fp_null(t0);
	fp_null(t1);

	TRY {
		fp_new(t0);
		fp_new(t1);

		/* t0 = x1^2. */
		fp_sqr(t0, p->x);
		/* t1 = x1^3. */
		fp_mul(t1, t0, p->x);

		/* t1 = x1^3 + a * x1 + b. */
		switch (ep_curve_opt_a()) {
			case OPT_ZERO:
				break;
			case OPT_ONE:
				fp_add(t1, t1, p->x);
				break;
#if FP_RDC != MONTY
			case OPT_DIGIT:
				fp_mul_dig(t0, p->x, ep_curve_get_a()[0]);
				fp_add(t1, t1, t0);
				break;
#endif
			default:
				fp_mul(t0, p->x, ep_curve_get_a());
				fp_add(t1, t1, t0);
				break;
		}

		switch (ep_curve_opt_b()) {
			case OPT_ZERO:
				break;
			case OPT_ONE:
				fp_add_dig(t1, t1, 1);
				break;
#if FP_RDC != MONTY
			case OPT_DIGIT:
				fp_add_dig(t1, t1, ep_curve_get_b()[0]);
				break;
#endif
			default:
				fp_add(t1, t1, ep_curve_get_b());
				break;
		}

		fp_copy(rhs, t1);

	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp_free(t0);
		fp_free(t1);
	}
}

void ep_tab(ep_t * t, ep_t p, int w) {
	if (w > 2) {
		ep_dbl(t[0], p);
#if defined(EP_MIXED)
		ep_norm(t[0], t[0]);
#endif
		ep_add(t[1], t[0], p);
		for (int i = 2; i < (1 << (w - 2)); i++) {
			ep_add(t[i], t[i - 1], t[0]);
		}
#if defined(EP_MIXED)
		ep_norm_sim(t + 1, t + 1, (1 << (w - 2)) - 1);
#endif
	}
	ep_copy(t[0], p);
}

int ep_is_valid(ep_t p) {
	ep_t t;
	int r = 0;

	ep_null(t);

	TRY {
		ep_new(t);

		ep_norm(t, p);

		ep_rhs(t->x, t);
		fp_sqr(t->y, t->y);
		r = (fp_cmp(t->x, t->y) == CMP_EQ) || ep_is_infty(p);

	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		ep_free(t);
	}
	return r;
}

void ep_print(ep_t p) {
	fp_print(p->x);
	fp_print(p->y);
	fp_print(p->z);
}
