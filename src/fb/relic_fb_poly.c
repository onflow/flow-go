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
static fb_st fb_poly;

/**
 * Trinomial or pentanomial non-zero coefficients.
 */
static int poly_a, poly_b, poly_c;

/**
 * Positions of the non-null coefficients on trinomials and pentanomials.
 */
static int pos_a, pos_b, pos_c;

#if FB_TRC == QUICK || !defined(STRIP)

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

	trc_a = trc_b = trc_c = -1;

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

#endif

#if FB_SLV == QUICK || !defined(STRIP)

/**
 * Size of the precomputed table of half-traces.
 */
#define HALF_SIZE		((FB_BITS - 1)/ 2)

/**
 * Table of precomputed half-traces.
 */
static fb_st fb_half[(FB_DIGIT / 8) * FB_DIGS][16];

/**
 * Precomputes half-traces for z^i with odd i.
 *
 * @throw ERR_NO_MEMORY if there is no available memory.
 */
static void find_solve() {
	int i, j, k, l;
	fb_t t0;

	fb_null(t0);

	TRY {
		fb_new(t0);

		l = 0;
		for (i = 0; i < FB_BITS; i += 8, l++) {
			for (j = 0; j < 16; j++) {
				fb_zero(t0);
				for (k = 0; k < 4; k++) {
					if (j & (1 << k)) {
						fb_set_bit(t0, i + 2 * k + 1, 1);
					}
				}
				fb_copy(fb_half[l][j], t0);
				for (k = 0; k < (FB_BITS - 1) / 2; k++) {
					fb_sqr(fb_half[l][j], fb_half[l][j]);
					fb_sqr(fb_half[l][j], fb_half[l][j]);
					fb_add(fb_half[l][j], fb_half[l][j], t0);
				}
			}
			fb_rsh(fb_half[l][j], fb_half[l][j], 1);
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

#if FB_SRT == QUICK || !defined(STRIP)

/**
 * Square root of z.
 */
static fb_st fb_srz;

#ifdef FB_PRECO
/**
 * Multiplication table for the z^(1/2).
 */
static fb_st fb_tab_srz[256];

#endif

/**
 * Precomputes the square root of z.
 */
static void find_srz() {

	fb_set_dig(fb_srz, 2);

	for (int i = 1; i < FB_BITS; i++) {
		fb_sqr(fb_srz, fb_srz);
	}

#ifdef FB_PRECO
	for (int i = 0; i <= 255; i++) {
		fb_mul_dig(fb_tab_srz[i], fb_srz, i);
	}
#endif
}

#endif

#if FB_INV == ITOHT || !defined(STRIP)

/**
 * Maximum number of elements in the addition chain for (FB_BITS - 1).
 */
#define MAX_CHAIN		16

/**
 * Stores an addition chain for (FB_BITS - 1).
 */
static int chain[MAX_CHAIN + 1];

/**
 * Stores the length of the addition chain.
 */
static int chain_len;

#ifdef FB_PRECO
/**
 * Tables for repeated squarings.
 */
fb_st fb_tab_sqr[MAX_CHAIN][(FB_DIGIT / 4) * FB_DIGS][16];
#endif

/**
 * Finds an addition chain for (FB_BITS - 1).
 */
static void find_chain() {
	int i, j, k, l;

	chain_len = -1;
	for (int i = 0; i < MAX_CHAIN; i++) {
		chain[i] = (i << 8) + i;
	}
	switch (FB_BITS) {
		case 193:
			chain[1] = (1 << 8) + 0;
			chain_len = 8;
			break;
		case 233:
			chain[1] = (1 << 8) + 0;
			chain[3] = (3 << 8) + 0;
			chain[6] = (6 << 8) + 0;
			chain_len = 10;
			break;
		case 353:
			chain[2] = (2 << 8) + 0;
			chain[4] = (4 << 8) + 0;
			chain_len = 10;
			break;
		default:
			l = 0;
			j = (FB_BITS - 1);
			for (k = 16; k >= 0; k--) {
				if (j & (1 << k)) {
					break;
				}
			}
			for (i = 1; i < k; i++) {
				if (j & (1 << i)) {
					l++;
				}
			}
			i = 0;
			chain_len = k + l;
			while (j != 1) {
				if ((j & 0x01) != 0) {
					i++;
					chain[chain_len - i] = ((chain_len - i) << 8) + 0;
				}
				i++;
				j = j >> 1;
			}
			break;
	}
#ifdef FB_PRECO
	fb_t t;
	int x, y, u[chain_len + 1];

	fb_null(t);

	TRY {
		fb_new(t);

		u[0] = 1;
		u[1] = 2;
		for (i = 2; i <= chain_len; i++) {
			x = chain[i - 1] >> 8;
			y = chain[i - 1] - (x << 8);
			if (x == y) {
				u[i] = 2 * u[i - 1];
			} else {
				u[i] = u[x] + u[y];
			}
		}

		for (i = 0; i <= chain_len; i++) {
			for (j = 0; j < FB_BITS; j += 4) {
				for (k = 0; k < 16; k++) {
					fb_zero(t);
					fb_set_dig(t, k);
					fb_lsh(t, t, j);
					for (l = 0; l < u[i]; l++) {
						fb_sqr(t, t);
					}
					fb_copy(fb_tab_sqr[i][j / 4][k], t);
				}
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(t);
	}
#endif
}

#endif

/**
 * Configures the irreducible polynomial of the binary field.
 *
 * @param[in] f				- the new irreducible polynomial.
 */
static void fb_poly_set(fb_t f) {
	fb_copy(fb_poly, f);
#if FB_TRC == QUICK || !defined(STRIP)
	find_trace();
#endif
#if FB_SLV == QUICK || !defined(STRIP)
	find_solve();
#endif
#if FB_SRT == QUICK || !defined(STRIP)
	find_srz();
#endif
#if FB_INV == ITOHT || !defined(STRIP)
	find_chain();
#endif
}

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fb_poly_init(void) {
	fb_zero(fb_poly);
	poly_a = poly_b = poly_c = 0;
	pos_a = pos_b = pos_c = -1;
}

void fb_poly_clean(void) {
}

dig_t *fb_poly_get(void) {
	return fb_poly;
}

void fb_poly_add(fb_t c, fb_t a) {
	if (c != a) {
		fb_copy(c, a);
	}

	if (poly_a != 0) {
		c[FB_DIGS - 1] ^= fb_poly[FB_DIGS - 1];
		if (pos_a != FB_DIGS - 1) {
			c[pos_a] ^= fb_poly[pos_a];
		}
		if (poly_b != 0 && poly_c != 0) {
			if (pos_b != pos_a) {
				c[pos_b] ^= fb_poly[pos_b];
			}
			if (pos_c != pos_a && pos_c != pos_b) {
				c[pos_c] ^= fb_poly[pos_c];
			}
		}
		if (pos_a != 0 && pos_b != 0 && pos_c != 0) {
			c[0] ^= 1;
		}
	} else {
		fb_add(c, a, fb_poly);
	}
}

void fb_poly_sub(fb_t c, fb_t a) {
	fb_poly_add(c, a);
}

void fb_poly_set_dense(fb_t f) {
	fb_poly_set(f);
	poly_a = poly_b = poly_c = 0;
	pos_a = pos_b = pos_c = -1;
}

void fb_poly_set_trino(int a) {
	fb_t f;

	fb_null(f);

	TRY {
		poly_a = a;
		poly_b = poly_c = 0;

		pos_a = poly_a >> FB_DIG_LOG;
		pos_b = pos_c = -1;

		fb_new(f);
		fb_zero(f);
		fb_set_bit(f, FB_BITS, 1);
		fb_set_bit(f, a, 1);
		fb_set_bit(f, 0, 1);
		fb_poly_set(f);
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
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(f);
	}
}

dig_t *fb_poly_get_srz(void) {
#if FB_SRT == QUICK || !defined(STRIP)
	return fb_srz;
#else
	return NULL;
#endif
}

dig_t *fb_poly_tab_sqr(int i) {
#if FB_INV == ITOHT || !defined(STRIP)

#ifdef FB_PRECO
	return (dig_t *)&(fb_tab_sqr[i]);
#else
	return NULL;
#endif

#else
	return NULL;
#endif
}

dig_t *fb_poly_tab_srz(int i) {
#if FB_SRT == QUICK || !defined(STRIP)

#ifdef FB_PRECO
	return fb_tab_srz[i];
#else
	return NULL;
#endif

#else
	return NULL;
#endif
}

void fb_poly_get_trc(int *a, int *b, int *c) {
#if FB_TRC == QUICK || !defined(STRIP)
	*a = trc_a;
	*b = trc_b;
	*c = trc_c;
#else
	*a = *b = *c = -1;
#endif
}

void fb_poly_get_rdc(int *a, int *b, int *c) {
	*a = poly_a;
	*b = poly_b;
	*c = poly_c;
}

dig_t *fb_poly_get_slv() {
#if FB_SLV == QUICK || !defined(STRIP)
	return (dig_t *)&(fb_half);
#else
	return NULL;
#endif
}

int *fb_poly_get_chain(int *len) {
#if FB_INV == ITOHT || !defined(STRIP)
	if (chain_len > 0 && chain_len < MAX_CHAIN) {
		if (len != NULL) {
			*len = chain_len;
		}
		return chain;
	} else {
		if (len != NULL) {
			*len = 0;
		}
		return NULL;
	}
#else
	return NULL;
#endif
}
