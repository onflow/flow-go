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
 * Implementation of the basic functions to manipulate ternary field elements.
 *
 * @version $Id$
 * @ingroup ft
 */

#include <ctype.h>

#include "relic_core.h"
#include "relic_ft.h"
#include "relic_ft_low.h"
#include "relic_rand.h"
#include "relic_error.h"
#include "relic_util.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void ft_copy(ft_t c, ft_t a) {
	int i;

	for (i = 0; i < FT_DIGS; i++, c++, a++) {
		*c = *a;
	}
}

void ft_neg(ft_t c, ft_t a) {
	int i;
	dig_t t;

	for (i = 0; i < FT_DIGS/2; i++, c++, a++) {
		t = *a;
		*c = *(a + FT_DIGS/2);
		*(c + FT_DIGS/2) = t;
	}
}

void ft_zero(ft_t a) {
	int i;

	for (i = 0; i < FT_DIGS; i++, a++)
		*a = 0;
}

int ft_is_zero(ft_t a) {
	int i;

	for (i = 0; i < FT_DIGS; i++) {
		if (a[i] != 0) {
			return 0;
		}
	}
	return 1;
}

int ft_test_bit(ft_t a, int bit) {
	int d;
	dig_t mask;

	SPLIT(bit, d, bit, FT_DIG_LOG);

	mask = ((dig_t)1) << bit;
	return (a[d] & mask) != 0;
}

int ft_get_bit(ft_t a, int bit) {
	int d;
	dig_t mask;

	SPLIT(bit, d, bit, FT_DIG_LOG);

	mask = (dig_t)1 << bit;

	return ((a[d] & mask) >> bit);
}

void ft_set_bit(ft_t a, int bit, int value) {
	int d;
	dig_t mask;

	SPLIT(bit, d, bit, FT_DIG_LOG);

	mask = (dig_t)1 << bit;

	if (value == 1) {
		a[d] |= mask;
	} else {
		a[d] &= ~mask;
	}
}

int ft_get_trit(ft_t a, int trit) {
	return ft_get_bit(a, trit) | (ft_get_bit(a + FT_DIGS/2, trit) << 1);
}

void ft_set_trit(ft_t a, int trit, int value) {
	int d;

	SPLIT(trit, d, trit, FT_DIG_LOG);
	switch (value % 3) {
		case 0:
			ft_set_bit(a + d, trit, 0);
			ft_set_bit(a + FT_DIGS / 2 + d, trit, 0);
			break;
		case 1:
			ft_set_bit(a + d, trit, 1);
			ft_set_bit(a + FT_DIGS / 2 + d, trit, 0);
			break;
		case 2:
			ft_set_bit(a + d, trit, 0);
			ft_set_bit(a + FT_DIGS / 2 + d, trit, 1);
			break;
	}
}

int ft_test_trit(ft_t a, int trit) {
	return ft_test_bit(a, trit) | ft_test_bit(a + FT_DIGS/2, trit);
}

int ft_bits(ft_t a) {
	int i, j;
	dig_t t;

	for (i = FT_DIGS - 1; i >= 0; i--) {
		t = a[i];
		if (t == 0 ) {
			continue;
		}
		j = util_bits_dig(t);
		return (i << FT_DIG_LOG) + j;
	}
	return 0;
}

int ft_trits(ft_t a) {
	int i, j;
	dig_t t;

	for (i = FT_DIGS/2 - 1; i >= 0; i--) {
		t = a[i] | a[i + FT_DIGS / 2];
		if (t == 0) {
			continue;
		}
		j = util_bits_dig(t);
		return (i << FT_DIG_LOG) + j;
	}
	return 0;
}

void ft_set_dig(ft_t c, dig_t a) {
	ft_zero(c);
	c[0] = a;
}

void ft_rand(ft_t a) {
	int i;
	unsigned char byte;

	ft_zero(a);
	for (i = 0; i < FT_TRITS; i++) {
		rand_bytes(&byte, 1);
		ft_set_trit(a, i, byte % 3);
	}
}

void ft_print(ft_t a) {
	int i;

	if (ft_is_zero(a)) {
		util_print("0");
	} else {
		for (i = ft_trits(a) - 1; i >= 0; i--) {
			util_print("%d", ft_get_trit(a, i));
		}
	}
	util_print("\n");
}

void ft_size(int *size, ft_t a) {
	*size = ft_trits(a) + 1;
}

void ft_read(ft_t a, const char *str, int len) {
	int i, j;

	ft_zero(a);

	j = 0;
	while (str[j] && j < len) {
		j++;
	}
	if (j > FT_TRITS) {
		THROW(ERR_NO_BUFFER);
	}
	for (i = 0; i < j; i++) {
		ft_set_trit(a, i, str[j - i - 1]);
	}
}

void ft_write(char *str, int len, ft_t a) {
	int i, j, l;

	ft_size(&l, a);
	if (len <= l) {
		THROW(ERR_NO_BUFFER);
	}
	len = l;

	if (ft_is_zero(a) == 1) {
		*str++ = '0';
		*str = '\0';
		return;
	}

	j = 0;
	for (i = ft_trits(a) - 1; i >= 0; i--) {
		*str++ = util_conv_char(ft_get_trit(a, i));
	}
	*str = '\0';
}
