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
	fp_add(c, a, a);
}

void fp_hlv(fp_t c, fp_t a) {
	fp_rsh1_low(c, a);
}

void fp_lsh(fp_t c, fp_t a, int bits) {
	int digits;

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
			fp_lsh1_low(c, c);
			break;
		default:
			fp_lshb_low(c, c, bits);
			break;
	}

}

void fp_rsh(fp_t c, fp_t a, int bits) {
	int digits;

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
			fp_rsh1_low(c, c);
			break;
		default:
			fp_rshb_low(c, c, bits);
			break;
	}
}
