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
 * Implementation of the ternary field multiplication by the basis.
 *
 * @version $Id$
 * @ingroup ft
 */

#include "relic_core.h"
#include "relic_ft.h"
#include "relic_ft_low.h"
#include "relic_util.h"
#include "relic_error.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void ft_lsh(ft_t c, ft_t a, int trits) {
	int digits;

	SPLIT(trits, digits, trits, FT_DIG_LOG);

	if (digits) {
		ft_lshd_low(c, a, digits);
		ft_lshd_low(c + FT_DIGS / 2, a + FT_DIGS / 2, digits);
	} else {
		if (c != a) {
			ft_copy(c, a);
		}
	}

	switch (trits) {
		case 0:
			break;
		case 1:
			ft_lsh1_low(c, c);
			ft_lsh1_low(c + FT_DIGS / 2, c + FT_DIGS / 2);
			break;
		default:
			ft_lshb_low(c, c, trits);
			ft_lshb_low(c + FT_DIGS / 2, c + FT_DIGS / 2, trits);
			break;
	}
}

void ft_rsh(ft_t c, ft_t a, int trits) {
	int digits;

	SPLIT(trits, digits, trits, FT_DIG_LOG);

	if (digits) {
		ft_rshd_low(c, a, digits);
		ft_rshd_low(c + FT_DIGS / 2, a + FT_DIGS / 2, digits);
	} else {
		if (c != a) {
			ft_copy(c, a);
		}
	}

	switch (trits) {
		case 0:
			break;
		case 1:
			ft_rsh1_low(c, c);
			ft_rsh1_low(c + FT_DIGS / 2, c + FT_DIGS / 2);
			break;
		default:
			ft_rshb_low(c, c, trits);
			ft_rshb_low(c + FT_DIGS / 2, c + FT_DIGS / 2, trits);
			break;
	}
}
