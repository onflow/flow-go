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
 * Implementation of the multiple precision arithmetic shift functions.
 *
 * @version $Id: relic_bn_shift.c 10 2009-04-16 02:20:12Z dfaranha $
 * @ingroup bn
 */

#include "relic_core.h"
#include "relic_util.h"
#include "relic_bn.h"
#include "relic_bn_low.h"
#include "relic_error.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void bn_dbl(bn_t c, bn_t a) {
	dig_t carry;

	bn_grow(c, a->used + 1);

	c->used = a->used;
	carry = bn_lsh1_low(c->dp, a->dp, c->used);

	/* If there is an additional carry. */
	if (carry != 0) {
		c->dp[c->used] = carry;
		(c->used)++;
	}

	c->sign = a->sign;
}

void bn_hlv(bn_t c, bn_t a) {
	dig_t carry;

	bn_grow(c, a->used);

	c->used = a->used;
	carry = bn_rsh1_low(c->dp, a->dp, c->used);

	c->sign = a->sign;
	bn_trim(c);
}

void bn_lsh(bn_t c, bn_t a, int bits) {
	int digits;
	dig_t carry;

	if (bits <= 0) {
		bn_copy(c, a);
		return;
	}

	SPLIT(bits, digits, bits, BN_DIG_LOG);

	if (bits > 0) {
		if (bn_bits(c) + bits > c->used * (int)BN_DIGIT) {
			bn_grow(c, c->used + digits + 1);
		}
	} else {
		bn_grow(c, c->used + digits);
	}

	if (digits > 0) {
		bn_lshd_low(c->dp, a->dp, a->used, digits);
	}
	c->used = a->used + digits;
	c->sign = a->sign;

	if (bits > 0) {
		if (c != a) {
			carry = bn_lshb_low(c->dp + digits, a->dp, a->used, bits);
		} else {
			carry = bn_lshb_low(c->dp + digits, c->dp + digits, c->used, bits);
		}
		if (carry != 0) {
			c->dp[c->used] = carry;
			(c->used)++;
		}
	}
	bn_trim(c);
}

void bn_rsh(bn_t c, bn_t a, int bits) {
	int digits = 0;
	dig_t carry;

	if (bits <= 0) {
		bn_copy(c, a);
		return;
	}

	SPLIT(bits, digits, bits, BN_DIG_LOG);

	if (digits > 0) {
		bn_rshd_low(c->dp, a->dp, a->used, digits);
	}
	c->used = a->used - digits;
	c->sign = a->sign;

	if (bits > 0) {
		if (digits == 0 && c != a) {
			carry = bn_rshb_low(c->dp, a->dp + digits, a->used - digits, bits);
		} else {
			carry = bn_rshb_low(c->dp, c->dp, c->used, bits);
		}
	}
	bn_trim(c);
}
