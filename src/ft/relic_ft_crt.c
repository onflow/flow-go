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
 * Implementation of the ternary field cube root.
 *
 * @version $Id$
 * @ingroup ft
 */

#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_ft.h"
#include "relic_ft_low.h"
#include "relic_util.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if FT_CRT == BASIC || !defined(STRIP)

void ft_crt_basic(ft_t c, ft_t a) {
	if (c != a) {
		ft_copy(c, a);
	}

	for (int i = 1; i < FT_TRITS; i++) {
		ft_cub(c, c);
	}
}

#endif

#if FT_CRT == QUICK || !defined(STRIP)

void ft_crt_quick(ft_t c, ft_t a) {
	ft_crtn_low(c, a);
}

#endif
