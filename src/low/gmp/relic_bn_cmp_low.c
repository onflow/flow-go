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
 * Implementation of the low-level multiple precision comparison functions.
 *
 * @version $Id: relic_bn_cmp_low.c 13 2009-04-16 02:24:55Z dfaranha $
 * @ingroup bn
 */

#include "relic_bn.h"
#include "relic_bn_low.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

int bn_cmp1_low(dig_t a, dig_t b) {
	return mpn_cmp(&a, &b, 1);
}

int bn_cmpn_low(dig_t *a, dig_t *b, int size) {
	return mpn_cmp(a, b, size);
}
