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
 * Implementation of the binary field multiplication functions.
 *
 * @version $Id$
 * @ingroup fb
 */

#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_fb.h"
#include "relic_fb_low.h"
#include "relic_bn_low.h"
#include "relic_util.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if FB_MUK > 0 || !defined(STRIP)

/**
 * Multiplies two binary field elements using shift-and-add multiplication.
 *
 * @param c					- the result.
 * @param a					- the first binary field element.
 * @param b					- the second binary field element.
 * @param size				- the number of digits to multiply.
 */
void fb_mul_basic_impl(dig_t *c, dig_t *a, dig_t *b, int size) {
	int i;
	dv_t s = NULL;

	TRY {
		/* We need a temporary variable so that c can be a or b. */
		dv_new(s);
		dv_zero(s, 2 * FB_DIGS);

		for (i = 0; i < size; i++) {
			s[i] = b[i];
		}

		if (a[0] & 1) {
			for (i = 0; i < size; i++) {
				c[i] = b[i];
			}
		}
		for (i = 1; i <= (FB_DIGIT * size) - 1; i++) {
			/* We are already shifting a temporary value, so this is more efficient
			 * than calling fb_lsh(). */
			bn_lsh1_low(s, s, size + 1);
			fb_rdc(s, s);
			if (fb_test_bit(a, i)) {
				fb_addd_low(c, c, s, size + 1);
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(s);
	}
}

/**
 * Multiplies two binary field elements using left-to-right comb multiplication.
 *
 * @param c					- the result.
 * @param a					- the first binary field element.
 * @param b					- the second binary field element.
 * @param size				- the number of digits to multiply.
 */
void fb_mul_lcomb_impl(dig_t *c, dig_t *a, dig_t *b, int size) {
	dig_t carry;

	for (int i = FB_DIGIT - 1; i >= 0; i--) {
		for (int j = 0; j < size; j++) {
			if (a[j] & ((dig_t)1 << i)) {
				fb_addd_low(c + j, c + j, b, size);
			}
		}
		if (i != 0) {
			carry = bn_lsh1_low(c, c, 2 * size);
		}
	}
}

/**
 * Multiplies two binary field elements using right-to-left comb multiplication.
 *
 * @param c					- the result.
 * @param a					- the first binary field element.
 * @param b					- the second binary field element.
 * @param size				- the number of digits to multiply.
 */
void fb_mul_rcomb_impl(dig_t *c, dig_t *a, dig_t *b, int size) {
	dv_t _b = NULL;
	dig_t carry;

	TRY {
		dv_new(_b);
		dv_zero(_b, size + 1);

		for (int i = 0; i < size; i++)
			_b[i] = b[i];

		for (int i = 0; i < FB_DIGIT; i++) {
			for (int j = 0; j < size; j++) {
				if (a[j] & ((dig_t)1 << i)) {
					fb_addd_low(c + j, c + j, _b, size + 1);
				}
			}
			if (i != FB_DIGIT - 1) {
				carry = bn_lsh1_low(_b, _b, size);
				_b[size] = (_b[size] << 1) | carry;
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(_b);
	}
}

/**
 * Multiplies two binary field elements using lopez-dahab multiplication.
 *
 * @param c					- the result.
 * @param a					- the first binary field element.
 * @param b					- the second binary field element.
 * @param size				- the number of digits to multiply.
 */
void fb_mul_lodah_impl(dig_t *c, dig_t *a, dig_t *b, int size) {
	dv_t table[16] = { NULL };
	dig_t u, *tmpa, *tmpc, r0, r1, r2, r4, r8;
	int i, j;

	TRY {
		for (i = 0; i < 16; i++) {
			dv_new(table[i]);
			dv_zero(table[i], size + 1);
		}

		u = 0;
		for (i = 0; i < size; i++) {
			r1 = r0 = b[i];
			r2 = (r0 << 1) | (u >> (FB_DIGIT - 1));
			r4 = (r0 << 2) | (u >> (FB_DIGIT - 2));
			r8 = (r0 << 3) | (u >> (FB_DIGIT - 3));
			table[0][i] = 0;
			table[1][i] = r1;
			table[2][i] = r2;
			table[3][i] = r1 ^ r2;
			table[4][i] = r4;
			table[5][i] = r1 ^ r4;
			table[6][i] = r2 ^ r4;
			table[7][i] = r1 ^ r2 ^ r4;
			table[8][i] = r8;
			table[9][i] = r1 ^ r8;
			table[10][i] = r2 ^ r8;
			table[11][i] = r1 ^ r2 ^ r8;
			table[12][i] = r4 ^ r8;
			table[13][i] = r1 ^ r4 ^ r8;
			table[14][i] = r2 ^ r4 ^ r8;
			table[15][i] = r1 ^ r2 ^ r4 ^ r8;
			u = r1;
		}

		if (u > 0) {
			r1 = 0;
			r2 = u >> (FB_DIGIT - 1);
			r4 = u >> (FB_DIGIT - 2);
			r8 = u >> (FB_DIGIT - 3);
			table[0][size] = table[1][size] = 0;
			table[2][size] = table[3][size] = r2;
			table[4][size] = table[5][size] = r4;
			table[6][size] = table[7][size] = r2 ^ r4;
			table[8][size] = table[9][size] = r8;
			table[10][size] = table[11][size] = r2 ^ r8;
			table[12][size] = table[13][size] = r4 ^ r8;
			table[14][size] = table[15][size] = r2 ^ r4 ^ r8;
		}

		for (i = FB_DIGIT - 4; i >= 4; i -= 4) {
			tmpa = a;
			tmpc = c;
			for (j = 0; j < size; j++, tmpa++, tmpc++) {
				u = (*tmpa >> i) & 0x0F;
				fb_addd_low(tmpc, tmpc, table[u], size);
				*(tmpc + size) ^= table[u][size];
			}
			bn_lshb_low(c, c, 2 * size, 4);
		}
		for (j = 0; j < size; j++, a++, c++) {
			u = *a & 0x0F;
			fb_addd_low(c, c, table[u], size);
			*(c + size) ^= table[u][size];
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		for (i = 0; i < 16; i++) {
			dv_free(table[i]);
		}
	}
}

/**
 * Multiplies two binary field elements using recursive Karatsuba
 * multiplication.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the first binary field element.
 * @param[in] b				- the second binary field element.
 * @param[in] size			- the number of digits to multiply.
 * @param[in] level			- the number of Karatsuba steps to apply.
 */
void fb_mul_karat_impl(dv_t c, fb_t a, fb_t b, int size, int level) {
	int i, h, h1;
	dv_t a1 = NULL, b1 = NULL, a0b0 = NULL, a1b1 = NULL;

	/* Compute half the digits of a or b. */
	h = size >> 1;
	h1 = size - h;

	TRY {
		/* Allocate the temp variables. */
		dv_new(a1);
		dv_new(b1);
		dv_new(a0b0);
		dv_new(a1b1);
		dv_zero(a1, h1);
		dv_zero(b1, h1);
		dv_zero(a0b0, 2 * h);
		dv_zero(a1b1, 2 * h1);

		/* a0b0 = a0 * b0 and a1b1 = a1 * b1 */
		if (level <= 1) {
#if FB_MUL == BASIC
			fb_mul_basic_impl(a0b0, a, b, h);
			fb_mul_basic_impl(a1b1, a + h, b + h, h1);
#elif FB_MUL == LCOMB
			fb_mul_lcomb_impl(a0b0, a, b, h);
			fb_mul_lcomb_impl(a1b1, a + h, b + h, h1);
#elif FB_MUL == RCOMB
			fb_mul_rcomb_impl(a0b0, a, b, h);
			fb_mul_rcomb_impl(a1b1, a + h, b + h, h1);
#elif FB_MUL == INTEG || FB_MUL == LODAH
			fb_mul_lodah_impl(a0b0, a, b, h);
			fb_mul_lodah_impl(a1b1, a + h, b + h, h1);
#endif
		} else {
			fb_mul_karat_impl(a0b0, a, b, h, level - 1);
			fb_mul_karat_impl(a1b1, a + h, b + h, h1, level - 1);
		}

		for (i = 0; i < 2 * h; i++) {
			c[i] = a0b0[i];
		}

		for (i = 0; i < 2 * h1; i++) {
			c[2 * h + i] = a1b1[i];
		}

		/* c = c - (a0*b0 << h digits) */
		fb_addd_low(c + h, c + h, a0b0, 2 * h);

		/* c = c - (a1*b1 << h digits) */
		fb_addd_low(c + h, c + h, a1b1, 2 * h1);

		/* a1 = (a1 + a0) */
		fb_addd_low(a1, a, a + h, h);

		/* b1 = (b1 + b0) */
		fb_addd_low(b1, b, b + h, h);
		if (h1 > h) {
			a1[h1 - 1] = a[h + h1 - 1];
			b1[h1 - 1] = b[h + h1 - 1];
		}

		for (i = 0; i < 2 * h1; i++) {
			a1b1[i] = 0;
		}

		if (level <= 1) {
			/* a1b1 = (a1 + a0)*(b1 + b0) */
#if FB_MUL == BASIC
			fb_mul_basic_impl(a1b1, a1, b1, h1);
#elif FB_MUL == LCOMB
			fb_mul_lcomb_impl(a1b1, a1, b1, h1);
#elif FB_MUL == RCOMB
			fb_mul_rcomb_impl(a1b1, a1, b1, h1);
#elif FB_MUL == INTEG || FB_MUL == LODAH
			fb_mul_lodah_impl(a1b1, a1, b1, h1);
#endif
		} else {
			fb_mul_karat_impl(a1b1, a1, b1, h1, level - 1);
		}

		/* c = c + [(a1 + a0)*(b1 + b0) << digits] */
		fb_addd_low(c + h, c + h, a1b1, 2 * h1);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(a1);
		dv_free(b1);
		dv_free(a0b0);
		dv_free(a1b1);
	}
}

#endif

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if FB_MUL == BASIC || !defined(STRIP)

void fb_mul_basic(fb_t c, fb_t a, fb_t b) {
	int i;
	dv_t s = NULL, t = NULL;

	TRY {
		/* We need a temporary variable so that c can be a or b. */
		dv_new(t);
		dv_new(s);
		dv_zero(t, 2 * FB_DIGS);
		dv_zero(s, 2 * FB_DIGS);
		fb_copy(s, b);

		if (a[0] & 1) {
			fb_copy(t, b);
		}
		for (i = 1; i <= FB_BITS - 1; i++) {
			/* We are already shifting a temporary value, so this is more efficient
			 * than calling fb_lsh(). */
			fb_lsh1_low(s, s);
			fb_rdc(s, s);
			if (fb_test_bit(a, i)) {
				fb_add(t, t, s);
			}
		}

		fb_copy(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(t);
		dv_free(s);
	}
}

#endif

#if FB_MUL == INTEG || !defined(STRIP)

void fb_mul_integ(fb_t c, fb_t a, fb_t b) {
	dv_t t = NULL;

	TRY {
		dv_new(t);

		fb_mulm_low(c, t, a, b);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(t);
	}
}

#endif

#if FB_MUL == LCOMB || !defined(STRIP)

void fb_mul_lcomb(fb_t c, fb_t a, fb_t b) {
	dv_t t = NULL;
	dig_t carry;

	TRY {
		dv_new(t);
		dv_zero(t, 2 * FB_DIGS);

		for (int i = FB_DIGIT - 1; i >= 0; i--) {
			for (int j = 0; j < FB_DIGS; j++) {
				if (a[j] & ((dig_t)1 << i)) {
					fb_addn_low(t + j, t + j, b);
				}
			}
			if (i != 0) {
				carry = fb_lsh1_low(t, t);
				fb_lsh1_low(t + FB_DIGS, t + FB_DIGS);
				t[FB_DIGS] |= carry;
			}
		}

		fb_rdc(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(t);
	}
}

#endif

#if FB_MUL == RCOMB || !defined(STRIP)

void fb_mul_rcomb(fb_t c, fb_t a, fb_t b) {
	dv_t t = NULL, _b = NULL;
	dig_t carry;

	TRY {
		dv_new(t);
		dv_new(_b);
		dv_zero(t, 2 * FB_DIGS);
		dv_zero(_b, FB_DIGS + 1);

		fb_copy(_b, b);

		for (int i = 0; i < FB_DIGIT; i++) {
			for (int j = 0; j < FB_DIGS; j++) {
				if (a[j] & ((dig_t)1 << i)) {
					fb_addd_low(t + j, t + j, _b, FB_DIGS + 1);
				}
			}
			if (i != FB_DIGIT - 1) {
				carry = fb_lsh1_low(_b, _b);
				_b[FB_DIGS] = (_b[FB_DIGS] << 1) | carry;
			}
		}

		fb_rdc(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(t);
		dv_free(_b);
	}
}

#endif

#if FB_MUL == LODAH || !defined(STRIP)

void fb_mul_lodah(fb_t c, fb_t a, fb_t b) {
	dv_t t = NULL;

	TRY {
		dv_new(t);

		fb_muln_low(t, a, b);

		fb_rdc(c, t);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(t);
	}
}

#endif

#if FB_MUK > 0 || !defined(STRIP)

void fb_mul_karat(fb_t c, fb_t a, fb_t b) {
	dv_t t = NULL;

	TRY {
		/* We need a temporary variable so that c can be a or b. */
		dv_new(t);
		dv_zero(t, 2 * FB_DIGS);

		fb_mul_karat_impl(t, a, b, FB_DIGS, FB_MUK);

		fb_rdc(c, t);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(t);
	}
}

#endif

void fb_mul_dig(fb_t c, fb_t a, dig_t b) {
	dv_t t = NULL;

	TRY {
		dv_new(t);

		fb_mul1_low(t, a, b);

		fb_rdc(c, t);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(t);
	}
}
