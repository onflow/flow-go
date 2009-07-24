/*
 * Copyright 2007 Project RELIC
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file.
 *
 * RELIC is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @file
 *
 * Implementation of binary field inversion functions.
 *
 * @version $Id$
 * @ingroup fb
 */

#include "relic_core.h"
#include "relic_fb.h"
#include "relic_fb_low.h"
#include "relic_bn_low.h"
#include "relic_util.h"
#include "relic_rand.h"
#include "relic_error.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if FB_INV == BASIC || !defined(STRIP)

void fb_inv_basic(fb_t c, fb_t a) {
	int lu, lv;
	fb_t u = NULL, v = NULL, g1 = NULL, g2 = NULL;

	TRY {
		fb_new(u);
		fb_new(v);
		fb_new(g1);
		fb_new(g2);

		fb_copy(u, a);
		fb_copy(v, fb_poly_get());
		fb_zero(g1);
		g1[0] = 1;
		fb_zero(g2);

		lu = FB_DIGS;
		lv = FB_DIGS;

		while (1) {
			while ((u[0] & 0x01) == 0) {
				bn_rsh1_low(u, u, lu);
				if ((g1[0] & 0x01) == 1) {
					fb_poly_add(g1, g1);
				}
				fb_rsh1_low(g1, g1);
			}

			while (u[lu - 1] == 0)
				lu--;
			if (lu == 1 && u[0] == 1)
				break;

			while ((v[0] & 0x01) == 0) {
				bn_rsh1_low(v, v, lv);
				if ((g2[0] & 0x01) == 1) {
					fb_poly_add(g2, g2);
				}
				fb_rsh1_low(g2, g2);
			}

			while (v[lv - 1] == 0)
				lv--;
			if (lv == 1 && v[0] == 1)
				break;

			if (lu > lv || (lu == lv && u[lu - 1] > v[lv - 1])) {
				fb_addd_low(u, u, v, lv);
				fb_add(g1, g1, g2);
			} else {
				fb_addd_low(v, v, u, lu);
				fb_add(g2, g2, g1);
			}
		}

		if (lu == 1 && u[0] == 1) {
			fb_copy(c, g1);
		} else {
			fb_copy(c, g2);
		}

	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(u);
		fb_free(v);
		fb_free(g1);
		fb_free(g2);
	}
}

#endif

#if FB_INV == EXGCD || !defined(STRIP)

void fb_inv_exgcd(fb_t c, fb_t a) {
	int j, d, lu, lv, lt, l1, l2, bu, bv;
	dv_t _u = NULL, _v = NULL, _g1 = NULL, _g2 = NULL;
	dig_t *t = NULL, *u = NULL, *v = NULL, *g1 = NULL, *g2 = NULL, carry;

	TRY {

		dv_new(_u);
		dv_new(_v);
		dv_new(_g1);
		dv_new(_g2);
		dv_zero(_g1, FB_DIGS + 1);
		dv_zero(_g2, FB_DIGS + 1);

		u = _u;
		v = _v;
		g1 = _g1;
		g2 = _g2;

		fb_copy(u, a);
		fb_copy(v, fb_poly_get());
		g1[0] = 1;

		lu = lv = FB_DIGS;
		l1 = l2 = 1;

		bu = fb_bits(u);
		bv = FB_BITS + 1;
		j = bu - bv;

		while (1) {
			if (j < 0) {
				t = u;
				u = v;
				v = t;

				lt = lu;
				lu = lv;
				lv = lt;

				t = g1;
				g1 = g2;
				g2 = t;

				lt = l1;
				l1 = l2;
				l2 = lt;

				j = -j;
			}

			SPLIT(j, d, j, FB_DIG_LOG);

			if (j > 0) {
				carry = fb_lshadd_low(u + d, v, j, lv);
				if (carry) {
					u[d + lv] ^= carry;
				}
			} else {
				fb_addd_low(u + d, u + d, v, lv);
			}

			if (j > 0) {
				carry = fb_lshadd_low(g1 + d, g2, j, l2);
				l1 = (l2 + d > l1 ? l2 + d : l1);
				if (carry) {
					g1[d + l2] ^= carry;
					l1 = (l2 + d >= l1 ? l1 + 1 : l1);
				}
			} else {
				fb_addd_low(g1 + d, g1 + d, g2, l2);
				l1 = (l2 + d > l1 ? l2 + d : l1);
			}

			while (u[lu - 1] == 0)
				lu--;
			while (v[lv - 1] == 0)
				lv--;

			if (lu == 1 && u[lu - 1] == 1)
				break;

			j = ((lu - lv) << FB_DIG_LOG) + (fb_bits_dig(u[lu - 1]) -
					fb_bits_dig(v[lv - 1]));
		}
		fb_copy(c, g1);

	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(_u);
		dv_free(_v);
		dv_free(_g1);
		dv_free(_g2);
	}
}

#endif

#if FB_INV == ALMOS || !defined(STRIP)

void fb_inv_almos(fb_t c, fb_t a) {
	int lu, lv, lt;
	fb_t _b = NULL, _d = NULL, _u = NULL, _v = NULL;
	dig_t *t = NULL, *u = NULL, *v = NULL, *b = NULL, *d = NULL;

	TRY {
		fb_new(_b);
		fb_new(_d);
		fb_new(_u);
		fb_new(_v);

		b = _b;
		d = _d;
		u = _u;
		v = _v;

		fb_zero(b);
		fb_zero(d);
		b[0] = 1;
		fb_copy(u, a);
		fb_copy(v, fb_poly_get());

		lu = FB_DIGS;
		lv = FB_DIGS;
		while (1) {
			while ((u[0] & 0x01) == 0) {
				bn_rsh1_low(u, u, lu);
				if ((b[0] & 0x01) == 1) {
					fb_poly_add(b, b);
				}
				/* b often has FB_DIGS digits. */
				fb_rsh1_low(b, b);
			}
			while (u[lu - 1] == 0)
				lu--;
			if (lu == 1 && u[0] == 1) {
				break;
			}
			if ((lu < lv) || ((lu == lv) && (u[lu - 1] < v[lv - 1]))) {
				t = u;
				u = v;
				v = t;

				/* Swap lu and lv too. */
				lt = lu;
				lu = lv;
				lv = lt;

				t = b;
				b = d;
				d = t;
			}
			fb_addd_low(u, u, v, lu);
			fb_addn_low(b, b, d);
		}

	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_copy(c, b);
		fb_free(_b);
		fb_free(_d);
		fb_free(_u);
		fb_free(_v);
	}
}

#endif
