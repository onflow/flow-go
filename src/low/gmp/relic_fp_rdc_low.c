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
 * Implementation of the low-level prime field modular reduction functions.
 *
 * @version $Id$
 * @ingroup fp
 */

#include <gmp.h>

#include "relic_fp.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

dig_t fp_rdcn_low(dig_t *c, dig_t *a, dig_t *m, dig_t u) {
	int i;
	dig_t r, carry, *tmp;

	tmp = a;

	for (i = 0; i < FP_DIGS; i++, tmp++) {
		r = (dig_t)(*tmp * u);
		carry = mpn_addmul_1(tmp, m, FP_DIGS, r);
		carry = mpn_add_1(tmp + FP_DIGS, tmp + FP_DIGS, FP_DIGS - i, carry);
	}
	for (i = 0; i < FP_DIGS; i++, c++, tmp++) {
		*c = *tmp;
	}
	return carry;
}
