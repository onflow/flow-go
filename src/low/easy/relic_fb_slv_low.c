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

static const dig_t table_evens[16] = {
	0, 1, 4, 5, 2, 3, 6, 7, 8, 9, 12, 13, 10, 11, 14, 15
};

static const dig_t table_odds[16] = {
	0, 4, 1, 5, 8, 12, 9, 13, 2, 6, 3, 7, 10, 14, 11, 15
};

static void fb_slvt_low(dig_t *c, dig_t *a, int fa) {
	int i, j, k, from, to, b, d, v[FB_BITS];
	dig_t u, u_e, *p;
	align dig_t s[FB_DIGS], t[FB_DIGS];
	dig_t mask;
	void *tab = fb_poly_get_slv();

	fb_zero(s);
	fb_copy(t, a);

	from = FB_BITS - fa;
	from = (from % 2 == 0 ? from - 1 : from - 2);
	to = (FB_BITS - 1) / 2;

	for (i = from; i > to; i -= 2) {
		if (fb_test_bit(t, i)) {
			SPLIT(b, d, 2 * i - FB_BITS + fa, FB_DIG_LOG);
			t[d] ^= ((dig_t)1 << b);
			SPLIT(b, d, 2 * i - FB_BITS, FB_DIG_LOG);
			t[d] ^= ((dig_t)1 << b);
			SPLIT(b, d, i, FB_DIG_LOG);
			s[d] ^= ((dig_t)1 << b);
		}
	}

	for (i = FB_DIGS - 1; i > 0; i--) {
		u = t[i];
		u_e = table_evens[((u & 0x05) + ((u & 0x50) >> 3))];
		for (j = 1; j < FB_DIGIT / 8; j++) {
			u >>= 8;
			u_e |= table_evens[((u & 0x05) + ((u & 0x50) >> 3))] << (j << 2);
		}
		u_e = u_e << (i & 1) * FB_DIGIT / 2;
		t[i >> 1] ^= u_e;
		s[i >> 1] ^= u_e;
	}

	for (i = FB_DIGIT / 2; i > 1; i = i >> 1) {
		u = (t[0] >> i) & MASK(i);
		u_e = table_evens[((u & 0x05) + ((u & 0x50) >> 3))];
		for (j = 1; j < i / 8; j++) {
			u >>= 8;
			u_e |= table_evens[((u & 0x05) + ((u & 0x50) >> 3))] << (j << 2);
		}
		u_e = u_e << (i >> 1);
		t[0] ^= u_e;
		s[0] ^= u_e;
	}

	k = 0;
	/* We need to + 1 to get the (FB_BITS - 1)/2-th even bit. */
	SPLIT(b, d, to + 1, FB_DIG_LOG);
	for (i = 0; i < d; i++) {
		u = t[i];
		for (j = 0; j < FB_DIGIT / 8; j++) {
			v[k++] = table_odds[((u & 0x0A) + ((u & 0xA0) >> 5))];
			u >>= 8;
		}
	}
	mask = (b == FB_DIGIT ? DMASK : MASK(b));
	u = t[d] & mask;
	/* We ignore the first even bit if it is present. */
	for (j = 1; j < b; j += 8) {
		v[k++] = table_odds[((u & 0x0A) + ((u & 0xA0) >> 5))];
		u >>= 8;
	}

	from = MAX(to + 1, FB_BITS - fa);
	from = (from % 2 == 0 ? from : from - 1);

	fb_rsh(t, t, from);
	SPLIT(b, d, FB_BITS - from, FB_DIG_LOG);
	b++;
	for (i = 0; i < d; i++) {
		u = t[i];
		for (j = 0; j < FB_DIGIT / 8; j++) {
			v[k++] = table_odds[((u & 0x0A) + ((u & 0xA0) >> 5))];
			u >>= 8;
		}
	}
	mask = (b == FB_DIGIT ? DMASK : MASK(b));
	u = t[d] & mask;
	for (j = 0; j < b; j += 8) {
		v[k++] = table_odds[((u & 0x0A) + ((u & 0xA0) >> 5))];
		u >>= 8;
	}

	for (i = 0; i < k; i++) {
		p = (dig_t *)(tab + (16 * i + v[i]) * sizeof(fb_st));
		fb_add(s, s, p);
	}

	fb_copy(c, s);
}

static void fb_slvp_low(dig_t *c, dig_t *a, int fa, int fb, int fc) {
	int i, j, k, from, to, b, d, v[FB_BITS];
	fb_t s, t;
	dig_t u, u_e, *p;
	dig_t mask;
	void *tab = fb_poly_get_slv();

	fb_null(s);
	fb_null(t);
	fb_new(s);
	fb_new(t);

	fb_zero(s);
	fb_copy(t, a);

	from = FB_BITS - fa;
	from = (from % 2 == 0 ? from - 1 : from - 2);
	to = (FB_BITS - 1) / 2;

	for (i = from; i > to; i -= 2) {
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

	for (i = FB_DIGS - 1; i > 0; i--) {
		u = t[i];
		u_e = table_evens[((u & 0x05) + ((u & 0x50) >> 3))];
		for (j = 1; j < FB_DIGIT / 8; j++) {
			u >>= 8;
			u_e |= table_evens[((u & 0x05) + ((u & 0x50) >> 3))] << (j << 2);
		}
		u_e = u_e << (i & 1) * FB_DIGIT / 2;
		t[i >> 1] ^= u_e;
		s[i >> 1] ^= u_e;
	}
	for (i = FB_DIGIT / 2; i > 1; i = i >> 1) {
		u = (t[0] >> i) & MASK(i);
		u_e = table_evens[((u & 0x05) + ((u & 0x50) >> 3))];
		for (j = 1; j < i / 8; j++) {
			u >>= 8;
			u_e |= table_evens[((u & 0x05) + ((u & 0x50) >> 3))] << (j << 2);
		}
		u_e = u_e << (i >> 1);
		t[0] ^= u_e;
		s[0] ^= u_e;
	}

	k = 0;
	/* We need to + 1 to get the (FB_BITS - 1)/2-th even bit. */
	SPLIT(b, d, to + 1, FB_DIG_LOG);
	for (i = 0; i < d; i++) {
		u = t[i];
		for (j = 0; j < FB_DIGIT / 8; j++) {
			v[k++] = table_odds[((u & 0x0A) + ((u & 0xA0) >> 5))];
			u >>= 8;
		}
	}
	mask = (b == FB_DIGIT ? DMASK : MASK(b));
	u = t[d] & mask;
	/* We ignore the first even bit if it is present. */
	for (j = 1; j < b; j += 8) {
		v[k++] = table_odds[((u & 0x0A) + ((u & 0xA0) >> 5))];
		u >>= 8;
	}

	from = MAX(to + 1, FB_BITS - fa);
	from = (from % 2 == 0 ? from : from - 1);

	fb_rsh(t, t, from);
	SPLIT(b, d, FB_BITS - from, FB_DIG_LOG);
	b++;
	for (i = 0; i < d; i++) {
		u = t[i];
		for (j = 0; j < FB_DIGIT / 8; j++) {
			v[k++] = table_odds[((u & 0x0A) + ((u & 0xA0) >> 5))];
			u >>= 8;
		}
	}
	mask = (b == FB_DIGIT ? DMASK : MASK(b));
	u = t[d] & mask;
	for (j = 0; j < b; j += 8) {
		v[k++] = table_odds[((u & 0x0A) + ((u & 0xA0) >> 5))];
		u >>= 8;
	}

	for (i = 0; i < k; i++) {
		p = (dig_t *)(tab + (16 * i + v[i]) * sizeof(fb_st));
		fb_add(s, s, p);
	}

	fb_copy(c, s);
}

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fb_slvn_low(dig_t *c, dig_t *a) {
	int fa, fb, fc;

	fb_poly_get_rdc(&fa, &fb, &fc);

	if (fb == 0) {
		fb_slvt_low(c, a, fa);
	} else {
		fb_slvp_low(c, a, fa, fb, fc);
	}
}
