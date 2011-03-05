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
 * Implementation of the binary hyperelliptic curve utilities.
 *
 * @version $Id: relic_hb_util.c 390 2010-06-05 22:15:02Z dfaranha $
 * @ingroup hb
 */

#include "relic_core.h"
#include "relic_hb.h"
#include "relic_rand.h"
#include "relic_error.h"
#include "relic_conf.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void hb_rand(hb_t p) {
	unsigned char b;

	rand_bytes(&b, 1);
	b &= 1;
	if (b & 1) {
		hb_rand_deg(p);
	} else {
		rand_bytes(&b, 1);
		hb_rand_non(p, b & 1);
	}
}

void hb_rand_deg(hb_t p) {
	fb_t x, y, t;

	fb_null(x);
	fb_null(y);
	fb_null(t);

	TRY {
		fb_new(x);
		fb_new(y);
		fb_new(t);

		while (1) {
			fb_rand(x);
			/* t = x^2. */
			fb_sqr(t, x);
			/* y = x^4. */
			fb_sqr(y, t);
			/* t = x^3. */
			fb_mul(t, t, x);
			/* y = x^5. */
			fb_mul(y, y, x);
			/* t = x^5 + x^3. */
			fb_add(t, t, y);
			if (fb_trc(t) != 0) {
				continue;
			} else {
				/* Solve y^2 + y = x^5 + x^3. */
				fb_slv(y, t);
				/* Return divisor [x - x0, y0]. */
				fb_set_dig(p->u1, 1);
				fb_neg(p->u0, x);
				fb_zero(p->v1);
				fb_copy(p->v0, y);
				p->deg = 1;
				break;
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(x);
		fb_free(y);
		fb_free(t);
	}
}

void hb_rand_non(hb_t p, int type) {
	bn_t n, k;
	fb_t t;
	hb_t g;

	bn_null(n);
	bn_null(k);
	hb_null(g);
	fb_null(t);

	TRY {
		while (1) {
			bn_new(k);
			bn_new(n);
			hb_new(g);
			fb_new(t);

			hb_curve_get_ord(n);
			hb_curve_get_gen(g);

			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);

			hb_mul(p, g, k);

			/* Check if x^2 + u1 * x + u0 has roots in the base field. */
			fb_sqr(t, p->u1);
			fb_inv(t, t);
			fb_mul(t, t, p->u0);
			if (fb_trc(t) == type && p->deg == 0) {
				break;
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(k);
		bn_free(n);
		hb_free(g);
		fb_free(t);
	}
}
