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
 * Implementation of hashing to a binary elliptic curve.
 *
 * @version $Id$
 * @ingroup eb
 */

#include "string.h"

#include "relic_core.h"
#include "relic_md.h"
#include "relic_eb.h"
#include "relic_error.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void eb_map(eb_t p, unsigned char *msg, int len) {
	bn_t k;
	fb_t t0, t1, t2;
	int i;
	unsigned char digest[MD_LEN];

	bn_null(k);
	fb_null(t0);
	fb_null(t1);
	fb_null(t2);

	TRY {
		bn_new(k);
		fb_new(t0);
		fb_new(t1);
		fb_new(t2);

		md_map(digest, msg, len);
		bn_read_bin(k, digest, MD_LEN, BN_POS);
		fb_set_dig(p->z, 1);

		i = 0;
		while (1) {
			bn_add_dig(k, k, 1);
			bn_mod_2b(k, k, FB_BITS);
			dv_copy(p->x, k->dp, FB_DIGS);
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
						fb_mul_dig(t2, p->x, eb_curve_get_a()[0]);
						fb_add(t1, t1, t2);
						break;
					default:
						fb_mul(t2, p->x, eb_curve_get_a());
						fb_add(t1, t1, t2);
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
						fb_mul_dig(t2, t0, eb_curve_get_a()[0]);
						fb_add(t1, t1, t2);
						break;
					default:
						fb_mul(t2, t0, eb_curve_get_a());
						fb_add(t1, t1, t2);
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

			if (eb_curve_is_super()) {
				if (eb_curve_opt_c() != OPT_ONE) {
					/* t0 = c^2. */
					fb_sqr(t0, eb_curve_get_c());
					/* t0 = 1/c^2. */
					fb_inv(t0, t0);
					/* t0 = t1/c^2. */
					fb_mul(t1, t0, t1);
				}
				/* Solve t1^2 + t1 = t0. */
				fb_trc(t2, t1);
				if (t2[0] != 0) {
					i++;
				} else {
					fb_slv(t1, t1);
					/* x3 = x1, y3 = t1 * c, z3 = 1. */
					fb_mul(p->y, t1, eb_curve_get_c());
					fb_set_dig(p->z, 1);

					p->norm = 1;
					break;
				}
			} else {
				/* t0 = 1/x1^2. */
				fb_inv(t0, t0);
				/* t0 = t1/x1^2. */
				fb_mul(t0, t0, t1);
				/* Solve t1^2 + t1 = t0. */
				fb_trc(t2, t0);
				if (t2[0] != 0) {
					i++;
				} else {
					fb_slv(t1, t0);
					/* x3 = x1, y3 = t1 * x1, z3 = 1. */
					fb_mul(p->y, t1, p->x);
					fb_set_dig(p->z, 1);

					p->norm = 1;
					break;
				}
			}
		}
		/* Now, multiply by cofactor to get the correct group. */
		eb_curve_get_cof(k);
		if (bn_bits(k) < BN_DIGIT) {
			eb_mul_dig(p, p, k->dp[0]);
		} else {
			eb_mul(p, p, k);
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(k);
		fb_free(t0);
		fb_free(t1);
		fb_free(t2);
	}
}
