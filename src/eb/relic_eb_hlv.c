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
 * Implementation of point doubling on binary elliptic curves.
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

void eb_hlv(eb_t r, eb_t p) {
	eb_t q;
	fb_t v, l, t;

	eb_null(q);
	fb_null(l);
	fb_null(t);
	fb_null(v);

	TRY {
		eb_new(q);
		fb_new(l);
		fb_new(t);
		fb_new(v);

		if (p->norm == 0) {
			eb_norm(q, p);
		} else {
			eb_copy(q, p);
		}

		/* Solve l^2 + l = u + a. */
		switch (eb_curve_opt_a()) {
			case OPT_ZERO:
				fb_copy(t, q->x);
				break;
			case OPT_ONE:
				fb_add_dig(t, q->x, (dig_t)1);
				break;
			case OPT_DIGIT:
				fb_add_dig(t, q->x, eb_curve_get_a()[0]);
				break;
			default:
				fb_add(t, q->x, eb_curve_get_a());
				break;
		}
		fb_slv(l, t);
		if (q->norm == 1) {
			/* Compute t = v + u * lambda. */
			fb_mul(t, l, q->x);
			fb_add(t, t, q->y);
		} else {
			/* Compute t = u * (u + lambda_P + lambda). */
			fb_add(t, l, q->y);
			fb_add(t, t, q->x);
			fb_mul(t, t, q->x);
		}
		/* If Tr(t) = 0 then lambda_P = lambda, u = sqrt(t + u). */
		fb_trc(v, t);
		if (fb_test_bit(v, 0) == 0) {
			fb_copy(r->y, l);
			fb_add(t, t, q->x);
			fb_srt(r->x, t);
		} else {
			/* Else lambda_P = lambda + 1, u = sqrt(t). */
			fb_add_dig(r->y, l, 1);
			fb_srt(r->x, t);
		}
		fb_zero(r->z);
		fb_set_bit(r->z, 0, 1);
		r->norm = 2;
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		eb_free(q);
		fb_free(l);
		fb_free(t);
		fb_free(v);
	}
}
