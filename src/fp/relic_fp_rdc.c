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
 * Implementation of the prime field modular reduction functions.
 *
 * @version $Id$
 * @ingroup fp
 */

#include <stdlib.h>
#include <stdio.h>
#include <string.h>

#include "relic_core.h"
#include "relic_bn_low.h"
#include "relic_dv.h"
#include "relic_fp.h"
#include "relic_fp_low.h"
#include "relic_error.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp_rdc_monty_basic(fp_t c, dv_t a) {
	int i;
	dig_t r, carry, *tmp, u0;

	tmp = a;

	u0 = *(fp_prime_get_rdc());

	for (i = 0; i < FP_DIGS; i++, tmp++) {
		r = (dig_t)(*tmp * u0);
		carry = fp_muladd_low(tmp, fp_prime_get(), r);
		/* We must use this because the size (FP_DIGS - i) is variable. */
		carry = bn_add1_low(tmp + FP_DIGS, tmp + FP_DIGS, carry, FP_DIGS - i);
	}
	fp_copy(c, a + FP_DIGS);

	if (carry || fp_cmp(c, fp_prime_get()) != CMP_LT) {
		fp_subn_low(c, c, fp_prime_get());
	}
}

void fp_rdc_monty_comba(fp_t c, dv_t a) {
	dig_t carry, u0;

	u0 = *(fp_prime_get_rdc());

	carry = fp_rdcn_low(c, a, fp_prime_get(), u0);

	if (carry || fp_cmp(c, fp_prime_get()) != CMP_LT) {
		fp_subn_low(c, c, fp_prime_get());
	}
}
