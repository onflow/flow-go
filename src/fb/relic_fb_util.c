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
 * Implementation of the basic functions to manipulate binary field elements.
 *
 * @version $Id$
 * @ingroup fb
 */

#include <ctype.h>

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

	for (i = 0; i < FB_DIGS; i++, c++, a++)
		*c = *a;
}

void fb_zero(fb_t a) {
	int i;

	for (i = 0; i < FB_DIGS; i++, a++)
		*a = 0;
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
	int i, j;
	dig_t b;
	dig_t t;

	for (i = FB_DIGS - 1; i >= 0; i--) {
		t = a[i];
		if (t == 0) {
			continue;
		}
		b = (dig_t)1 << (FB_DIGIT - 1);
		j = FB_DIGIT - 1;
		while (!(t & b)) {
			j--;
			b >>= 1;
		}
		return (i << FB_DIG_LOG) + j + 1;
	}
	return 0;
}

int fb_bits_dig(dig_t a) {
#if WORD == 8 || WORD == 16
	static const unsigned char table[16] = {
		0, 1, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4
	};
#endif
#if WORD == 8
	if (a >> 4 == 0) {
		return table[a & 0xF];
	} else {
		return table[a >> 4] + 4;
	}
	return 0;
#elif WORD == 16
	int offset;

	if (a >= ((dig_t)1 << 8)) {
			offset = 8;
	} else {
			offset = 0;
	}
	a = a >> offset;
	if (a >> 4 == 0) {
			return table[a & 0xF] + offset;
	} else {
			return table[a >> 4] + 4 + offset;
	}
	return 0;
#else
	if (a == 0) {
		return 0;
	} else {
		return FB_DIGIT - __builtin_clzl(a);
	}
#endif
}

void fb_rand(fb_t a) {
	int bits;

	rand_bytes((unsigned char *)a, FB_DIGS * sizeof(dig_t));
	bits = FB_BITS - (FB_DIGS - 1) * FB_DIGIT;
	if (bits > 0) {
		dig_t mask = ((dig_t)1 << (dig_t)bits) - 1;
		a[FB_DIGS - 1] &= mask;
	}
	if (fb_cmp(a, fb_poly_get()) != CMP_LT) {
		fb_addn_low(a, a, fb_poly_get());
	}
}

void fb_print(fb_t a) {
	int i;

	/* Suppress possible unused parameter warning. */
	(void)a;
	for (i = FB_DIGS - 1; i >= 0; i--) {
		util_print("%.*lX ", (int)(2 * sizeof(dig_t)), (unsigned long int)a[i]);
	}
	printf("\n");
}

void fb_size(int *size, fb_t a, int radix) {
	int digits, l;
	fb_t t;

	*size = 0;

	/* Binary case. */
	if (radix == 2) {
		*size = fb_bits(a) + 1;
		return;
	}

	/* Check the radix. */
	if (!valid_radix(radix)) {
		THROW(ERR_INVALID);
	}
	l = log_radix(radix);

	if (fb_is_zero(a)) {
		*size = 2;
		return;
	}

	fb_new(t);
	fb_copy(t, a);
	digits = 0;
	while (fb_is_zero(t) == 0) {
		fb_rsh(t, t, l);
		digits++;
	}
	fb_free(t);

	*size = digits + 1;
}

void fb_read(fb_t a, const char *str, int len, int radix) {
	int i, j, l;
	char c;

	fb_zero(a);

	if (!valid_radix(radix)) {
		THROW(ERR_INVALID);
	}
	l = log_radix(radix);

	j = 0;
	while (str[j] && j < len) {
		c = (char)((radix < 36) ? toupper(str[j]) : str[j]);
		for (i = 0; i < 64; i++) {
			if (c == util_conv_char(i)) {
				break;
			}
		}

		if (i < radix) {
			fb_lshb_low(a, a, l);
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

	fb_size(&l, a, radix);
	if (len < l) {
		THROW(ERR_NO_BUFFER);
	}
	len = l;

	l = log_radix(radix);
	if (!valid_radix(radix)) {
		THROW(ERR_INVALID)
	}

	if (fb_is_zero(a) == 1) {
		*str++ = '0';
		*str = '\0';
		return;
	}

	fb_new(t);
	fb_copy(t, a);

	j = 0;
	while (!fb_is_zero(t)) {
		d = t[0] % radix;
		fb_rsh(t, t, l);
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

	fb_free(t);
}
