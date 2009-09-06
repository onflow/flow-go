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
 * Implementation of the prime field arithmetic shift functions.
 *
 * @version $Id$
 * @ingroup fp
 */

#include <stdlib.h>
#include <stdio.h>
#include <string.h>

#include "relic_core.h"
#include "relic_fp.h"
#include "relic_fp_low.h"
#include "relic_util.h"
#include "relic_error.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp_dbl(fp_t c, fp_t a) {
	dig_t carry;

	carry = fp_lsh1_low(c, a);

	/* If there is an additional carry. */
	if (carry || (fp_cmp(c, fp_prime_get()) != CMP_LT)) {
		carry = fp_subn_low(c, c, fp_prime_get());
	}
}

void fp_hlv(fp_t c, fp_t a) {
	dig_t carry;

	carry = fp_rsh1_low(c, a);
}

void fp_lsh(fp_t c, fp_t a, int bits) {
	int digits;
	dig_t carry;

	SPLIT(bits, digits, bits, FP_DIG_LOG);

	if (digits) {
		fp_lshd_low(c, a, digits);
	} else {
		if (c != a) {
			fp_copy(c, a);
		}
	}

	switch (bits) {
		case 0:
			break;
		case 1:
			carry = fp_lsh1_low(c, c);
			break;
		default:
			carry = fp_lshb_low(c, c, bits);
			break;
	}

}

void fp_rsh(fp_t c, fp_t a, int bits) {
	int digits;
	dig_t carry;

	SPLIT(bits, digits, bits, FP_DIG_LOG);

	if (digits) {
		fp_rshd_low(c, a, digits);
	} else {
		if (c != a) {
			fp_copy(c, a);
		}
	}

	switch (bits) {
		case 0:
			break;
		case 1:
			carry = fp_rsh1_low(c, c);
			break;
		default:
			carry = fp_rshb_low(c, c, bits);
			break;
	}
}
