/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2011 RELIC Authors
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
 * Implementation of the ternary field comparison functions.
 *
 * @version $Id$
 * @ingroup ft
 */

#include "relic_core.h"
#include "relic_ft.h"
#include "relic_ft_low.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

int ft_cmp_dig(ft_t a, dig_t b) {
	for (int i = 1; i < FT_DIGS; i++) {
		if (a[i] > 0) {
			return CMP_GT;
		}
	}

	return ft_cmp1_low(a[0], b);
}

int ft_cmp(ft_t a, ft_t b) {
	return ft_cmpn_low(a, b);
}
