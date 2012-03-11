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
 * Implementation of divisor class octupling on binary hyperelliptic curves.
 *
 * @version $Id$
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
 * Octuples a divisor class represented in affine coordinates on a supersingular
 * binary hyperelliptic curve.
 *
 * @param r					- the result.
 * @param p					- the divisor class to octuple.
 */
static void hb_oct_basic_super(hb_t r, hb_t p) {
	fb_t z0, z1, z2, z3;

	fb_null(z0);
	fb_null(z1);
	fb_null(z2);
	fb_null(z3);

	TRY {
		fb_new(z0);
		fb_new(z1);
		fb_new(z2);
		fb_new(z3);

		/* Let p = [[u11, u10], [v11, v10]]]. */

		if (!p->deg) {
			/* z1 = u1^64, z0 = u0^64, z3 = v1^64, z2 = v0^64. */
			fb_sqr(z1, p->u1);
			fb_sqr(z0, p->u0);
			fb_sqr(z3, p->v1);
			fb_sqr(z2, p->v0);
			for (int i = 0; i < 5; i++) {
				fb_sqr(z1, z1);
				fb_sqr(z0, z0);
				fb_sqr(z2, z2);
				fb_sqr(z3, z3);
			}
			/* u30 = (u1 + u0 + 1)^64. */
			fb_add(r->u0, z0, z1);
			fb_add_dig(r->u0, r->u0, 1);
			/* u31 = u1^64. */
			fb_copy(r->u1, z1);
			/* v30 = (u1 + u0 + v1 + v0 + 1)^64. */
			fb_add(r->v0, r->u0, z2);
			fb_add(r->v0, r->v0, z3);
			/* v31 = (v1 + u1)^64. */
			fb_add(r->v1, z1, z3);

			r->deg = 0;
		} else {
			/* z0 = u0^2 + 1. */
			fb_sqr(r->u0, p->u0);
			fb_add_dig(r->u0, r->u0, 1);
			/* v30 = (v10 + u0^2 + 1)^64, u30 = (u0 + 1)^64. */
			fb_add(r->v0, r->u0, p->v0);
			for (int i = 0; i < 5; i++) {
				fb_sqr(r->v0, r->v0);
				fb_sqr(r->u0, r->u0);
			}
			fb_sqr(r->v0, r->v0);
			/* u31 = 1, v31 = 0. */
			fb_zero(r->v1);
			fb_set_dig(r->u1, 1);

			r->deg = 1;
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
	}
}

#endif /* EB_SUPER */

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void hb_oct_basic(hb_t r, hb_t p) {
	if (hb_is_infty(p)) {
		hb_set_infty(r);
		return;
	}
#if defined(HB_SUPER)
	if (hb_curve_is_super()) {
		hb_oct_basic_super(r, p);
		return;
	}
#endif
}
