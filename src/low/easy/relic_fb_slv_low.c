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
 * Implementation of the low-level binary field square root.
 *
 * @version $Id$
 * @ingroup fb
 */

#include <stdlib.h>

#include "relic_fb.h"
#include "relic_fb_low.h"
#include "relic_util.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

static void fb_slvt_low(dig_t *c, dig_t *a, int fa) {
	int i, from, d, b;
	fb_t s = NULL, t = NULL;

	fb_new(s);
	fb_new(t);

	fb_zero(s);
	fb_copy(t, a);

	from = FB_BITS - fa;
	from = (from % 2 == 0 ? from - 1 : from - 2);
	for (i = from; i > (FB_BITS - 1) / 2; i -= 2) {
		if (fb_test_bit(t, i)) {
			SPLIT(b, d, 2 * i - FB_BITS + fa, FB_DIG_LOG);
			t[d] ^= ((dig_t)1 << b);
			SPLIT(b, d, 2 * i - FB_BITS, FB_DIG_LOG);
			t[d] ^= ((dig_t)1 << b);
			SPLIT(b, d, i, FB_DIG_LOG);
			s[d] ^= ((dig_t)1 << b);
		}
	}
	for (i = (FB_BITS - 1) / 2; i >= 1; i--) {
		if (fb_test_bit(t, 2 * i)) {
			SPLIT(b, d, i, FB_DIG_LOG);
			t[d] ^= ((dig_t)1 << b);
			s[d] ^= ((dig_t)1 << b);
		}
	}
	for (i = 1; i <= (FB_BITS - 1) / 2; i += 2) {
		if (fb_test_bit(t, i)) {
			fb_add(s, s, fb_poly_get_slv(i));
		}
	}
	from = MAX((FB_BITS - 1) / 2 + 1, FB_BITS - fa);
	from = (from % 2 == 0 ? from + 1 : from);
	for (i = from; i <= FB_BITS - 2; i += 2) {
		if (fb_test_bit(t, i)) {
			fb_add(s, s, fb_poly_get_slv(i));
		}
	}
	fb_copy(c, s);
}

static void fb_slvp_low(dig_t *c, dig_t *a, int fa, int fb, int fc) {
	int i, from, d, b;
	fb_t s = NULL, t = NULL;

	fb_new(s);
	fb_new(t);

	fb_zero(s);
	fb_copy(t, a);

	from = FB_BITS - fa;
	from = (from % 2 == 0 ? from - 1 : from - 2);
	for (i = from; i > (FB_BITS - 1) / 2; i -= 2) {
		if (fb_test_bit(t, i)) {
			SPLIT(b, d, 2 * i - FB_BITS + fa, FB_DIG_LOG);
			t[d] ^= ((dig_t)1 << b);
			SPLIT(b, d, 2 * i - FB_BITS + fb, FB_DIG_LOG);
			t[d] ^= ((dig_t)1 << b);
			SPLIT(b, d, 2 * i - FB_BITS + fc, FB_DIG_LOG);
			t[d] ^= ((dig_t)1 << b);
			SPLIT(b, d, 2 * i - FB_BITS, FB_DIG_LOG);
			t[d] ^= ((dig_t)1 << b);
			SPLIT(b, d, i, FB_DIG_LOG);
			s[d] ^= ((dig_t)1 << b);
		}
	}
	for (i = (FB_BITS - 1) / 2; i >= 1; i--) {
		if (fb_test_bit(t, 2 * i)) {
			SPLIT(b, d, i, FB_DIG_LOG);
			t[d] ^= ((dig_t)1 << b);
			s[d] ^= ((dig_t)1 << b);
		}
	}
	for (i = 1; i <= (FB_BITS - 1) / 2; i += 2) {
		if (fb_test_bit(t, i)) {
			fb_add(s, s, fb_poly_get_slv(i));
		}
	}
	from = MAX((FB_BITS - 1) / 2 + 1, FB_BITS - fa);
	from = (from % 2 == 0 ? from + 1 : from);
	for (i = from; i <= FB_BITS - 2; i += 2) {
		if (fb_test_bit(t, i)) {
			fb_add(s, s, fb_poly_get_slv(i));
		}
	}
	fb_copy(c, s);
}

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fb_slvn_low(dig_t *c, dig_t *a) {
	int fa, fb, fc;

	fb_poly_get_rdc(&fa, &fb, &fc);

	if (fb == -1) {
		fb_slvt_low(c, a, fa);
	} else {
		fb_slvp_low(c, a, fa, fb, fc);
	}
}
