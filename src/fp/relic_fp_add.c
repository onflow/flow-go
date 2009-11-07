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
 * Implementation of the prime field addition and subtraction functions.
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
#include "relic_error.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp_add(fp_t c, fp_t a, fp_t b) {
	dig_t carry;

	carry = fp_addn_low(c, a, b);
	if (carry || (fp_cmp(c, fp_prime_get()) != CMP_LT)) {
		carry = fp_subn_low(c, c, fp_prime_get());
	}
}

void fp_add_dig(fp_t c, fp_t a, dig_t b) {
	dig_t carry;

	carry = fp_add1_low(c, a, b);
	if (carry || fp_cmp(c, fp_prime_get()) != CMP_LT) {
		carry = fp_subn_low(c, c, fp_prime_get());
	}
}

void fp_sub(fp_t c, fp_t a, fp_t b) {
	dig_t carry;

	carry = fp_subn_low(c, a, b);
	if (carry) {
		fp_addn_low(c, c, fp_prime_get());
	}
}

void fp_sub_dig(fp_t c, fp_t a, dig_t b) {
	dig_t carry;

	carry = fp_sub1_low(c, a, b);
	if (carry) {
		fp_addn_low(c, c, fp_prime_get());
	}
}
