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
 * Implementation of the binary field modulus manipulation.
 *
 * @version $Id$
 * @ingroup fb
 */

#include <stdlib.h>
#include <stdio.h>
#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_dv.h"
#include "relic_fb.h"
#include "relic_fb_low.h"
#include "relic_error.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Prime modulus.
 */
static fb_st poly;

/**
 * Trinomial or pentanomial non-zero coefficients.
 */
static int poly_a, poly_b, poly_c;

/**
 * Positions of the non-null coefficients on trinomials and pentanomials.
 */
static int pos_a, pos_b, pos_c;

/**
 * Powers of z with non-zero traces.
 */
static int trc_a, trc_b, trc_c;

/**
 * Find non-zero bits for fast trace computation.
 *
 * @throw ERR_NO_MEMORY if there is no available memory.
 * @throw ERR_INVALID if the polynomial is invalid.
 */
static void find_trace() {
	fb_t t0, t1;
	int counter;

	fb_null(t0);
	fb_null(t1);

	TRY {
		fb_new(t0);
		fb_new(t1);

		counter = 0;
		for (int i = 0; i < FB_BITS; i++) {
			fb_zero(t0);
			fb_set_bit(t0, i, 1);
			fb_copy(t1, t0);
			for (int j = 1; j < FB_BITS; j++) {
				fb_sqr(t1, t1);
				fb_add(t0, t0, t1);
			}
			if (!fb_is_zero(t0)) {
				switch (counter) {
					case 0:
						trc_a = i;
						trc_b = trc_c = -1;
						break;
					case 1:
						trc_b = i;
						trc_c = -1;
						break;
					case 2:
						trc_c = i;
						break;
					default:
						THROW(ERR_INVALID);
						break;
				}
				counter++;
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(t0);
		fb_free(t1);
	}
}

#if FB_SLV == QUICK || !defined(STRIP)

/**
 * Size of the precomputed table of half-traces.
 */
#define HALF_SIZE		(FB_BITS / 2 + 1)

/**
 * Table of precomputed half-traces.
 */
fb_st half[HALF_SIZE];

/**
 * Precomputes half-traces for z^i with odd i.
 *
 * @throw ERR_NO_MEMORY if there is no available memory.
 */
static void find_solve() {
	int i, j;
	fb_t t0;

	fb_null(t0);

	TRY {
		fb_new(t0);

		for (i = FB_BITS - 2; i >= 1; i-=2) {
			fb_zero(t0);
			fb_set_bit(t0, i, 1);
			fb_copy(half[i/2], t0);
			for (j = 0; j < (FB_BITS - 1) / 2; j++) {
				fb_sqr(half[i/2], half[i/2]);
				fb_sqr(half[i/2], half[i/2]);;
				fb_add(half[i/2], half[i/2], t0);
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(t0);
	}
}

#endif

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fb_poly_init(void) {
	fb_zero(poly);
	poly_a = poly_b = poly_c = -1;
	trc_a = trc_b = trc_c = pos_a = pos_b = pos_c = -1;
}

void fb_poly_clean(void) {
}

dig_t *fb_poly_get(void) {
	return poly;
}

void fb_poly_set(fb_t f) {
	fb_copy(poly, f);
	find_trace();
	find_solve();
}

void fb_poly_add(fb_t c, fb_t a) {
	if (c != a) {
		fb_copy(c, a);
	}

	if (poly_a != 0) {
		c[FB_DIGS - 1] ^= poly[FB_DIGS - 1];
		c[pos_a] ^= poly[pos_a];
		if (poly_b != 0 && poly_c != 0) {
			if (pos_b != pos_a) {
				c[pos_b] ^= poly[pos_b];
			}
			if (pos_c != pos_a && pos_c != pos_b) {
				c[pos_c] ^= poly[pos_c];
			}
		}
		c[0] ^= 1;
	} else {
		fb_add(c, a, poly);
	}
}

void fb_poly_set_trino(int a) {
	fb_t f;

	fb_null(f);

	TRY {
		poly_a = a;
		poly_b = poly_c = -1;

		pos_a = poly_a >> FB_DIG_LOG;
		pos_b = pos_c = -1;

		fb_new(f);
		fb_zero(f);
		fb_set_bit(f, FB_BITS, 1);
		fb_set_bit(f, a, 1);
		fb_set_bit(f, 0, 1);
		fb_poly_set(f);

#if FB_SRT == QUICK
		if ((FB_BITS % 2 == 0) || (a % 2 == 0)) {
			THROW(ERR_INVALID);
		}
#endif
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(f);
	}
}

void fb_poly_set_penta(int a, int b, int c) {
	fb_t f;

	fb_null(f);

	TRY {
		fb_new(f);

		poly_a = a;
		poly_b = b;
		poly_c = c;

		pos_a = poly_a >> FB_DIG_LOG;
		pos_b = poly_b >> FB_DIG_LOG;
		pos_c = poly_c >> FB_DIG_LOG;

		fb_zero(f);
		fb_set_bit(f, FB_BITS, 1);
		fb_set_bit(f, a, 1);
		fb_set_bit(f, b, 1);
		fb_set_bit(f, c, 1);
		fb_set_bit(f, 0, 1);
		fb_poly_set(f);
		fb_free(f);

#if FB_SRT == QUICK
		if ((FB_BITS % 2 == 0) || (a % 2 == 0) || (b % 2 == 0) || (c % 2 == 0)) {
			THROW(ERR_INVALID);
		}
#endif
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(f);
	}
}

void fb_poly_get_trc(int *a, int *b, int *c) {
	*a = trc_a;
	*b = trc_b;
	*c = trc_c;
}

void fb_poly_get_rdc(int *a, int *b, int *c) {
	*a = poly_a;
	*b = poly_b;
	*c = poly_c;
}

dig_t *fb_poly_get_slv(int i) {
#if FB_SLV == QUICK || !defined(STRIP)
	return half[i/2];
#else
	return NULL;
#endif
}
