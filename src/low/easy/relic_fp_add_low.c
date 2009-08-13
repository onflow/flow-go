/*
 * Copyright 2007-2009 RELIC Project
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
 * Implementation of the low-level prime field addition and subtraction
 * functions.
 *
 * @version $Id$
 * @ingroup fp
 */

#include "relic_fp.h"
#include "relic_fp_low.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

dig_t fp_add1_low(dig_t *c, dig_t *a, dig_t digit) {
	int i;
	dig_t carry, r0;

	carry = digit;
	for (i = 0; i < FP_DIGS && carry; i++, a++, c++) {
		r0 = (*a) + carry;
		carry = (r0 < carry);
		(*c) = r0;
	}
	for (; i < FP_DIGS; i++, a++, c++) {
		(*c) = (*a);
	}
	return carry;
}

dig_t fp_addn_low(dig_t *c, dig_t *a, dig_t *b) {
	int i;
	dig_t carry, c0, c1, r0, r1;

	carry = 0;
	for (i = 0; i < FP_DIGS; i++, a++, b++, c++) {
		r0 = (*a) + (*b);
		c0 = (r0 < (*a));
		r1 = r0 + carry;
		c1 = (r1 < r0);
		carry = c0 | c1;
		(*c) = r1;
	}
	return carry;
}

dig_t fp_sub1_low(dig_t *c, dig_t *a, dig_t digit) {
	int i;
	dig_t carry, r0;

	carry = digit;
	for (i = 0; i < FP_DIGS; i++, c++, a++) {
		r0 = (*a) - carry;
		carry = (r0 > (*a));
		(*c) = r0;
	}
	return carry;
}

dig_t fp_subn_low(dig_t *c, dig_t *a, dig_t *b) {
	int i;
	dig_t carry, r0, diff;

	/* Zero the carry. */
	carry = 0;
	for (i = 0; i < FP_DIGS; i++, a++, b++, c++) {
		diff = (*a) - (*b);
		r0 = diff - carry;
		carry = ((*a) < (*b)) || (carry && !diff);
		(*c) = r0;
	}
	return carry;
}
