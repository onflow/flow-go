/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2013 RELIC Authors
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
 * Implementation of the basic functions to manipulate binary field elements.
 *
 * @version $Id$
 * @ingroup fb
 */

#include <ctype.h>
#include <inttypes.h>

#include "relic_core.h"
#include "relic_fb.h"
#include "relic_fb_low.h"
#include "relic_rand.h"
#include "relic_error.h"
#include "relic_util.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Checks if a radix is a power of two.
 *
 * @param[in] radix				- the radix to check.
 * @return if radix is a valid radix.
 */
static int valid_radix(int radix) {
	while (radix > 0) {
		if (radix != 1 && radix % 2 == 1)
			return 0;
		radix = radix / 2;
	}
	return 1;
}

/**
 * Computes the logarithm of a valid radix in basis two.
 *
 * @param[in] radix				- the valid radix.
 * @return the logarithm of the radix in basis two.
 */
static int log_radix(int radix) {
	int l = 0;

	while (radix > 0) {
		radix = radix / 2;
		l++;
	}
	return --l;
}

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fb_copy(fb_t c, fb_t a) {
	int i;

	for (i = 0; i < FB_DIGS; i++, c++, a++) {
		*c = *a;
	}
}

void fb_neg(fb_t c, fb_t a) {
	int i;

	for (i = 0; i < FB_DIGS; i++, c++, a++) {
		*c = *a;
	}
}

void fb_zero(fb_t a) {
	int i;

	for (i = 0; i < FB_DIGS; i++, a++) {
		*a = 0;
	}
}

int fb_is_zero(fb_t a) {
	int i;

	for (i = 0; i < FB_DIGS; i++) {
		if (a[i] != 0) {
			return 0;
		}
	}
	return 1;
}

int fb_test_bit(fb_t a, int bit) {
	int d;
	dig_t mask;

	SPLIT(bit, d, bit, FB_DIG_LOG);

	mask = ((dig_t)1) << bit;
	return (a[d] & mask) != 0;
}

int fb_get_bit(fb_t a, int bit) {
	int d;
	dig_t mask;

	SPLIT(bit, d, bit, FB_DIG_LOG);

	mask = (dig_t)1 << bit;

	return ((a[d] & mask) >> bit);
}

void fb_set_bit(fb_t a, int bit, int value) {
	int d;
	dig_t mask;

	SPLIT(bit, d, bit, FB_DIG_LOG);

	mask = (dig_t)1 << bit;

	if (value == 1) {
		a[d] |= mask;
	} else {
		a[d] &= ~mask;
	}
}

int fb_bits(fb_t a) {
	int i = FB_DIGS - 1;

	while (a[i] == 0) {
		i--;
	}

	return (i << FB_DIG_LOG) + util_bits_dig(a[i]);
}

void fb_set_dig(fb_t c, dig_t a) {
	fb_zero(c);
	c[0] = a;
}

void fb_rand(fb_t a) {
	int bits, digits;

	rand_bytes((unsigned char *)a, FB_DIGS * sizeof(dig_t));

	SPLIT(bits, digits, FB_BITS, FB_DIG_LOG);
	if (bits > 0) {
		dig_t mask = MASK(bits);
		a[FB_DIGS - 1] &= mask;
	}
}

void fb_print(fb_t a) {
	int i;

	/* Suppress possible unused parameter warning. */
	(void)a;
	for (i = FB_DIGS - 1; i >= 0; i--) {
#if WORD == 64
		util_print("%.*" PRIX64 " ", (int)(2 * (FB_DIGIT / 8)), (uint64_t)a[i]);
#else
		util_print("%.*" PRIX32 " ", (int)(2 * (FB_DIGIT / 8)), (uint32_t)a[i]);
#endif
	}
	util_print("\n");
}

void fb_size(int *size, fb_t a, int radix) {
	int digits = 0, l;
	fb_t t;

	fb_null(t);

	*size = 0;

	/* Binary case. */
	if (radix == 2) {
		*size = fb_bits(a) + 1;
		return;
	}

	/* Check the radix. */
	if (!valid_radix(radix)) {
		THROW(ERR_NO_VALID);
	}
	l = log_radix(radix);

	if (fb_is_zero(a)) {
		*size = 2;
		return;
	}

	TRY {
		fb_new(t);

		fb_copy(t, a);
		digits = 0;
		while (fb_is_zero(t) == 0) {
			fb_rsh(t, t, l);
			digits++;
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(t);
	}

	*size = digits + 1;
}

void fb_read(fb_t a, const char *str, int len, int radix) {
	int i, j, l;
	char c;
	dig_t carry;

	fb_zero(a);

	if (!valid_radix(radix)) {
		THROW(ERR_NO_VALID);
	}
	l = log_radix(radix);

	j = 0;
	while (str[j] && j < len) {
		c = (char)((radix < 36) ? TOUPPER(str[j]) : str[j]);
		for (i = 0; i < 64; i++) {
			if (c == util_conv_char(i)) {
				break;
			}
		}

		if (i < radix) {
			carry = fb_lshb_low(a, a, l);
			if (carry != 0) {
				THROW(ERR_NO_BUFFER);
			}
			fb_add_dig(a, a, (dig_t)i);
		} else {
			break;
		}
		j++;
	}
}

void fb_write(char *str, int len, fb_t a, int radix) {
	fb_t t;
	int d, l, i, j;
	char c;

	fb_null(t);

	fb_size(&l, a, radix);
	if (len <= l) {
		THROW(ERR_NO_BUFFER);
	}
	len = l;

	l = log_radix(radix);
	if (!valid_radix(radix)) {
		THROW(ERR_NO_VALID)
	}

	if (fb_is_zero(a) == 1) {
		*str++ = '0';
		*str = '\0';
		return;
	}

	TRY {
		fb_new(t);
		fb_copy(t, a);

		j = 0;
		while (!fb_is_zero(t)) {
			d = t[0] % radix;
			fb_rshb_low(t, t, l);
			str[j] = util_conv_char(d);
			j++;
		}

		/* Reverse the digits of the string. */
		i = 0;
		j = len - 2;
		while (i < j) {
			c = str[i];
			str[i] = str[j];
			str[j] = c;
			++i;
			--j;
		}

		str[len - 1] = '\0';
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(t);
	}
}
