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
 * Implementation of prime field exponentiation functions.
 *
 * @version $Id$
 * @ingroup bn
 */

#include <string.h>

#include "relic_conf.h"
#include "relic_core.h"
#include "relic_fb.h"
#include "relic_bn.h"
#include "relic_util.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if FB_EXP == BASIC || !defined(STRIP)

void fb_exp_basic(fb_t c, fb_t a, bn_t b) {
	int i, l;
	fb_t r;

	fb_null(r);

	TRY {
		fb_new(r);

		l = bn_bits(b);

		fb_copy(r, a);

		for (i = l - 2; i >= 0; i--) {
			fb_sqr(r, r);
			if (bn_test_bit(b, i)) {
				fb_mul(r, r, a);
			}
		}

		fb_copy(c, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(r);
	}
}

#endif

#if FB_EXP == SLIDE || !defined(STRIP)

void fb_exp_slide(fb_t c, fb_t a, bn_t b) {
	fb_t tab[1 << (FB_WIDTH - 1)], t;
	int i, j, l;
	unsigned char win[FB_BITS];

	fb_null(t);

	/* Initialize table. */
	for (i = 0; i < (1 << (FB_WIDTH - 1)); i++) {
		fb_null(tab[i]);
	}

	TRY {
		for (i = 0; i < (1 << (FB_WIDTH - 1)); i++) {
			fb_new(tab[i]);
		}
		fb_new(t);

		fb_copy(tab[0], a);
		fb_sqr(t, a);

		/* Create table. */
		for (i = 1; i < 1 << (FB_WIDTH - 1); i++) {
			fb_mul(tab[i], tab[i - 1], t);
		}

		fb_set_dig(t, 1);
		bn_rec_slw(win, &l, b, FB_WIDTH);
		for (i = 0; i < l; i++) {
			if (win[i] == 0) {
				fb_sqr(t, t);
			} else {
				for (j = 0; j < util_bits_dig(win[i]); j++) {
					fb_sqr(t, t);
				}
				fb_mul(t, t, tab[win[i] >> 1]);
			}
		}

		fb_copy(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		for (i = 0; i < (1 << (FB_WIDTH - 1)); i++) {
			fb_free(tab[i]);
		}
		fb_free(t);
	}
}

#endif

#if FB_EXP == MONTY || !defined(STRIP)

void fb_exp_monty(fb_t c, fb_t a, bn_t b) {
	fb_t tab0, tab1;
	dig_t *tab[2];
	dig_t buf;
	int bitcnt, digidx, j;

	fb_null(tab[0]);
	fb_null(tab[1]);

	TRY {
		fb_new(tab0);
		fb_new(tab1);

		fb_set_dig(tab0, 1);
		fb_copy(tab1, a);

		/* This trick avoid buggy compilers with alignment issues. */
		tab[0] = tab0;
		tab[1] = tab1;

		/* Set initial mode and bitcnt, */
		bitcnt = 1;
		buf = 0;
		digidx = b->used - 1;

		for (;;) {
			/* Grab next digit as required. */
			if (--bitcnt == 0) {
				/* If digidx == -1 we are out of digits so break. */
				if (digidx == -1) {
					break;
				}
				/* Read next digit and reset bitcnt. */
				buf = b->dp[digidx--];
				bitcnt = (int)FB_DIGIT;
			}

			/* Grab the next msb from the exponent. */
			j = (buf >> (FB_DIGIT - 1)) & 0x01;
			buf <<= (dig_t)1;

			fb_mul(tab[j ^ 1], tab[0], tab[1]);
			fb_sqr(tab[j], tab[j]);
		}

		fb_copy(c, tab[0]);

	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(tab[1]);
		fb_free(tab[0]);
	}
}

#endif
