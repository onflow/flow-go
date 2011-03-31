/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2011 RELIC Authors
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
 * Implementation of the binary elliptic curve utilities.
 *
 * @version $Id$
 * @ingroup eb
 */

#include "relic_core.h"
#include "relic_eb.h"
#include "relic_error.h"
#include "relic_conf.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

int eb_is_infty(eb_t p) {
	return (fb_is_zero(p->z) == 1);
}

void eb_set_infty(eb_t p) {
	fb_zero(p->x);
	fb_zero(p->y);
	fb_zero(p->z);
	p->norm = 1;
}

void eb_copy(eb_t r, eb_t p) {
	fb_copy(r->x, p->x);
	fb_copy(r->y, p->y);
	fb_copy(r->z, p->z);
	r->norm = p->norm;
}

int eb_cmp(eb_t p, eb_t q) {
	if (fb_cmp(p->x, q->x) != CMP_EQ) {
		return CMP_NE;
	}

	if (fb_cmp(p->y, q->y) != CMP_EQ) {
		return CMP_NE;
	}

	if (fb_cmp(p->z, q->z) != CMP_EQ) {
		return CMP_NE;
	}

	return CMP_EQ;
}

void eb_rand(eb_t p) {
	bn_t n, k;

	bn_null(n);
	bn_null(k);

	TRY {
		bn_new(k);
		bn_new(n);

		eb_curve_get_ord(n);

		bn_rand(k, BN_POS, bn_bits(n));
		bn_mod(k, k, n);

		eb_mul_gen(p, k);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		bn_free(k);
		bn_free(n);
	}
}

void eb_rhs(fb_t rhs, eb_t p) {
	fb_t t0, t1;

	fb_null(t0);
	fb_null(t1);
	fb_null(t0);

	TRY {
		fb_new(t0);
		fb_new(t1);
		fb_new(t0);

		/* t0 = x1^2. */
		fb_sqr(t0, p->x);
		/* t1 = x1^3. */
		fb_mul(t1, t0, p->x);

		if (eb_curve_is_super()) {
			/* t1 = x1^3 + a * x1 + b. */
			switch (eb_curve_opt_a()) {
				case OPT_ZERO:
					break;
				case OPT_ONE:
					fb_add(t1, t1, p->x);
					break;
				case OPT_DIGIT:
					fb_mul_dig(t0, p->x, eb_curve_get_a()[0]);
					fb_add(t1, t1, t0);
					break;
				default:
					fb_mul(t0, p->x, eb_curve_get_a());
					fb_add(t1, t1, t0);
					break;
			}
		} else {
			/* t1 = x1^3 + a * x1^2 + b. */
			switch (eb_curve_opt_a()) {
				case OPT_ZERO:
					break;
				case OPT_ONE:
					fb_add(t1, t1, t0);
					break;
				case OPT_DIGIT:
					fb_mul_dig(t0, t0, eb_curve_get_a()[0]);
					fb_add(t1, t1, t0);
					break;
				default:
					fb_mul(t0, t0, eb_curve_get_a());
					fb_add(t1, t1, t0);
					break;
			}
		}

		switch (eb_curve_opt_b()) {
			case OPT_ZERO:
				break;
			case OPT_ONE:
				fb_add_dig(t1, t1, 1);
				break;
			case OPT_DIGIT:
				fb_add_dig(t1, t1, eb_curve_get_b()[0]);
				break;
			default:
				fb_add(t1, t1, eb_curve_get_b());
				break;
		}

		fb_copy(rhs, t1);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(t0);
		fb_free(t1);
		fb_free(t0);
	}
}

int eb_is_valid(eb_t p) {
	eb_t t;
	fb_t lhs;
	int r = 0;

	eb_null(t);
	fb_null(lhs);

	TRY {
		eb_new(t);
		fb_new(lhs);

		eb_norm(t, p);

		if (eb_curve_is_super()) {
			fb_mul(lhs, t->y, eb_curve_get_c());
		} else {
			fb_mul(lhs, t->x, t->y);
		}
		eb_rhs(t->x, t);
		fb_sqr(t->y, t->y);
		fb_add(lhs, lhs, t->y);
		r = (fb_cmp(lhs, t->x) == CMP_EQ) || eb_is_infty(p);

	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		eb_free(t);
		fb_free(lhs);
	}
	return r;
}

void eb_print(eb_t p) {
	fb_print(p->x);
	fb_print(p->y);
	fb_print(p->z);
}
