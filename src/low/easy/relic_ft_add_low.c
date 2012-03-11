/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2012 RELIC Authors
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
 * Implementation of the low-level ternary field addition and subtraction
 * functions.
 *
 * @version $Id$
 * @ingroup fb
 */

#include "relic_ft.h"
#include "relic_ft_low.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void ft_add1_low(dig_t *c, dig_t *a, dig_t digit) {
	int i;

	(*c) = (*a) ^ digit;
	c++;
	a++;
	for (i = 0; i < FT_DIGS - 1; i++, a++, c++) {
		(*c) = (*a);
	}
}

void ft_addn_low(dig_t *c, dig_t *a, dig_t *b) {
	int i;
	dig_t t;

	dig_t *a_l = a, *a_h = a + FT_DIGS / 2;
	dig_t *b_l = b, *b_h = b + FT_DIGS / 2;
	dig_t *c_l = c, *c_h = c + FT_DIGS / 2;

	for (i = 0; i < FT_DIGS / 2; i++) {
		t = (a_l[i] | a_h[i]) & (b_l[i] | b_h[i]);
		c_l[i] = t ^ (a_l[i] | b_l[i]);
		c_h[i] = t ^ (a_h[i] | b_h[i]);
	}
}

void ft_addd_low(dig_t *c, dig_t *a, dig_t *b, int digits, int size) {
	int i;
	dig_t t;

	dig_t *a_l = a, *a_h = a + size / 2;
	dig_t *b_l = b, *b_h = b + size / 2;
	dig_t *c_l = c, *c_h = c + size / 2;

	for (i = 0; i < digits / 2; i++) {
		t = (a_l[i] | a_h[i]) & (b_l[i] | b_h[i]);
		c_l[i] = t ^ (a_l[i] | b_l[i]);
		c_h[i] = t ^ (a_h[i] | b_h[i]);
	}
}

void ft_sub1_low(dig_t *c, dig_t *a, dig_t digit) {
	int i;

	(*c) = (*a) ^ digit;
	c++;
	a++;
	for (i = 0; i < FT_DIGS - 1; i++, a++, c++) {
		(*c) = (*a);
	}
}

void ft_subn_low(dig_t *c, dig_t *a, dig_t *b) {
	int i;
	dig_t *a_l = a, *a_h = a + FT_DIGS / 2;
	dig_t *b_l = b, *b_h = b + FT_DIGS / 2;
	dig_t *c_l = c, *c_h = c + FT_DIGS / 2;
	dig_t t0, t1;

	for (i = 0; i < FT_DIGS / 2; i++) {
		t0 = (a_l[i] | a_h[i]) & (b_l[i] | b_h[i]);
		t1 = (a_h[i] | b_l[i]);
		c_l[i] = t0 ^ (a_l[i] | b_h[i]);
		c_h[i] = t0 ^ t1;
	}
}

void ft_subd_low(dig_t *c, dig_t *a, dig_t *b, int digits, int size) {
	int i;
	dig_t *a_l = a, *a_h = a + size / 2;
	dig_t *b_l = b, *b_h = b + size / 2;
	dig_t *c_l = c, *c_h = c + size / 2;
	dig_t t0, t1;

	for (i = 0; i < digits / 2; i++) {
		t0 = (a_l[i] | a_h[i]) & (b_l[i] | b_h[i]);
		t1 = (a_h[i] | b_l[i]);
		c_l[i] = t0 ^ (a_l[i] | b_h[i]);
		c_h[i] = t0 ^ t1;
	}
}
