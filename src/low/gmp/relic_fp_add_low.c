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
 * Implementation of the low-level prime field addition and subtraction
 * functions.
 *
 * @version $Id$
 * @ingroup fp
 */

#include <gmp.h>

#include "relic_fp.h"
#include "relic_fp_low.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

dig_t fp_add1_low(dig_t *c, dig_t *a, dig_t digit) {
	return mpn_add_1(c, a, FP_DIGS, digit);
}

dig_t fp_addn_low(dig_t *c, dig_t *a, dig_t *b) {
	return mpn_add_n(c, a, b, FP_DIGS);
}

dig_t fp_sub1_low(dig_t *c, dig_t *a, dig_t digit) {
	return mpn_sub_1(c, a, FP_DIGS, digit);
}

dig_t fp_subn_low(dig_t *c, dig_t *a, dig_t *b) {
	return mpn_sub_n(c, a, b, FP_DIGS);
}
