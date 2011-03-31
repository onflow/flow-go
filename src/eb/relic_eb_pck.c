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
 * Implementation of point compression on binary elliptic curves.
 *
 * @version $Id$
 * @ingroup eb
 */

#include "string.h"

#include "relic_core.h"
#include "relic_eb.h"
#include "relic_error.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void eb_pck(eb_t r, eb_t p) {
	if (eb_curve_is_super()) {
		/* z3 = y1/c. */
		fb_inv(r->z, eb_curve_get_c());
		fb_mul(r->z, r->z, p->y);
		/* x3 = x1. */
		fb_copy(r->x, p->x);
		/* y3 = b(y1/c). */
		fb_set_dig(r->y, fb_get_bit(r->z, 0));
		/* z3 = 1. */
		fb_set_dig(r->z, 1);
	} else {
		/* z3 = y1/x1. */
		fb_inv(r->z, p->x);
		fb_mul(r->z, r->z, p->y);
		/* x3 = x1. */
		fb_copy(r->x, p->x);
		/* y3 = b(y1/x1). */
		fb_set_dig(r->y, fb_get_bit(r->z, 0));
		/* z3 = 1. */
		fb_set_dig(r->z, 1);
	}

	r->norm = 1;
}

void eb_upk(eb_t r, eb_t p) {
	fb_t t0, t1;

	fb_null(t0);
	fb_null(t1);

	TRY {
		fb_new(t0);
		fb_new(t1);

		eb_rhs(t1, p);

		if (eb_curve_is_super()) {
			/* t0 = c^2. */
			fb_sqr(t0, eb_curve_get_c());
			/* t0 = 1/c^2. */
			fb_inv(t0, t0);
			/* t0 = t1/c^2. */
			fb_mul(t0, t0, t1);
			/* Solve t1^2 + t1 = t0. */
			fb_slv(t1, t0);
			/* If this is not the correct solution, try the other. */
			if (fb_get_bit(t1, 0) != fb_get_bit(p->y, 0)) {
				fb_add_dig(t1, t1, 1);
			}
			/* x3 = x1, y3 = t1 * c, z3 = 1. */
			fb_mul(r->y, t1, eb_curve_get_c());
		} else {
			fb_sqr(t0, p->x);
			/* t0 = 1/x1^2. */
			fb_inv(t0, t0);
			/* t0 = t1/x1^2. */
			fb_mul(t0, t0, t1);
			/* Solve t1^2 + t1 = t0. */
			fb_slv(t1, t0);
			/* If this is not the correct solution, try the other. */
			if (fb_get_bit(t1, 0) != fb_get_bit(p->y, 0)) {
				fb_add_dig(t1, t1, 1);
			}
			/* x3 = x1, y3 = t1 * x1, z3 = 1. */
			fb_mul(r->y, t1, p->x);
		}
		fb_copy(r->x, p->x);
		fb_set_dig(r->z, 1);

		r->norm = 1;
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(t0);
		fb_free(t1);
	}
}
