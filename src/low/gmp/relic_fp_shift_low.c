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
 * Implementation of the low-level prime field shifting functions.
 *
 * @version $Id$
 * @ingroup bn
 */

#include "relic_fp.h"
#include "relic_fp_low.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

dig_t fp_lsh1_low(dig_t *c, dig_t *a) {
	return mpn_lshift(c, a, FP_DIGS, 1);
}

dig_t fp_lshb_low(dig_t *c, dig_t *a, int bits) {
	return mpn_lshift(c, a, FP_DIGS, bits);
}

void fp_lshd_low(dig_t *c, dig_t *a, int digits) {
	dig_t *top, *bot;
	int i;

	top = c + FP_DIGS + digits - 1;
	bot = a + FP_DIGS - 1;

	for (i = 0; i < FP_DIGS; i++, top--, bot--) {
		*top = *bot;
	}
	for (i = 0; i < digits; i++, c++) {
		*c = 0;
	}
}

dig_t fp_rsh1_low(dig_t *c, dig_t *a) {
	return mpn_rshift(c, a, FP_DIGS, 1);
}

dig_t fp_rshb_low(dig_t *c, dig_t *a, int bits) {
	return mpn_rshift(c, a, FP_DIGS, bits);
}

void fp_rshd_low(dig_t *c, dig_t *a, int digits) {
	dig_t *top, *bot;
	int i;

	top = a + digits;
	bot = c;

	for (i = 0; i < FP_DIGS - digits; i++, top++, bot++) {
		*bot = *top;
	}
	for (; i < FP_DIGS; i++, bot++) {
		*bot = 0;
	}
}
