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
 * Implementation of the low-level multiple precision addition and subtraction
 * functions.
 *
 * @version $Id: relic_bn_add_low.c 3 2009-04-08 01:05:51Z dfaranha $
 * @ingroup bn
 */

#include <gmp.h>

#include "relic_bn.h"
#include "relic_bn_low.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

dig_t bn_add1_low(dig_t *c, dig_t *a, dig_t digit, int size) {
	return mpn_add_1(c, a, size, digit);
}

dig_t bn_addn_low(dig_t *c, dig_t *a, dig_t *b, int size) {
	return mpn_add_n(c, a, b, size);
}

dig_t bn_sub1_low(dig_t *c, dig_t *a, dig_t digit, int size) {
	return mpn_sub_1(c, a, size, digit);
}

dig_t bn_subn_low(dig_t *c, dig_t *a, dig_t *b, int size) {
	return mpn_sub_n(c, a, b, size);
}
