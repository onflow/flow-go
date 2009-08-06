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

#if FB_RDC == BASIC || !defined(STRIP)

/**
 * Type of a shifted polynomial (used to maintain aligment).
 */
typedef align dig_t shift_t[FB_DIGS + 1];

/**
 * Precomputed table of the modulus shifted by different amounts.
 */
static shift_t poly_shift[FB_DIGIT];

#else

/**
 * Emulate the precomputation table with a single digit vector.
 */
static dig_t poly_shift[FB_DIGIT][FB_DIGS + 1];

#endif

/**
 * Trinomial or pentanomial non-zero coefficients.
 */
int poly_a, poly_b, poly_c;

/**
 * Positions of the non-null coefficients on trinomials and pentanomials.
 */
int pos_a, pos_b, pos_c;

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fb_poly_init(void) {
	fb_zero(poly);
#if FB_RDC == BASIC || !defined(STRIP)
	for (int i = 0; i < FB_DIGIT; i++) {
		dv_zero(poly_shift[i], 2 * FB_DIGS);
	}
#endif
}

void fb_poly_clean(void) {
}

dig_t *fb_poly_get(void) {
	return poly;
}

dig_t *fb_poly_get_rdc(int shift) {
#if FB_RDC == BASIC || !defined(STRIP)
	return poly_shift[shift];
#else
	fb_lsh(poly_shift, poly, shift);
	return poly_shift;
#endif
}

void fb_poly_set(fb_t f) {
	fb_copy(poly, f);
#if FB_RDC == BASIC || !defined(STRIP)
	dig_t carry;

	for (int i = 0; i < FB_DIGIT; i++) {
		carry = fb_lshb_low(poly_shift[i], poly, i);
		poly_shift[i][FB_DIGS] = carry;
	}
#else
	dv_zero(poly_shift, FB_DIGS + 1);
#endif

	poly_a = poly_b = poly_c = 0;
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
	fb_t f = NULL;

	TRY {
		fb_new(f);
		fb_zero(f);
		fb_set_bit(f, FB_BITS, 1);
		fb_set_bit(f, a, 1);
		fb_set_bit(f, 0, 1);
		fb_poly_set(f);
		fb_free(f);

		poly_a = a;
		poly_b = 0;
		poly_c = 0;

		pos_a = poly_a >> FB_DIG_LOG;
		pos_b = poly_b >> FB_DIG_LOG;
		pos_c = poly_c >> FB_DIG_LOG;

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
	fb_t f = NULL;

	TRY {
		fb_new(f);

		fb_zero(f);
		fb_set_bit(f, FB_BITS, 1);
		fb_set_bit(f, a, 1);
		fb_set_bit(f, b, 1);
		fb_set_bit(f, c, 1);
		fb_set_bit(f, 0, 1);
		fb_poly_set(f);
		fb_free(f);

		poly_a = a;
		poly_b = b;
		poly_c = c;

		pos_a = poly_a >> FB_DIG_LOG;
		pos_b = poly_b >> FB_DIG_LOG;
		pos_c = poly_c >> FB_DIG_LOG;

#if FB_QRT == QUICK
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

void fb_poly_get_quick(int *a, int *b, int *c) {
	*a = poly_a;
	*b = poly_b;
	*c = poly_c;
}
