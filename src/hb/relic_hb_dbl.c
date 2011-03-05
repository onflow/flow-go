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
 * Implementation of divisor class doubling on binary hyperelliptic curves.
 *
 * @version $Id: relic_hb_add.c 181 2009-11-17 18:33:42Z dfaranha $
 * @ingroup hb
 */

#include "string.h"

#include "relic_core.h"
#include "relic_hb.h"
#include "relic_error.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if defined(HB_SUPER)
/**
 * Doubles a divisor class represented in affine coordinates on a supersingular
 * binary hyperelliptic curve.
 *
 * @param r					- the result.
 * @param p					- the divisor class to double.
 */
static void hb_dbl_basic_super(hb_t r, hb_t p) {
	fb_t z0, z1, z2, z3, z4, s0;

	fb_null(z0);
	fb_null(z1);
	fb_null(z2);
	fb_null(z3);
	fb_null(z4);
	fb_null(s0);

	TRY {
		fb_new(z0);
		fb_new(z1);
		fb_new(z2);
		fb_new(z3);
		fb_new(z4);
		fb_new(s0);

		/* Let p = [[u11, u10], [v11, v10]]]. */

		if (!p->deg) {
			/* z0 = u11^2, z1 = v11^2. */
			fb_sqr(z0, p->u1);
			fb_sqr(z1, p->v1);
			/* w0 = f3 + z0. */
			fb_add(z0, z0, hb_curve_get_f3());
			if (fb_is_zero(z0)) {
				/* s0 = h0^{-1} * (f2 + z1), w1 = u10 * s0 + v10 + h0. */
				fb_copy(s0, z1);
				fb_mul(z1, p->u0, s0);
				fb_add(z1, z1, p->v0);
				fb_add_dig(z1, z1, 1);
				/* u30 = s0^2. */
				fb_sqr(r->u0, s0);
				/* w2 = s0 * (u11 + u30) + v11. */
				fb_add(z2, p->u1, r->u0);
				fb_mul(z2, z2, s0);
				fb_add(z2, z2, p->v1);
				/* v30 = u30 * w2 + w1. */
				fb_mul(r->v0, r->u0, z2);
				fb_add(r->v0, r->v0, z1);
				/* u31 = 0, v31 = 0. */
				fb_set_dig(r->u1, 1);
				fb_zero(r->v1);

				r->deg = 1;
			} else {
				/* z4 = w1 = 1/w0. */
				fb_inv(z4, z0);
				/* s0 = (f2 + z1) * w1 + u11, z2 = u30 = s0^2. */
				fb_mul(s0, z1, z4);
				fb_add(s0, s0, p->u1);
				fb_sqr(z2, s0);
				/* w2 = h0^2 * w1 = w1, z3 = u31 = w2 * w1. */
				fb_sqr(z3, z4);
				/* v31 = h0^{-1} * (f1 + u10^2 + u30 * w0 + w2 * (u31 + u11 + s0)). */
				fb_add(r->v1, z3, p->u1);
				fb_add(r->v1, r->v1, s0);
				fb_mul(r->v1, r->v1, z4);
				fb_add(r->v1, r->v1, hb_curve_get_f1());
				fb_mul(z0, z0, z2);
				fb_add(r->v1, r->v1, z0);
				fb_sqr(z0, p->u0);
				fb_add(r->v1, r->v1, z0);
				/* v30 = h0^{-1} * (f0 + v0^2 + u30 * (f2 + z1 + w2)) + h0. */
				fb_add(z4, z4, z1);
				fb_mul(z4, z4, z2);
				fb_sqr(r->v0, p->v0);
				fb_add(r->v0, r->v0, z4);
				fb_add(r->v0, r->v0, hb_curve_get_f0());
				fb_add_dig(r->v0, r->v0, 1);
				/* u31 = z3, u30 = z2. */
				fb_copy(r->u1, z3);
				fb_copy(r->u0, z2);

				r->deg = 0;
			}
		} else {
			fb_sqr(r->u0, p->u0);
			fb_zero(r->u1);
			fb_sqr(r->v1, r->u0);
			fb_add(r->v1, r->v1, r->u0);
			fb_sqr(r->v0, p->v0);
			fb_add(r->v0, r->v0, hb_curve_get_f0());

			r->deg = 0;
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(z0);
		fb_free(z1);
		fb_free(z2);
		fb_free(z3);
		fb_free(z4);
		fb_free(s0);
	}
}

#endif /* EB_SUPER */

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void hb_dbl_basic(hb_t r, hb_t p) {
	if (hb_is_infty(p)) {
		hb_set_infty(r);
		return;
	}
#if defined(HB_SUPER)
	if (hb_curve_is_super()) {
		hb_dbl_basic_super(r, p);
		return;
	}
#endif
}
